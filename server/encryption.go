package server

import (
	codes "dana/simulator/server/packets"
	"encoding/base64"
	"fmt"
)

// Dana-I
var ble5Keys = []byte{0x36, 0x36, 0x36, 0x38, 0x36, 0x36}
var ble5RandomKeys = []byte{
	secondLvlEncryptionLookup[((ble5Keys[0]-0x30)*10)+ble5Keys[1]-0x30],
	secondLvlEncryptionLookup[((ble5Keys[2]-0x30)*10)+ble5Keys[3]-0x30],
	secondLvlEncryptionLookup[((ble5Keys[4]-0x30)*10)+ble5Keys[5]-0x30],
}

// DanaRS-v3
var pairingKeys = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
var randomPairingKeys = []byte{0x00, 0x00, 0x00}

// DanaRS-v1
var timeSecret = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
var passKeySecret = []byte{0x00, 0x00}
var passwordSecret = []byte{0x00, 0x00}

type DanaEncryption struct {
	state *SimulatorState

	randomSyncKey byte
}

type EncryptionParams struct {
	operationCode byte
	data          []byte
}

func (e *DanaEncryption) ResetRandomSyncKey() {
	e.randomSyncKey = 0
}

func (e DanaEncryption) Encryption(params EncryptionParams) []byte {
	switch params.operationCode {
	case codes.OPCODE_ENCRYPTION__PUMP_CHECK:
		return e.encodePumpCheck()
	case codes.OPCODE_ENCRYPTION__TIME_INFORMATION:
		return e.encodeTimeInformation()
	}

	return e.encodeMessage(params.data, params.operationCode, false)
}

func (e *DanaEncryption) EncryptionSecondLvl(data []byte) []byte {
	if e.state.pumpType == PUMP_TYPE_DANA_RS_V3 {
		var updatedRandomSyncKey = e.randomSyncKey
		if data[0] == 0xa5 && data[1] == 0xa5 {
			data[0] = 0x7a
			data[1] = 0x7a
		}

		if data[len(data)-2] == 0x5a && data[len(data)-1] == 0x5a {
			data[len(data)-2] = 0x2e
			data[len(data)-1] = 0x2e
		}

		for i := 0; i < len(data); i++ {
			data[i] ^= pairingKeys[0]
			data[i] -= updatedRandomSyncKey
			data[i] = ((data[i] >> 4) & 0xf) | ((data[i] & 0xf) << 4)

			data[i] += pairingKeys[1]
			data[i] ^= pairingKeys[2]
			data[i] = ((data[i] >> 4) & 0xf) | ((data[i] & 0xf) << 4)

			data[i] -= pairingKeys[3]
			data[i] ^= pairingKeys[4]
			data[i] = ((data[i] >> 4) & 0x0f) | ((data[i] & 0x0f) << 4)

			data[i] ^= pairingKeys[5]
			data[i] ^= updatedRandomSyncKey

			data[i] ^= secondLvlEncryptionLookup[pairingKeys[0]]
			data[i] += secondLvlEncryptionLookup[pairingKeys[1]]
			data[i] -= secondLvlEncryptionLookup[pairingKeys[2]]
			data[i] = ((data[i] >> 4) & 0x0f) | ((data[i] & 0x0f) << 4)

			data[i] ^= secondLvlEncryptionLookup[pairingKeys[3]]
			data[i] += secondLvlEncryptionLookup[pairingKeys[4]]
			data[i] -= secondLvlEncryptionLookup[pairingKeys[5]]
			data[i] = ((data[i] >> 4) & 0x0f) | ((data[i] & 0x0f) << 4)

			data[i] ^= secondLvlEncryptionLookup[randomPairingKeys[0]]
			data[i] += secondLvlEncryptionLookup[randomPairingKeys[1]]
			data[i] -= secondLvlEncryptionLookup[randomPairingKeys[2]]

			updatedRandomSyncKey = data[i]
		}

		e.randomSyncKey = updatedRandomSyncKey
	} else if e.state.pumpType == PUMP_TYPE_DANA_I {
		if data[0] == 0xa5 && data[1] == 0xa5 {
			data[0] = 0xaa
			data[1] = 0xaa
		}

		if data[len(data)-2] == 0x5a && data[len(data)-1] == 0x5a {
			data[len(data)-2] = 0xee
			data[len(data)-1] = 0xee
		}

		for i := 0; i < len(data); i++ {
			data[i] += ble5RandomKeys[0]
			data[i] = ((data[i] >> 4) & 0x0f) | (((data[i] & 0x0f) << 4) & 0xf0)

			data[i] -= ble5RandomKeys[1]
			data[i] ^= ble5RandomKeys[2]
		}
	}

	return data
}

