package server

import (
	"encoding/base64"
	"fmt"
	"math"
	"time"

	"tinygo.org/x/bluetooth"
)

type CommandCenter struct {
	encryption          DanaEncryption
	state               *SimulatorState
	writeCharacteristic *bluetooth.Characteristic
}

func (c *CommandCenter) ProcessEncryptionCommand(data []byte) {
	switch data[1] {
	case OPCODE_ENCRYPTION__PUMP_CHECK:
		c.respondToCommandRequest()
		return
	case OPCODE_ENCRYPTION__TIME_INFORMATION:
		c.respondToTimeRequest()
		return
	}

	fmt.Println("ERROR: UNIMPLEMENTED ENCRYPTION COMMAND: " + fmt.Sprint(data[1]))
}

func (c *CommandCenter) ProcessCommand(data []byte) {
	var command = data[1]

	if !c.state.isInHistoryUploadMode && command >= OPCODE_REVIEW__BOLUS_AVG && command <= OPCODE_REVIEW__ALL_HISTORY {
		fmt.Println("ERROR: Trying to do a history command while not in history upload mode...")
		return
	}

	if command >= OPCODE_REVIEW__BOLUS_AVG && command <= OPCODE_REVIEW__ALL_HISTORY {
		var date = getDate(data, 0, time.Local)
		c.respondToHistoryRequest(command, date)
		return
	}

	switch data[1] {
	case OPCODE_ETC__KEEP_CONNECTION:
		c.respondToKeepConnection()
		return
	case OPCODE_REVIEW__INITIAL_SCREEN_INFORMATION:
		c.respondToInitialScreenInformation()
		return
	case OPCODE_OPTION__GET_PUMP_TIME:
		c.respondToGetTime()
		return
	case OPCODE_OPTION__GET_PUMP_UTC_AND_TIME_ZONE:
		c.respondToGetTimeWithUtc()
		return
	case OPCODE_OPTION__SET_PUMP_TIME:
		c.respondToSetTime(data)
		return
	case OPCODE_OPTION__GET_USER_OPTION:
		c.respondToGetUserOptions()
		return
	case OPCODE_REVIEW__SET_HISTORY_UPLOAD_MODE:
		c.respondToSetHistoryMode(data[2] == 1)
		return
	}

	fmt.Println("ERROR: UNIMPLEMENTED COMMAND: " + fmt.Sprint(data[1]))
}

func (c *CommandCenter) respondToCommandRequest() {
	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_ENCRYPTION__PUMP_CHECK, data: []byte{}})

	fmt.Println("INFO: Sending OPCODE_ENCRYPTION__PUMP_CHECK - Data: " + base64.StdEncoding.EncodeToString(data))
	c.write(data)
}

func (c *CommandCenter) respondToTimeRequest() {
	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_ENCRYPTION__TIME_INFORMATION, data: []byte{}})

	fmt.Println("INFO: Sending OPCODE_ENCRYPTION__TIME_INFORMATION - Data: " + base64.StdEncoding.EncodeToString(data))
	c.write(data)
}

func (c CommandCenter) respondToKeepConnection() {
	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_ETC__KEEP_CONNECTION, data: []byte{0}})
	data = c.encryption.EncryptionSecondLvl(data)

	fmt.Println("INFO: Sending OPCODE_ETC__KEEP_CONNECTION - Data: " + base64.StdEncoding.EncodeToString([]byte{0}))
	c.write(data)
}

func (c CommandCenter) respondToInitialScreenInformation() {
	// TODO: Add isExtendedInProgress & isDualBolusInProgress
	var status byte = 0
	if c.state.isSuspended {
		status += 0x01
	}
	if c.state.tempBasalActiveTill != nil {
		status += 0x10
	}

	var length = 15
	if c.state.pumpType == PUMP_TYPE_DANA_I {
		length = 16
	}
	var message = make([]byte, length)
	message[0] = status

	// dailyTotalUnits - Not used
	message[1] = 0
	message[2] = 0

	// maxDailyTotalUnits - Not used
	message[3] = 0
	message[4] = 250

	var reservoirLevel = int(c.state.reservoirLevel * 100)
	message[5] = byte(reservoirLevel << 8)
	message[6] = byte(reservoirLevel)

	// currentBasal - Not used
	message[7] = 0
	message[8] = 0

	// tempBasalPercent - Not used
	message[9] = byte(c.state.tempBasalPercentage)

	// batteryRemaining
	message[10] = byte(c.state.batteryRemaining)

	// extendedBolusAbsoluteRemaining - Not used
	message[11] = 0
	message[12] = 0

	// insulinOnBoard - Not used
	message[13] = 0
	message[14] = 0

	if c.state.pumpType == PUMP_TYPE_DANA_I {
		// error state - Not used
		message[15] = 0
	}

	fmt.Println("INFO: Sending OPCODE_REVIEW__INITIAL_SCREEN_INFORMATION - Data: " + base64.StdEncoding.EncodeToString(message))

	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_REVIEW__INITIAL_SCREEN_INFORMATION, data: message})
	data = c.encryption.EncryptionSecondLvl(data)

	c.write(data)
}

