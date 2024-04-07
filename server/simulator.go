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

type Simulator struct{}

var adapter = bluetooth.DefaultAdapter
var state = SimulatorState{
	status:   Idle,
	pumpType: DanaI,
	name:     randomName(),
}

func (s Simulator) StartBluetooth() {
	setDeviceName(state.name)

	must("enable BLE stack", adapter.Enable())

	// Define the peripheral device info.
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    state.name,
		ServiceUUIDs: []bluetooth.UUID{},
	}))

	// Start advertising
	must("start adv", adv.Start())
	fmt.Println("Adversing with name: " + state.name)

	var heartRateMeasurement bluetooth.Characteristic
	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.ServiceUUIDHeartRate,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &heartRateMeasurement,
				UUID:   bluetooth.CharacteristicUUIDHeartRateMeasurement,
				Value:  []byte{0, 75},
				Flags:  bluetooth.CharacteristicNotifyPermission,
			},
		},
	}))
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
