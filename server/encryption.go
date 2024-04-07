package server

import (
	codes "dana/simulator/server/packets"
)

type DanaEncryption struct {
	deviceName         string
	deviceType         int
	enhancedEncryption int
	passwordSecret     []byte
	timeSecret         []byte
	passKeySecret      []byte
}

type EncryptionParams struct {
	operationCode byte
	data          []byte
}

func (e DanaEncryption) Encryption(params EncryptionParams) []byte {
	switch params.operationCode {
	case codes.OPCODE_ENCRYPTION__PUMP_CHECK:
		return e.encodePumpCheck()
	}

	return e.encodeMessage(params.data, params.operationCode, false)
}

func (e DanaEncryption) encodePumpCheck() []byte {
	var length byte = 0x04 // Default length of DanaRS-v1
	if e.deviceType == DanaRSv3 {
		length = 0x09
	} else if e.deviceType == DanaI {
		length = 0x0e
	}

	var data = make([]byte, 0, length)

	// Data
	if e.deviceType == DanaI {
		// Hardware model
		data[0] = 0x09
		data[1] = 0x00

		// Firmware protocol
		data[2] = 0x13

		// BLE-5 keys
		data[3] = 0x00
		data[4] = 0x00
		data[5] = 0x00
		data[6] = 0x00
		data[7] = 0x00
		data[8] = 0x00
	} else if e.deviceType == DanaRSv3 {
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

func (e DanaEncryption) encodeMessage(data []byte, opCode byte, isEncryptionCommand bool) []byte {
	var length = 0x02 + byte(len(data))
	var buffer = make([]byte, 0, 5+len(data))
	buffer[0] = 0xa5   // header 1
	buffer[1] = 0xa5   // header 2
	buffer[2] = length // length

	// Message type. Either RESPONSE or NOTIFY or ENCRYPTION_RESPONSE
	if isEncryptionCommand {
		buffer[3] = codes.TYPE_ENCRYPTION_RESPONSE
	} else {
		buffer[3] = codes.TYPE_RESPONSE
	}

	buffer[4] = opCode

	for i := 0; i < len(data); i++ {
		buffer[5+i] = data[i]
	}

	var crc = generateCrc(buffer[3:3+buffer[2]], e.enhancedEncryption, isEncryptionCommand)
	buffer[5+length] = byte(crc >> 8)
	buffer[6+length] = byte(8 & 0xff)
	buffer[7+length] = 0x5a // footer 1
	buffer[8+length] = 0x5a // footer 2

	var encodedBuffer = encodePacketSerialNumber(&buffer, e.deviceName)
	if e.enhancedEncryption == 0 && isEncryptionCommand {
		encodedBuffer = encodePacketTime(&encodedBuffer, e.timeSecret)
		encodedBuffer = encodePacketPassword(&encodedBuffer, e.passwordSecret)
		encodedBuffer = encodePacketPassKey(&encodedBuffer, e.passKeySecret)
	}

	return encodedBuffer
}

func generateCrc(buffer []byte, enhancedEncryption int, isEncryptionCommand bool) uint16 {
	var crc uint16 = 0

	for _, byte := range buffer {
		result := ((crc >> 8) | (crc << 8)) ^ uint16(byte)
		result ^= (result & 0xff) >> 4
		result ^= (result << 12)

		if enhancedEncryption == 0 {
			tmp := (result&0xff)<<3 | ((result&0xff)>>2)<<5
			result ^= tmp
		} else if enhancedEncryption == 1 {
			var tmp uint16 = 0
			if isEncryptionCommand {
				tmp = (result&0xff)<<3 | ((result&0xff)>>2)<<5
			} else {
				tmp = (result&0xff)<<5 | ((result&0xff)>>4)<<2
			}
			result ^= tmp
		} else if enhancedEncryption == 2 {
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
