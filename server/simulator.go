package server

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"time"

	"tinygo.org/x/bluetooth"
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
		UUID: bluetooth.ServiceUUIDHeartRate,
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
}

func (s *Simulator) handleConnectionChange(device bluetooth.Device, connected bool) {
	if connected && s.hasOpenConnection {
		device.Disconnect()
		fmt.Println("Rejecting connection from " + device.Address.String() + ", Already has an open connection")
	} else if connected {
		s.hasOpenConnection = true
		s.isConnectionSecure = false
		encryption.ResetRandomSyncKey()
		s.readBuffer = []byte{}
		fmt.Println("Device connected: " + device.Address.String())
	} else {
		s.hasOpenConnection = false
		fmt.Println("Device disconnected: " + device.Address.String())
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