func (e *DanaEncryption) Decryption(data []byte, isEncryptionCommand bool) []byte {
	data = encodePacketSerialNumber(&data, e.state.name)

	if isEncryptionCommand && e.state.pumpType == PUMP_TYPE_DANA_RS_V1 {
		panic("DanaRSv1 not supported yet (Decryption !isSecure)")
	}

	if int(data[2]) != (len(data) - 7) {
		fmt.Println("ERROR: Invalid message received. Message too short - Data: " + base64.StdEncoding.EncodeToString(data))
		return []byte{}
	}

	var endContent = len(data) - 4
	var content = data[3:endContent]
	var crc = generateCrc(content, e.state.pumpType, isEncryptionCommand)

	if byte(crc>>8) != data[len(data)-4] || byte(crc&0xff) != data[len(data)-3] {
		fmt.Println("ERROR: Invalid message received. Mismatch CRC - Data: " + base64.StdEncoding.EncodeToString(data) + ", crc: " + fmt.Sprint(crc))
		return []byte{}
	}

	return content
}

func (e *DanaEncryption) DecryptionSecondLvl(data []byte) []byte {
	if e.state.pumpType == PUMP_TYPE_DANA_RS_V3 {
		for i := 0; i < len(data); i++ {
			copyRandomSyncKey := data[i]

			data[i] += secondLvlEncryptionLookup[randomPairingKeys[2]]
			data[i] -= secondLvlEncryptionLookup[randomPairingKeys[1]]
			data[i] ^= secondLvlEncryptionLookup[randomPairingKeys[0]]
			data[i] = ((data[i] >> 4) & 0xf) | (((data[i] & 0xf) << 4) & 0xff)

			data[i] += secondLvlEncryptionLookup[pairingKeys[5]]
			data[i] -= secondLvlEncryptionLookup[pairingKeys[4]]
			data[i] ^= secondLvlEncryptionLookup[pairingKeys[3]]
			data[i] = ((data[i] >> 4) & 0xf) | (((data[i] & 0xf) << 4) & 0xff)

			data[i] += secondLvlEncryptionLookup[pairingKeys[2]]
			data[i] -= secondLvlEncryptionLookup[pairingKeys[1]]
			data[i] ^= secondLvlEncryptionLookup[pairingKeys[0]]
			data[i] ^= e.randomSyncKey
			data[i] ^= pairingKeys[5]

			data[i] = ((data[i] >> 4) & 0xf) | (((data[i] & 0xf) << 4) & 0xff)
			data[i] ^= pairingKeys[4]
			data[i] += pairingKeys[3]

			data[i] = ((data[i] >> 4) & 0xf) | (((data[i] & 0xf) << 4) & 0xff)
			data[i] ^= pairingKeys[2]
			data[i] -= pairingKeys[1]

			data[i] = ((data[i] >> 4) & 0xf) | (((data[i] & 0xf) << 4) & 0xff)
			data[i] += e.randomSyncKey
			data[i] ^= pairingKeys[0]

			e.randomSyncKey = copyRandomSyncKey
		}

		if data[0] == 0x7a && data[1] == 0x7a {
			data[0] = 0xa5
			data[1] = 0xa5
		}

		if data[len(data)-2] == 0x2e && data[len(data)-1] == 0x2e {
			data[len(data)-2] = 0x5a
			data[len(data)-1] = 0x5a
		}
	} else if e.state.pumpType == PUMP_TYPE_DANA_I {
		for i := 0; i < len(data); i++ {
			data[i] ^= ble5RandomKeys[2]
			data[i] += ble5RandomKeys[1]

			data[i] = ((data[i] >> 4) & 0xf) | (((data[i] & 0xf) << 4) & 0xff)
			data[i] -= ble5RandomKeys[0]
		}
	}

	return data
}

