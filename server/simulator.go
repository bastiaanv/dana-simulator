package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	PACKET_START_BYTE    byte = 0xa5
	PACKET_END_BYTE      byte = 0x5a
	ENCRYPTED_START_BYTE byte = 0xaa
	ENCRYPTED_END_BYTE   byte = 0xee
)

var _, timeZoneOffset = time.Now().Zone()

type Simulator struct {
	State         *SimulatorState
	encryption    *DanaEncryption
	commandCenter *CommandCenter
	readBuffer    []byte

	shouldDoSecondDecryption bool

	writeCharacteristic bluetooth.Characteristic
	readCharacteristic  bluetooth.Characteristic
}

func NewSimulator() Simulator {
	var state = GetDefaultState()
	var encryption = DanaEncryption{
		state: &state,
	}

	var commandCenter = CommandCenter{
		state:      &state,
		encryption: &encryption,
	}

	return Simulator{
		State:         &state,
		encryption:    &encryption,
		commandCenter: &commandCenter,
	}
}

func (s *Simulator) StartBluetooth() {
	setDeviceName(s.State.Name)

	var adapter = bluetooth.DefaultAdapter
	// TinyGo bluetooth (linux) doesnt support connection handler
	// adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
	// 	if connected && s.hasOpenConnection {
	// 		fmt.Println("ERROR: Rejecting connection from " + device.Address.String() + ", Already has an open connection")
	// 	} else if connected {
	// 		s.hasOpenConnection = true
	// 		s.isConnectionSecure = false
	// 		encryption.ResetRandomSyncKey()
	// 		s.readBuffer = []byte{}
	// 		fmt.Println("INFO: Device connected: " + device.Address.String())
	// 	} else {
	// 		s.hasOpenConnection = false
	// 		fmt.Println("INFO: Device disconnected: " + device.Address.String())
	// 	}
	// })

	must("enable BLE stack", adapter.Enable())

	// Define the peripheral device info.
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    s.State.Name,
		ServiceUUIDs: []bluetooth.UUID{},
	}))

	// Start advertising
	must("start adv", adv.Start())
	fmt.Println("Adversing with name: " + s.State.Name)

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

	s.commandCenter.writeCharacteristic = &s.writeCharacteristic
	s.State.Status = STATUS_RUNNING

	json, err := json.Marshal(s.State)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(time.Now().Format(time.RFC3339) + "INFO: Running pump with state: " + string(json))
}

func (s *Simulator) handleMessage(client bluetooth.Connection, offset int, value []byte) {
	// If we receive a new message (for a non-danaRS-v1 pump) and the start byte isnt the normal start byte,
	// we assume we need to do a second lvl decryption first.

	// Since TinyGo Bluetooth doesnt notify us when a device is connected or disconnected,
	// is this the only we currently

	if s.State.PumpType == PUMP_TYPE_DANA_RS_V1 {
		// Isnt supported with the DanaRS_v1
		s.shouldDoSecondDecryption = false
	} else if len(s.readBuffer) == 0 {
		// Only check if when the buffer is empty == new message
		s.shouldDoSecondDecryption = value[0] != PACKET_START_BYTE
	}

	if s.shouldDoSecondDecryption {
		fmt.Println("Doing second lvl decryption")
		value = s.encryption.DecryptionSecondLvl(value)
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

	var decryptedData = s.encryption.Decryption(s.readBuffer)
	s.readBuffer = []byte{}

	if len(decryptedData) == 0 {
		fmt.Println("ERROR: Failed to decrypt")
		return
	}

	if decryptedData[0] == TYPE_ENCRYPTION_REQUEST {
		s.commandCenter.ProcessEncryptionCommand(decryptedData)

	} else if decryptedData[0] == TYPE_COMMAND {
		s.commandCenter.ProcessCommand(decryptedData)

	} else {
		fmt.Println("ERROR: Received invalid command type. Got: " + fmt.Sprint(decryptedData[0]))
	}
}

func setDeviceName(name string) {
	// Force bluetooth name. This only works on linux
	must("Write new machine-info file", os.WriteFile("machine-info", []byte("PRETTY_HOSTNAME="+name), 0666))

	var cmd = exec.Command("/bin/sh", "-c", "sudo mv machine-info /etc/machine-info")
	must("Write bluetooth name", cmd.Run())

	// Randomize MAC-address to prevent device name caching issues, based on device name
	// https://raspberrypi.stackexchange.com/a/124117
	var newMac = ""
	for i := 0; i < 6; i++ {
		newMac += fmt.Sprintf("0x%x ", name[i])
	}

	fmt.Println("New BLE mac address (reversed): " + newMac)
	cmd = exec.Command("/bin/sh", "-c", "sudo hcitool cmd 0x3f 0x001 "+newMac)
	must("Randomize MAC-address", cmd.Run())

	cmd = exec.Command("/bin/sh", "-c", "sudo hciconfig hci0 reset")
	must("Reset bluetooth driver", cmd.Run())

	cmd = exec.Command("/bin/sh", "-c", "sudo service bluetooth restart")
	must("Restart bluetooth chip", cmd.Run())

	time.Sleep(5 * time.Second)
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
