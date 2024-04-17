package server

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"time"

	codes "dana/simulator/server/packets"

	"tinygo.org/x/bluetooth"
)

const (
	PACKET_START_BYTE    byte = 0xa5
	PACKET_END_BYTE      byte = 0x5a
	ENCRYPTED_START_BYTE byte = 0xaa
	ENCRYPTED_END_BYTE   byte = 0xee
)

type Simulator struct {
	hasOpenConnection  bool
	isConnectionSecure bool
	readBuffer         []byte

	writeCharacteristic bluetooth.Characteristic
	readCharacteristic  bluetooth.Characteristic
}

var adapter = bluetooth.DefaultAdapter
var state = SimulatorState{
	status:   Idle,
	pumpType: DanaI,
	name:     randomName(),
}

var encryption = DanaEncryption{
	state: &state,
}

var commandCenter = CommandCenter{
	state:      &state,
	encryption: encryption,
}

func (s *Simulator) StartBluetooth() {
	setDeviceName(state.name)

	must("enable BLE stack", adapter.Enable())

	// Define the peripheral device info.
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    state.name,
		ServiceUUIDs: []bluetooth.UUID{},
	}))

	adapter.SetConnectHandler(s.handleConnectionChange)

	// Start advertising
	must("start adv", adv.Start())
	fmt.Println("Adversing with name: " + state.name)

	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.New16BitUUID(0xFFF0),
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &s.writeCharacteristic,
				UUID:   bluetooth.New16BitUUID(0xFFF1),
				Value:  []byte{},
				Flags:  bluetooth.CharacteristicNotifyPermission,
			},
			{
				Handle:     &s.readCharacteristic,
				UUID:       bluetooth.New16BitUUID(0xFFF2),
				Value:      []byte{},
				Flags:      bluetooth.CharacteristicWritePermission | bluetooth.CharacteristicWriteWithoutResponsePermission,
				WriteEvent: s.handleMessage,
			},
		},
	}))

	commandCenter.writeCharacteristic = &s.writeCharacteristic
}

func (s *Simulator) handleConnectionChange(device bluetooth.Device, connected bool) {
	if connected && s.hasOpenConnection {
		device.Disconnect()
		fmt.Println("ERROR: Rejecting connection from " + device.Address.String() + ", Already has an open connection")
	} else if connected {
		s.hasOpenConnection = true
		s.isConnectionSecure = false
		encryption.ResetRandomSyncKey()
		s.readBuffer = []byte{}
		fmt.Println("INFO: Device connected: " + device.Address.String())
	} else {
		s.hasOpenConnection = false
		fmt.Println("INFO: Device disconnected: " + device.Address.String())
	}
}

func (s *Simulator) handleMessage(client bluetooth.Connection, offset int, value []byte) {
	if s.isConnectionSecure {
		// Do second level decryption
		value = encryption.DecryptionSecondLvl(value)
	}

	s.readBuffer = append(s.readBuffer, value...)
	if len(s.readBuffer) < 6 {
		// Buffer is not ready to be processed
		return
	}

	if !(s.readBuffer[0] == PACKET_START_BYTE || s.readBuffer[0] == ENCRYPTED_START_BYTE) ||
		!(s.readBuffer[1] == PACKET_START_BYTE || s.readBuffer[1] == ENCRYPTED_START_BYTE) {
		// The buffer does not start with the opening bytes. Check if the buffer is filled with old data

		var indexStartByte = slices.Index(s.readBuffer, PACKET_START_BYTE)
		var indexStartEncryptedByte = slices.Index(s.readBuffer, ENCRYPTED_START_BYTE)
		if indexStartByte != -1 {
			s.readBuffer = s.readBuffer[indexStartByte:len(s.readBuffer)]

		} else if indexStartEncryptedByte != -1 {
			s.readBuffer = s.readBuffer[indexStartEncryptedByte:len(s.readBuffer)]

		} else {
			fmt.Println("ERROR: Received invalid packets. Starting bytes do not exists in message. Data: " + base64.StdEncoding.EncodeToString(s.readBuffer))
			s.readBuffer = []byte{}
			return
		}
	}

	var length = int(s.readBuffer[2])
	if length+7 != len(s.readBuffer) {
		// Not all packets have been received yet...
		return
	}

	if !(s.readBuffer[length+5] == PACKET_END_BYTE || s.readBuffer[length+5] == ENCRYPTED_END_BYTE) ||
		!(s.readBuffer[length+6] == PACKET_END_BYTE || s.readBuffer[length+6] == ENCRYPTED_END_BYTE) {
		fmt.Println("ERROR: Received invalid packets. Ending bytes do not match. Data: " + base64.StdEncoding.EncodeToString(s.readBuffer))
		s.readBuffer = []byte{}
		return
	}

	var decryptedData = encryption.Decryption(s.readBuffer, !s.isConnectionSecure)
	s.readBuffer = []byte{}

	if len(decryptedData) == 0 {
		fmt.Println("ERROR: Failed to decrypt")
		return
	}

	if decryptedData[0] == codes.TYPE_ENCRYPTION_REQUEST {
		s.isConnectionSecure = commandCenter.ProcessEncryptionCommand(decryptedData)

	} else if decryptedData[0] == codes.TYPE_COMMAND {
		commandCenter.ProcessCommand(decryptedData)

	} else {
		fmt.Println("ERROR: Received invalid command type. Got: " + fmt.Sprint(decryptedData[0]))
	}
}

func setDeviceName(name string) {
	// Force bluetooth name. This only works on linux
	must("Write new machine-info file", os.WriteFile("machine-info", []byte("PRETTY_HOSTNAME="+name), 0666))

	var cmd = exec.Command("/bin/sh", "-c", "sudo mv machine-info /etc/machine-info")
	must("Write bluetooth name", cmd.Run())

	// Randomize MAC-address to prevent device name caching issues
	// https://raspberrypi.stackexchange.com/a/124117
	var newMac = "0x" + randomHex() + " 0x" + randomHex() + " 0x" + randomHex() + " 0x" + randomHex() + " 0x" + randomHex() + " 0x" + randomHex()
	fmt.Println("New BLE mac address (reversed): " + newMac)
	cmd = exec.Command("/bin/sh", "-c", "sudo hcitool cmd 0x3f 0x001 "+newMac)
	must("Randomize MAC-address", cmd.Run())

	cmd = exec.Command("/bin/sh", "-c", "sudo hciconfig hci0 reset")
	must("Reset bluetooth driver", cmd.Run())

	cmd = exec.Command("/bin/sh", "-c", "sudo service bluetooth restart")
	must("Restart bluetooth chip", cmd.Run())

	time.Sleep(5 * time.Second)
}

func randomName() string {
	var characters = "ABCDEFGHIJKLMNOPQRSTUVXYZ"
	var length = len(characters)
	var name = string(characters[randomInt(0, length)]) +
		string(characters[randomInt(0, length)]) +
		string(characters[randomInt(0, length)]) +
		strconv.Itoa(int(randomInt(0, 9))) +
		strconv.Itoa(int(randomInt(0, 9))) +
		strconv.Itoa(int(randomInt(0, 9))) +
		strconv.Itoa(int(randomInt(0, 9))) +
		strconv.Itoa(int(randomInt(0, 9))) +
		string(characters[randomInt(0, length)]) +
		string(characters[randomInt(0, length)])

	return name
}

func randomHex() string {
	return fmt.Sprintf("%x", randomInt(0, 255))
}

func randomInt(min, max int) uint8 {
	return uint8(min + rand.Intn(max-min))
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