func (e DanaEncryption) encodePumpCheck() []byte {
	var length byte = 0x04 // Default length of DanaRS-v1
	if e.state.pumpType == PUMP_TYPE_DANA_RS_V3 {
		length = 0x09
	} else if e.state.pumpType == PUMP_TYPE_DANA_I {
		length = 0x0c
	}

	var data = make([]byte, length)

	// Data
	if e.state.pumpType == PUMP_TYPE_DANA_I {
		// OK - response code
		data[0] = 0x4f // O
		data[1] = 0x4b // K

		data[2] = 0x4d // Unknown usage

		// Hardware model
		data[3] = 0x09
		data[4] = 0x50 // Unsure what this value is, but is unused

		// Firmware protocol
		data[5] = 0x13

		// BLE-5 keys
		data[6] = ble5Keys[0]
		data[7] = ble5Keys[1]
		data[8] = ble5Keys[2]
		data[9] = ble5Keys[3]
		data[10] = ble5Keys[4]
		data[11] = ble5Keys[5]
	} else if e.state.pumpType == PUMP_TYPE_DANA_RS_V3 {
		// Hardware model
		data[0] = 0x05
		data[1] = 0x00

		// Firmware protocol
		data[2] = 0x13
	} else {
		data[0] = 0x04
	}

	return e.encodeMessage(data, codes.OPCODE_ENCRYPTION__PUMP_CHECK, true)
}

func (e DanaEncryption) encodeTimeInformation() []byte {
	var length byte = 1

	var data = make([]byte, length)
	if e.state.pumpType == PUMP_TYPE_DANA_I {
		data[0] = 0x00
	}

	return e.encodeMessage(data, codes.OPCODE_ENCRYPTION__TIME_INFORMATION, true)
}

func (e DanaEncryption) encodeMessage(data []byte, opCode byte, isEncryptionCommand bool) []byte {
	var length = len(data)
	var buffer = make([]byte, 9+len(data))
	buffer[0] = 0xa5                // header 1
	buffer[1] = 0xa5                // header 2
	buffer[2] = byte(length) + 0x02 // length

	// Message type. Either RESPONSE or NOTIFY or ENCRYPTION_RESPONSE
	if isEncryptionCommand {
		buffer[3] = codes.TYPE_ENCRYPTION_RESPONSE
	} else {
		buffer[3] = codes.TYPE_RESPONSE
	}

	buffer[4] = opCode

	for i := 0; i < length; i++ {
		buffer[5+i] = data[i]
	}

	var crc = generateCrc(buffer[3:5+length], e.state.pumpType, isEncryptionCommand)
	buffer[5+length] = byte(crc >> 8)
	buffer[6+length] = byte(crc & 0xff)
	buffer[7+length] = 0x5a // footer 1
	buffer[8+length] = 0x5a // footer 2

	var encodedBuffer = encodePacketSerialNumber(&buffer, e.state.name)
	if e.state.pumpType == PUMP_TYPE_DANA_RS_V1 && isEncryptionCommand {
		encodedBuffer = encodePacketTime(&encodedBuffer, timeSecret)
		encodedBuffer = encodePacketPassword(&encodedBuffer, passwordSecret)
		encodedBuffer = encodePacketPassKey(&encodedBuffer, passKeySecret)
	}

	return encodedBuffer
}

