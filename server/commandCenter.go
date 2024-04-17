package server

import (
	codes "dana/simulator/server/packets"
	"encoding/base64"
	"fmt"
	"math"

	"tinygo.org/x/bluetooth"
)

type CommandCenter struct {
	encryption          DanaEncryption
	state               *SimulatorState
	writeCharacteristic *bluetooth.Characteristic
}

func (c *CommandCenter) ProcessEncryptionCommand(data []byte) bool {
	switch data[1] {
	case codes.OPCODE_ENCRYPTION__PUMP_CHECK:
		return c.respondToCommandRequest()
	case codes.OPCODE_ENCRYPTION__TIME_INFORMATION:
		return c.respondToTimeRequest()
	}

	return false
}

func (c *CommandCenter) ProcessCommand(data []byte) {
	fmt.Println("ERROR: Received unsupported request - command: " + fmt.Sprint(data[1]))
}

func (c *CommandCenter) respondToCommandRequest() bool {
	var data = c.encryption.Encryption(EncryptionParams{operationCode: codes.OPCODE_ENCRYPTION__PUMP_CHECK, data: []byte{}})

	fmt.Println("INFO: Sending OPCODE_ENCRYPTION__PUMP_CHECK - Data: " + base64.StdEncoding.EncodeToString(data))
	c.write(data)

	return false
}

func (c *CommandCenter) respondToTimeRequest() bool {
	var data = c.encryption.Encryption(EncryptionParams{operationCode: codes.OPCODE_ENCRYPTION__TIME_INFORMATION, data: []byte{}})

	fmt.Println("INFO: Sending OPCODE_ENCRYPTION__TIME_INFORMATION - Data: " + base64.StdEncoding.EncodeToString(data))
	c.write(data)

	// When pump type is Dana-I, handshake is completed
	return c.state.pumpType == DanaI
}

func (c *CommandCenter) write(data []byte) {
	var index = 0
	for index < len(data) {
		var length = int(math.Min(20, float64(len(data)-index)))
		var subData = data[index : index+length]

		var _, err = c.writeCharacteristic.Write(subData)
		if err != nil {
			fmt.Println("ERROR: failed to write data: " + err.Error())
			return
		}

		index += length
	}
}