func (c CommandCenter) respondToGetTime() {
	var duration = time.Duration(c.state.pumpTimeSkewInSeconds * int(time.Second))
	var time = time.Now().Add(duration)

	var message = make([]byte, 6)
	message[0] = byte(time.Year() - 2000)
	message[1] = byte(time.Month())
	message[2] = byte(time.Day())
	message[3] = byte(time.Hour())
	message[4] = byte(time.Minute())
	message[5] = byte(time.Second())

	fmt.Println("INFO: Sending OPCODE_OPTION__GET_PUMP_TIME - Data: " + base64.StdEncoding.EncodeToString(message))

	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_OPTION__GET_PUMP_TIME, data: message})
	data = c.encryption.EncryptionSecondLvl(data)

	c.write(data)
}

func (c CommandCenter) respondToGetTimeWithUtc() {
	if c.state.pumpType != PUMP_TYPE_DANA_I {
		fmt.Println("WARNING: OPCODE_OPTION__GET_PUMP_UTC_AND_TIME_ZONE is only supported on the Dana-I")
		return
	}

	var duration = time.Duration(c.state.pumpTimeSkewInSeconds * int(time.Second))

	var timeZone = time.FixedZone("EDT", c.state.pumpTimeZoneOffsetInSeconds)
	var time = time.Now().Add(duration).In(timeZone)

	var message = make([]byte, 7)
	message[0] = byte(time.UTC().Year() - 2000)
	message[1] = byte(time.UTC().Month())
	message[2] = byte(time.UTC().Day())
	message[3] = byte(time.UTC().Hour())
	message[4] = byte(time.UTC().Minute())
	message[5] = byte(time.UTC().Second())
	message[6] = byte(c.state.pumpTimeZoneOffsetInSeconds / 3600)

	fmt.Println("INFO: Sending OPCODE_OPTION__GET_PUMP_TIME - Data: " + base64.StdEncoding.EncodeToString(message))

	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_OPTION__GET_PUMP_UTC_AND_TIME_ZONE, data: message})
	data = c.encryption.EncryptionSecondLvl(data)

	c.write(data)
}

func (c CommandCenter) respondToGetUserOptions() {
	var length = 18
	if c.state.pumpType == PUMP_TYPE_DANA_I {
		length = 20
	}

	var message = make([]byte, length)
	message[0] = 0
	if c.state.timeDisplayIn24H {
		message[0] = 1
	}

	message[1] = 0
	if c.state.buttonScroll {
		message[1] = 1
	}

	message[2] = byte(c.state.beepAndAlarm)
	message[3] = byte(c.state.lcdOnInSeconds)
	message[4] = byte(c.state.backlightOnInSeconds)
	message[5] = byte(c.state.selectedLanguage)
	message[6] = byte(c.state.units)
	message[7] = byte(c.state.shutdownInHours)
	message[8] = byte(c.state.lowReservoirWarning)
	message[9] = byte(c.state.cannulaVolume << 8)
	message[10] = byte(c.state.cannulaVolume)
	message[11] = byte(c.state.refillAmount << 8)
	message[12] = byte(c.state.refillAmount)
	message[13] = 1 // Selectable language 1
	message[14] = 1 // Selectable language 2
	message[15] = 1 // Selectable language 3
	message[16] = 1 // Selectable language 4
	message[17] = 1 // Selectable language 5

	if c.state.pumpType == PUMP_TYPE_DANA_I {
		message[18] = byte(c.state.targetBg << 8)
		message[19] = byte(c.state.targetBg)
	}

	fmt.Println("INFO: Sending OPCODE_OPTION__GET_USER_OPTION - Data: " + base64.StdEncoding.EncodeToString(message))

	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_OPTION__GET_USER_OPTION, data: message})
	data = c.encryption.EncryptionSecondLvl(data)

	c.write(data)
}