func generateCrc(buffer []byte, pumpType int, isEncryptionCommand bool) uint16 {
	var crc uint16 = 0

	for index := range buffer {
		var result uint16 = ((crc >> 8) | (crc << 8)) ^ uint16(buffer[index])
		result ^= (result & 0xff) >> 4
		result ^= (result << 12)

		if pumpType == PUMP_TYPE_DANA_RS_V1 {
			var tmp uint16 = (result&0xff)<<3 | ((result&0xff)>>2)<<5
			result ^= tmp
		} else if pumpType == PUMP_TYPE_DANA_RS_V3 {
			var tmp uint16 = 0
			if isEncryptionCommand {
				tmp = (result&0xff)<<3 | ((result&0xff)>>2)<<5
			} else {
				tmp = (result&0xff)<<5 | ((result&0xff)>>4)<<2
			}
			result ^= tmp
		} else if pumpType == PUMP_TYPE_DANA_I {
			var tmp uint16 = 0
			if isEncryptionCommand {
				tmp = (result&0xff)<<3 | ((result&0xff)>>2)<<5
			} else {
				tmp = (result&0xff)<<4 | ((result&0xff)>>3)<<2
			}
			result ^= tmp
		}

		crc = result
	}

	return crc
}

func encodePacketPassKey(buffer *[]byte, passkeySecret []byte) []byte {
	for i := 0; i < len(*buffer)-5; i++ {
		(*buffer)[i+3] ^= passkeySecret[(i+1)%2]
	}

	return *buffer
}

func encodePacketPassKeySerialNumber(value uint8, deviceName string) uint8 {
	var tmp uint8 = 0
	for i := 0; i < min(10, len(deviceName)); i++ {
		charCode := uint8(deviceName[i])
		tmp = tmp + charCode
	}

	return value ^ tmp
}

func encodePacketPassword(buffer *[]byte, passwordSecret []byte) []byte {
	tmp := passwordSecret[0] + passwordSecret[1]
	for i := 3; i < len(*buffer)-2; i++ {
		(*buffer)[i] ^= tmp
	}

	return *buffer
}

func encodePacketSerialNumber(buffer *[]byte, deviceName string) []byte {
	tmp := []byte{
		uint8(deviceName[0]) + uint8(deviceName[1]) + uint8(deviceName[2]),
		uint8(deviceName[3]) + uint8(deviceName[4]) + uint8(deviceName[5]) + uint8(deviceName[6]) + uint8(deviceName[7]),
		uint8(deviceName[8]) + uint8(deviceName[9]),
	}

	for i := 0; i < len(*buffer)-5; i++ {
		(*buffer)[i+3] ^= tmp[i%3]
	}

	return *buffer
}

func encodePacketTime(buffer *[]byte, timeSecret []byte) []byte {
	tmp := byte(0)
	for _, v := range timeSecret {
		tmp += v
	}

	for i := 3; i < len(*buffer)-2; i++ {
		(*buffer)[i] ^= tmp
	}

	return *buffer
}

func encodePairingKey(buffer *[]byte, pairingKey []byte, globalRandomSyncKey uint8) (uint8, []byte) {
	newRandomSyncKey := globalRandomSyncKey

	for i, v := range *buffer {
		(*buffer)[i] ^= pairingKey[0]
		(*buffer)[i] -= newRandomSyncKey
		(*buffer)[i] = ((v >> 4) & 0xF) | ((v & 0xF) << 4)

		(*buffer)[i] += pairingKey[1]
		(*buffer)[i] ^= pairingKey[2]
		(*buffer)[i] = ((v >> 4) & 0xF) | ((v & 0xF) << 4)

		(*buffer)[i] -= pairingKey[3]
		(*buffer)[i] ^= pairingKey[4]
		(*buffer)[i] = ((v >> 4) & 0xF) | ((v & 0xF) << 4)

		(*buffer)[i] ^= pairingKey[5]
		(*buffer)[i] ^= newRandomSyncKey

		newRandomSyncKey = v
	}

	return newRandomSyncKey, *buffer
}

func getDescPairingKey(buffer *[]byte, pairingKey []byte, globalRandomSyncKey uint8) (uint8, []byte) {
	newRandomSyncKey := globalRandomSyncKey

	for i, v := range *buffer {
		tmp := v

		(*buffer)[i] ^= newRandomSyncKey
		(*buffer)[i] ^= pairingKey[5]

		(*buffer)[i] = ((v >> 4) & 0xF) | ((v & 0xF) << 4)
		(*buffer)[i] ^= pairingKey[4]
		(*buffer)[i] -= pairingKey[3]

		(*buffer)[i] = ((v >> 4) & 0xF) | ((v & 0xF) << 4)
		(*buffer)[i] ^= pairingKey[2]
		(*buffer)[i] += pairingKey[1]
		(*buffer)[i] ^= pairingKey[0]

		(*buffer)[i] = ((v >> 4) & 0xF) | ((v & 0xF) << 4)
		(*buffer)[i] -= newRandomSyncKey

		newRandomSyncKey = tmp
	}

	return newRandomSyncKey, *buffer
}

func encryptionRandomSyncKey(randomSyncKey uint8, randomPairingKey []byte) uint8 {
	var tmp uint8 = 0

	tmp = ((randomSyncKey >> 4) | ((randomSyncKey & 0xF) << 4)) + randomPairingKey[0]
	tmp = ((tmp >> 4) | ((tmp & 0xF) << 4)) ^ randomPairingKey[1]

	return ((tmp >> 4) | ((tmp & 0xF) << 4)) - randomPairingKey[2]
}

func decryptionRandomSyncKey(randomSyncKey uint8, randomPairingKey []byte) uint8 {
	var tmp uint8 = 0

	tmp = ((randomSyncKey + randomPairingKey[2]) >> 4) | ((((randomSyncKey + randomPairingKey[2]) & 0xF) << 4) ^ randomPairingKey[1])
	tmp = ((tmp >> 4) | ((tmp & 0xF) << 4)) - randomPairingKey[0]

	return (tmp >> 4) | ((tmp & 0xF) << 4)
}

func initialRandomSyncKey(pairingKey []byte) uint8 {
	var tmp uint8 = 0

	tmp = (((pairingKey[0] + pairingKey[1]) >> 4) | (((pairingKey[0] + pairingKey[1]) & 0xF) << 4) ^ pairingKey[2]) - pairingKey[3]
	tmp = ((tmp >> 4) | ((tmp & 0xF) << 4)) ^ pairingKey[4]

	return ((tmp >> 4) | ((tmp & 0xF) << 4)) ^ pairingKey[5]
}