func (c *CommandCenter) respondToSetHistoryMode(enabled bool) {
	c.state.isInHistoryUploadMode = enabled

	fmt.Println("INFO: Sending OPCODE_REVIEW__SET_HISTORY_UPLOAD_MODE - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))

	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_REVIEW__SET_HISTORY_UPLOAD_MODE, data: []byte{0x00}})
	data = c.encryption.EncryptionSecondLvl(data)

	c.write(data)
}

func (c *CommandCenter) respondToHistoryRequest(code byte, from time.Time) {
	var filterOnDate = func(h HistoryItem) bool { return h.timestamp.After(from) }
	var filterOnCode = func(h HistoryItem) bool { return h.code == code }

	var items = filter(c.state.history, filterOnDate)
	if code != OPCODE_REVIEW__ALL_HISTORY {
		items = filter(items, filterOnCode)
	}

	fmt.Println("INFO: Uploading history items. Count: " + fmt.Sprint(items))

	for _, item := range items {
		var message = make([]byte, 11)
		message[0] = item.code - 0x0f
		message[1] = byte(item.timestamp.Year() - 2000)
		message[2] = byte(item.timestamp.Month())
		message[3] = byte(item.timestamp.Day())
		message[4] = byte(item.timestamp.Hour())
		message[5] = byte(item.timestamp.Minute())
		message[6] = byte(item.timestamp.Second())
		message[7] = byte(item.param7)
		message[8] = byte(item.param8)
		message[9] = byte(item.value << 8)
		message[10] = byte(item.value)

		var data = c.encryption.Encryption(EncryptionParams{operationCode: code, data: message})
		data = c.encryption.EncryptionSecondLvl(data)

		fmt.Println("INFO: Sending hisory item - Data: " + base64.StdEncoding.EncodeToString(message))
		c.write(data)
	}

	// Send upload done message
	var message = make([]byte, 3)
	message[0] = 0
	message[1] = 0
	message[2] = 0

	var data = c.encryption.Encryption(EncryptionParams{operationCode: code, data: message})
	data = c.encryption.EncryptionSecondLvl(data)

	fmt.Println("INFO: Done uploading history - Data: " + base64.StdEncoding.EncodeToString(message))
	c.write(data)
}

func (c *CommandCenter) respondToSetTime(request []byte) {
	var pumpTime = time.Now().Add(time.Duration(c.state.pumpTimeSkewInSeconds * int(time.Second)))
	var requestTime = getDate(request, 0, time.Local)

	var diff = pumpTime.Sub(requestTime)
	c.state.pumpTimeSkewInSeconds = int(diff.Seconds())

	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_OPTION__SET_PUMP_TIME, data: []byte{0x00}})
	data = c.encryption.EncryptionSecondLvl(data)

	fmt.Println("INFO: Done uploading history - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.write(data)
}

func (c *CommandCenter) respondToSetTimeWithUtc(request []byte) {
	var pumpTime = time.Now().UTC().Add(time.Duration(c.state.pumpTimeSkewInSeconds * int(time.Second)))
	var requestTime = getDate(request, 0, time.UTC)

	var diff = pumpTime.Sub(requestTime)
	c.state.pumpTimeSkewInSeconds = int(diff.Seconds())

	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_OPTION__SET_PUMP_TIME, data: []byte{0x00}})
	data = c.encryption.EncryptionSecondLvl(data)

	c.state.pumpTimeZoneOffsetInSeconds = int(request[8]) * 3600

	fmt.Println("INFO: Done uploading history - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.write(data)
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

func getDate(data []byte, startIndex int, loc *time.Location) time.Time {
	return time.Date(
		int(data[startIndex+2])+2000,
		time.Month(data[startIndex+3]),
		int(data[startIndex+4]),
		int(data[startIndex+5]),
		int(data[startIndex+6]),
		int(data[startIndex+7]),
		0,
		loc,
	)
}

func filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}