var secondLvlEncryptionLookup = []byte{
	0x63, 0x7c, 0x77, 0x7b, 0xf2, 0x6b, 0x6f, 0xc5, 0x30, 0x01, 0x67, 0x2b, 0xfe, 0xd7, 0xab, 0x76, 0xca, 0x82, 0xc9, 0x7d, 0xfa, 0x59, 0x47, 0xf0, 0xad, 0xd4,
	0xa2, 0xaf, 0x9c, 0xa4, 0x72, 0xc0, 0xb7, 0xfd, 0x93, 0x26, 0x36, 0x3f, 0xf7, 0xcc, 0x34, 0xa5, 0xe5, 0xf1, 0x71, 0xd8, 0x31, 0x15, 0x04, 0xc7, 0x23, 0xc3,
	0x18, 0x96, 0x05, 0x9a, 0x07, 0x12, 0x80, 0xe2, 0xeb, 0x27, 0xb2, 0x75, 0x09, 0x83, 0x2c, 0x1a, 0x1b, 0x6e, 0x5a, 0xa0, 0x52, 0x3b, 0xd6, 0xb3, 0x29, 0xe3,
	0x2f, 0x84, 0x53, 0xd1, 0x00, 0xed, 0x20, 0xfc, 0xb1, 0x5b, 0x6a, 0xcb, 0xbe, 0x39, 0x4a, 0x4c, 0x58, 0xcf, 0xd0, 0xef, 0xaa, 0xfb, 0x43, 0x4d, 0x33, 0x85,
	0x45, 0xf9, 0x02, 0x7f, 0x50, 0x3c, 0x9f, 0xa8, 0x51, 0xa3, 0x40, 0x8f, 0x92, 0x9d, 0x38, 0xf5, 0xbc, 0xb6, 0xda, 0x21, 0x10, 0xff, 0xf3, 0xd2, 0xcd, 0x0c,
	0x13, 0xec, 0x5f, 0x97, 0x44, 0x17, 0xc4, 0xa7, 0x7e, 0x3d, 0x64, 0x5d, 0x19, 0x73, 0x60, 0x81, 0x4f, 0xdc, 0x22, 0x2a, 0x90, 0x88, 0x46, 0xee, 0xb8, 0x14,
	0xde, 0x5e, 0x0b, 0xdb, 0xe0, 0x32, 0x3a, 0x0a, 0x49, 0x06, 0x24, 0x5c, 0xc2, 0xd3, 0xac, 0x62, 0x91, 0x95, 0xe4, 0x79, 0xe7, 0xc8, 0x37, 0x6d, 0x8d, 0xd5,
	0x4e, 0xa9, 0x6c, 0x56, 0xf4, 0xea, 0x65, 0x7a, 0xae, 0x08, 0xba, 0x78, 0x25, 0x2e, 0x1c, 0xa6, 0xb4, 0xc6, 0xe8, 0xdd, 0x74, 0x1f, 0x4b, 0xbd, 0x8b, 0x8a,
	0x70, 0x3e, 0xb5, 0x66, 0x48, 0x03, 0xf6, 0x0e, 0x61, 0x35, 0x57, 0xb9, 0x86, 0xc1, 0x1d, 0x9e, 0xe1, 0xf8, 0x98, 0x11, 0x69, 0xd9, 0x8e, 0x94, 0x9b, 0x1e,
	0x87, 0xe9, 0xce, 0x55, 0x28, 0xdf, 0x8c, 0xa1, 0x89, 0x0d, 0xbf, 0xe6, 0x42, 0x68, 0x41, 0x99, 0x2d, 0x0f, 0xb0, 0x54, 0xbb, 0x16,
}

var secondLvlEncryptionLookupShort = []byte{
	0x63, 0x7c, 0x77, 0x7b, 0xf2, 0x6b, 0x6f, 0xc5, 0x30, 0x01, 0x67, 0x2b, 0xfe, 0xd7, 0xab, 0x76, 0x6c, 0x70, 0x48, 0x50, 0xfd, 0xed, 0xb9, 0xda, 0x5e, 0x15,
	0x46, 0x57, 0xa7, 0x8d, 0x9d, 0x84, 0xb7, 0xfd, 0x93, 0x26, 0x36, 0x3f, 0xf7, 0xcc, 0x34, 0xa5, 0xe5, 0xf1, 0x71, 0xd8, 0x31, 0x15, 0x47, 0xf1, 0x1a, 0x71,
	0x1d, 0x29, 0xc5, 0x89, 0x6f, 0xb7, 0x62, 0x0e, 0xaa, 0x18, 0xbe, 0x1b, 0x09, 0x83, 0x2c, 0x1a, 0x1b, 0x6e, 0x5a, 0xa0, 0x52, 0x3b, 0xd6, 0xb3, 0x29, 0xe3,
	0x2f, 0x84, 0x53, 0xd1, 0xa0, 0xed, 0x20, 0xfc, 0xb1, 0x5b, 0x6a, 0xcb, 0xbe, 0x39, 0x4a, 0x4c, 0x58, 0xcf, 0xb0, 0x54, 0xbb, 0x16,
}
