package server

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"tinygo.org/x/bluetooth"
)

type CommandCenter struct {
	encryption          *DanaEncryption
	state               *SimulatorState
	writeCharacteristic *bluetooth.Characteristic

	bolusTicker   *time.Ticker
	currentAmount float32
}

func (c *CommandCenter) ProcessEncryptionCommand(data []byte) {
	switch data[1] {
	case OPCODE_ENCRYPTION__PUMP_CHECK:
		c.respondToCommandRequest()
		return
	case OPCODE_ENCRYPTION__TIME_INFORMATION:
		c.respondToTimeRequest(data)
		return
	}

	fmt.Println(time.Now().Format(time.RFC3339) + " ERROR: UNIMPLEMENTED ENCRYPTION COMMAND: " + fmt.Sprint(data[1]))
}

func (c *CommandCenter) ProcessCommand(data []byte) {
	var command = data[1]

	if !c.state.IsInHistoryUploadMode && command >= OPCODE_REVIEW__BOLUS_AVG && command <= OPCODE_REVIEW__ALL_HISTORY {
		fmt.Println(time.Now().Format(time.RFC3339) + " ERROR: Trying to do a history command while not in history upload mode...")
		return
	}

	if command >= OPCODE_REVIEW__BOLUS_AVG && command <= OPCODE_REVIEW__ALL_HISTORY {
		var date = getDate(data, 0, time.Local)
		c.respondToHistoryRequest(command, date)
		return
	}

	switch command {
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
	case OPCODE_OPTION__SET_PUMP_UTC_AND_TIME_ZONE:
		c.respondToSetTimeWithUtc(data)
		return
	case OPCODE_OPTION__GET_USER_OPTION:
		c.respondToGetUserOptions()
		return
	case OPCODE_OPTION__SET_USER_OPTION:
		c.respondToSetUserOptions(data)
		return
	case OPCODE_REVIEW__SET_HISTORY_UPLOAD_MODE:
		c.respondToSetHistoryMode(data[2] == 1)
		return
	case OPCODE_BOLUS__SET_STEP_BOLUS_START:
		c.respondToBolusStart(data)
		return
	case OPCODE_BOLUS__SET_STEP_BOLUS_STOP:
		c.respondToCancelBolus()
		return
	case OPCODE_BASAL__SET_PROFILE_BASAL_RATE:
		c.respondToSetBasal(data)
		return
	case OPCODE_BASAL__SET_PROFILE_NUMBER:
		c.respondToSetBasalProfile()
		return
	case OPCODE_BASAL__SET_SUSPEND_ON:
		c.respondToSuspend(true)
		return
	case OPCODE_BASAL__SET_SUSPEND_OFF:
		c.respondToSuspend(false)
		return
	case OPCODE_BASAL__SET_TEMPORARY_BASAL:
		c.respondToTempBasal(OPCODE_BASAL__SET_TEMPORARY_BASAL, int(data[2]), time.Duration(data[3])*time.Hour)
		return
	case OPCODE_BASAL__APS_SET_TEMPORARY_BASAL:
		var percentage = int(data[2]) + (int(data[3]) << 8)
		var duration time.Duration = time.Duration(15 * time.Second)
		if data[4] == 160 {
			duration = time.Duration(30 * time.Second)
		}

		c.respondToTempBasal(OPCODE_BASAL__APS_SET_TEMPORARY_BASAL, percentage, duration)
		return
	case OPCODE_BASAL__CANCEL_TEMPORARY_BASAL:
		c.respondToStopTempBasal()
		return
	case OPCODE_BASAL__GET_BASAL_RATE:
		c.respondToBasalGetRate()
		return
	case OPCODE_BOLUS__GET_STEP_BOLUS_INFORMATION:
		c.respondToBolusStepInformation()
		return
	}

	fmt.Println(time.Now().Format(time.RFC3339) + " ERROR: UNIMPLEMENTED COMMAND: " + fmt.Sprint(data[1]))
}

func (c *CommandCenter) respondToCommandRequest() {
	if c.bolusTicker != nil {
		fmt.Println(time.Now().Format(time.RFC3339) + " ERROR: Bolus is running... No new connections can be accepted - Sending BUSY")
		c.write(c.encryption.EncodePumpBusy())
		return
	}

	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_ENCRYPTION__PUMP_CHECK, data: []byte{}, isEncryptionCommand: true})

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_ENCRYPTION__PUMP_CHECK - Data: " + base64.StdEncoding.EncodeToString(data))
	c.write(data)
}

func (c *CommandCenter) respondToTimeRequest(request []byte) {
	if request[2] == 1 {
		c.encryption.ResetRandomSyncKey()

		fmt.Println("---------------------------------------")
		fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Pairing key: " + hex.EncodeToString(pairingKeys))
		fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Random pairing key: " + hex.EncodeToString(randomPairingKeys))
		fmt.Println("---------------------------------------")
	}

	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_ENCRYPTION__TIME_INFORMATION, data: []byte{}, isEncryptionCommand: true})

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_ENCRYPTION__TIME_INFORMATION - Data: " + base64.StdEncoding.EncodeToString(data))
	c.write(data)
}

func (c CommandCenter) respondToKeepConnection() {
	var data = c.encryption.Encryption(EncryptionParams{operationCode: OPCODE_ETC__KEEP_CONNECTION, data: []byte{0}, isEncryptionCommand: false})
	data = c.encryption.EncryptionSecondLvl(data)

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_ETC__KEEP_CONNECTION - Data: " + base64.StdEncoding.EncodeToString([]byte{0}))
	c.write(data)
}

func (c CommandCenter) respondToInitialScreenInformation() {
	// TODO: Add isExtendedInProgress & isDualBolusInProgress
	var status byte = 0
	if c.state.IsSuspended {
		status += 0x01
	}
	if c.state.TempBasalActiveTill != nil {
		status += 0x10
	}

	var length = 15
	if c.state.PumpType == PUMP_TYPE_DANA_I {
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

	var reservoirLevel = int(c.state.ReservoirLevel * 100)
	message[5] = byte(reservoirLevel)
	message[6] = byte(reservoirLevel >> 8)

	// currentBasal
	var currentBasal = int(c.currentBasal() * 100)
	message[7] = byte(currentBasal)
	message[8] = byte(currentBasal >> 8)

	// tempBasalPercent - Not used
	message[9] = byte(c.state.TempBasalPercentage)

	// batteryRemaining
	message[10] = byte(c.state.BatteryRemaining)

	// extendedBolusAbsoluteRemaining - Not used
	message[11] = 0
	message[12] = 0

	// insulinOnBoard - Not used
	message[13] = 0
	message[14] = 0

	if c.state.PumpType == PUMP_TYPE_DANA_I {
		// error state - Not used
		message[15] = 0
	}

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_REVIEW__INITIAL_SCREEN_INFORMATION - Data: " + base64.StdEncoding.EncodeToString(message))
	c.encodeAndWrite(OPCODE_REVIEW__INITIAL_SCREEN_INFORMATION, message)
}

func (c CommandCenter) respondToGetTime() {
	var duration = time.Duration(c.state.PumpTimeSkewInSeconds * int(time.Second))
	var now = time.Now().Add(duration)

	var message = make([]byte, 6)
	message[0] = byte(now.Year() - 2000)
	message[1] = byte(now.Month())
	message[2] = byte(now.Day())
	message[3] = byte(now.Hour())
	message[4] = byte(now.Minute())
	message[5] = byte(now.Second())

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_OPTION__GET_PUMP_TIME - Data: " + base64.StdEncoding.EncodeToString(message))
	c.encodeAndWrite(OPCODE_OPTION__GET_PUMP_TIME, message)
}

func (c CommandCenter) respondToGetTimeWithUtc() {
	if c.state.PumpType != PUMP_TYPE_DANA_I {
		fmt.Println(time.Now().Format(time.RFC3339) + " WARNING: OPCODE_OPTION__GET_PUMP_UTC_AND_TIME_ZONE is only supported on the Dana-I")
		return
	}

	var duration = time.Duration(c.state.PumpTimeSkewInSeconds * int(time.Second))

	var timeZone = time.FixedZone("EDT", c.state.PumpTimeZoneOffsetInSeconds)
	var now = time.Now().Add(duration).In(timeZone).UTC()

	var message = make([]byte, 7)
	message[0] = byte(now.Year() - 2000)
	message[1] = byte(now.Month())
	message[2] = byte(now.Day())
	message[3] = byte(now.Hour())
	message[4] = byte(now.Minute())
	message[5] = byte(now.Second())
	message[6] = byte(c.state.PumpTimeZoneOffsetInSeconds / 3600)

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_OPTION__GET_PUMP_TIME - Data: " + base64.StdEncoding.EncodeToString(message))
	c.encodeAndWrite(OPCODE_OPTION__GET_PUMP_UTC_AND_TIME_ZONE, message)
}

func (c CommandCenter) respondToGetUserOptions() {
	var length = 18
	if c.state.PumpType == PUMP_TYPE_DANA_I {
		length = 20
	}

	var message = make([]byte, length)
	message[0] = 0
	if c.state.TimeDisplayIn12H {
		message[0] = 1
	}

	message[1] = 0
	if c.state.ButtonScroll {
		message[1] = 1
	}

	message[2] = byte(c.state.BeepAndAlarm)
	message[3] = byte(c.state.LcdOnInSeconds)
	message[4] = byte(c.state.BacklightOnInSeconds)
	message[5] = byte(c.state.SelectedLanguage)
	message[6] = byte(c.state.Units)
	message[7] = byte(c.state.ShutdownInHours)
	message[8] = byte(c.state.LowReservoirWarning)
	message[9] = byte(c.state.CannulaVolume)
	message[10] = byte(c.state.CannulaVolume >> 8)
	message[11] = byte(c.state.RefillAmount)
	message[12] = byte(c.state.RefillAmount >> 8)
	message[13] = 1 // Selectable language 1
	message[14] = 1 // Selectable language 2
	message[15] = 1 // Selectable language 3
	message[16] = 1 // Selectable language 4
	message[17] = 1 // Selectable language 5

	if c.state.PumpType == PUMP_TYPE_DANA_I {
		message[18] = byte(c.state.TargetBg)
		message[19] = byte(c.state.TargetBg >> 8)
	}

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_OPTION__GET_USER_OPTION - Data: " + base64.StdEncoding.EncodeToString(message))
	c.encodeAndWrite(OPCODE_OPTION__GET_USER_OPTION, message)
}

func (c *CommandCenter) respondToSetUserOptions(request []byte) {
	c.state.TimeDisplayIn12H = request[2] == 0x01
	c.state.ButtonScroll = request[3] == 0x01
	c.state.BeepAndAlarm = int(request[4])
	c.state.LcdOnInSeconds = int(request[5])
	c.state.BacklightOnInSeconds = int(request[6])
	c.state.SelectedLanguage = int(request[7])
	c.state.Units = int(request[8])
	c.state.ShutdownInHours = int(request[9])
	c.state.LowReservoirWarning = int(request[10])
	c.state.CannulaVolume = int(request[11]) | (int(request[12]) << 8)
	c.state.RefillAmount = int(request[13]) | (int(request[14]) << 8)

	if c.state.PumpType == PUMP_TYPE_DANA_I {
		c.state.TargetBg = int(request[15]) | (int(request[16]) << 8)
	}

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_OPTION__SET_USER_OPTION - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.encodeAndWrite(OPCODE_OPTION__SET_USER_OPTION, []byte{0x00})
}

func (c *CommandCenter) respondToSetHistoryMode(enabled bool) {
	c.state.IsInHistoryUploadMode = enabled

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_REVIEW__SET_HISTORY_UPLOAD_MODE - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.encodeAndWrite(OPCODE_REVIEW__SET_HISTORY_UPLOAD_MODE, []byte{0x00})
}

func (c *CommandCenter) respondToHistoryRequest(code byte, from time.Time) {
	var filterOnDate = func(h HistoryItem) bool { return h.timestamp.After(from) }
	var filterOnCode = func(h HistoryItem) bool { return h.code == code }

	var items = filter(c.state.History, filterOnDate)
	if code != OPCODE_REVIEW__ALL_HISTORY {
		items = filter(items, filterOnCode)
	}

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Uploading history items. Count: " + fmt.Sprint(items))

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
		message[9] = byte(item.value)
		message[10] = byte(item.value << 8)

		fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending hisory item - Data: " + base64.StdEncoding.EncodeToString(message))
		c.encodeAndWrite(code, message)
	}

	// Send upload done message
	var message = make([]byte, 3)
	message[0] = 0
	message[1] = 0
	message[2] = 0

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Done uploading history - Data: " + base64.StdEncoding.EncodeToString(message))
	c.encodeAndWrite(code, message)
}

func (c *CommandCenter) respondToSetTime(request []byte) {
	var pumpTime = time.Now().Add(time.Duration(c.state.PumpTimeSkewInSeconds * int(time.Second)))
	var requestTime = getDate(request, 0, time.Local)

	var diff = pumpTime.Sub(requestTime)
	c.state.PumpTimeSkewInSeconds = int(diff.Seconds())
	c.state.Save()

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_OPTION__SET_PUMP_TIME - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.encodeAndWrite(OPCODE_OPTION__SET_PUMP_TIME, []byte{0x00})
}

func (c *CommandCenter) respondToSetTimeWithUtc(request []byte) {
	var pumpTime = time.Now().UTC().Add(time.Duration(c.state.PumpTimeSkewInSeconds * int(time.Second)))
	var requestTime = getDate(request, 0, time.UTC)

	var diff = pumpTime.Sub(requestTime)
	c.state.PumpTimeSkewInSeconds = int(diff.Seconds())
	c.state.PumpTimeZoneOffsetInSeconds = int(request[8]) * 3600
	c.state.Save()

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_OPTION__SET_PUMP_UTC_AND_TIME_ZONE - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.encodeAndWrite(OPCODE_OPTION__SET_PUMP_UTC_AND_TIME_ZONE, []byte{0x00})
}

func (c *CommandCenter) respondToBolusStart(request []byte) {
	if c.state.IsSuspended {
		fmt.Println(time.Now().Format(time.RFC3339) + " ERROR: Pump is suspended, rejecting bolus" + base64.StdEncoding.EncodeToString([]byte{0x01}))
		c.encodeAndWrite(OPCODE_BOLUS__SET_STEP_BOLUS_START, []byte{0x01})
		return
	}

	var amount = float32(request[2]) + float32(int(request[3])<<8)
	var speed = request[4]

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_BOLUS__SET_STEP_BOLUS_START - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.encodeAndWrite(OPCODE_BOLUS__SET_STEP_BOLUS_START, []byte{0x00})

	c.doBolus(amount/100, speed)
}

func (c *CommandCenter) respondToCancelBolus() {
	var message = []byte{0x00}

	if c.bolusTicker == nil {
		message = []byte{0x01}
	} else {
		c.bolusTicker.Stop()
		c.bolusTicker = nil

		c.storeBolus(c.currentAmount)
	}

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_BOLUS__SET_STEP_BOLUS_STOP - Data: " + base64.StdEncoding.EncodeToString(message))
	c.encodeAndWrite(OPCODE_BOLUS__SET_STEP_BOLUS_STOP, message)
}

func (c *CommandCenter) respondToSetBasal(message []byte) {
	var basalSchedule = make([]float32, 48)
	for i := 2; i < len(message)-1; i += 2 {
		var rate = (float32(int(message[i])<<8) + float32(message[i+1])) / 100
		basalSchedule[(i/2)-1] = rate
	}

	c.state.BasalSchedule = basalSchedule
	c.state.Save()

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_BASAL__SET_PROFILE_BASAL_RATE - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.encodeAndWrite(OPCODE_BASAL__SET_PROFILE_BASAL_RATE, []byte{0x00})
}

func (c *CommandCenter) respondToSetBasalProfile() {
	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_BASAL__SET_PROFILE_NUMBER - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.encodeAndWrite(OPCODE_BASAL__SET_PROFILE_NUMBER, []byte{0x00})
}

func (c *CommandCenter) respondToSuspend(activated bool) {
	c.state.IsSuspended = activated
	c.state.Save()

	if activated {
		fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_BASAL__SET_SUSPEND_ON - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
		c.encodeAndWrite(OPCODE_BASAL__SET_SUSPEND_ON, []byte{0x00})
	} else {
		fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_BASAL__SET_SUSPEND_OFF - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
		c.encodeAndWrite(OPCODE_BASAL__SET_SUSPEND_OFF, []byte{0x00})
	}
}

func (c *CommandCenter) respondToStopTempBasal() {
	if c.state.TempBasalActiveTill == nil {
		fmt.Println(time.Now().Format(time.RFC3339) + " ERROR: No acitve temp basal, nothing to canel - Data: " + base64.StdEncoding.EncodeToString([]byte{0x01}))
		c.encodeAndWrite(OPCODE_BASAL__CANCEL_TEMPORARY_BASAL, []byte{0x01})
		return
	}

	c.state.TempBasalPercentage = 100
	c.state.TempBasalActiveTill = nil
	c.state.Save()

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_BASAL__CANCEL_TEMPORARY_BASAL - Data: " + base64.StdEncoding.EncodeToString([]byte{0x00}))
	c.encodeAndWrite(OPCODE_BASAL__CANCEL_TEMPORARY_BASAL, []byte{0x00})
}

func (c *CommandCenter) respondToTempBasal(code byte, percentage int, duration time.Duration) {
	if percentage > 200 && duration > 15*time.Second {
		// reject any temp basal command which is bigger than 200% that isnt 15 min long
		c.encodeAndWrite(code, []byte{0x01})
		return
	}

	var activeTill = time.Now().Add(duration)
	c.state.TempBasalPercentage = percentage
	c.state.TempBasalActiveTill = &activeTill
	c.state.Save()

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Setting temp basal - percentage: " + fmt.Sprint(percentage) + "%, duration: " + fmt.Sprint(duration))
	c.encodeAndWrite(code, []byte{0x00})
}

func (c *CommandCenter) respondToBasalGetRate() {
	var message = []byte{
		// Max basal
		byte(c.state.MaxBasal * 100), byte((c.state.MaxBasal * 100) >> 8),
		// Basal step
		1,
		// Basal rate
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
		0, 0,
	}

	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Get basal rate - Data: " + base64.StdEncoding.EncodeToString(message))
	c.encodeAndWrite(OPCODE_BASAL__GET_BASAL_RATE, message)
}

func (c *CommandCenter) respondToBolusStepInformation() {
	var message = []byte{
		// Bolus type
		0, 0,
		// Initial bolus amount
		0, 0,
		// last bolus time (hh:mm)
		0, 0,
		// last bolus amount
		0, 0,
		// Max bolus
		byte(c.state.MaxBolus * 100), byte((c.state.MaxBolus * 100) >> 8),
		// Bolus step
		0,
	}
	fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Get bolus step rate - Data: " + base64.StdEncoding.EncodeToString(message))
	c.encodeAndWrite(OPCODE_BOLUS__GET_STEP_BOLUS_INFORMATION, message)
}

func (c *CommandCenter) encodeAndWrite(code byte, message []byte) {
	var data = c.encryption.Encryption(EncryptionParams{operationCode: code, data: message, isEncryptionCommand: false})
	data = c.encryption.EncryptionSecondLvl(data)
	c.write(data)
}

func (c *CommandCenter) write(data []byte) {
	var index = 0
	for index < len(data) {
		var length = int(math.Min(20, float64(len(data)-index)))
		var subData = data[index : index+length]

		var _, err = c.writeCharacteristic.Write(subData)
		if err != nil {
			fmt.Println(time.Now().Format(time.RFC3339) + " ERROR: failed to write data: " + err.Error())
			return
		}

		index += length
	}
}

func (c *CommandCenter) doBolus(amount float32, speed byte) {
	var send = func(code byte, currentAmount int) {
		var message = make([]byte, 2)

		message[0] = byte(currentAmount)
		message[1] = byte(currentAmount >> 8)

		var data = c.encryption.Encryption(EncryptionParams{operationCode: code, data: message, isNotifyCommand: true, isEncryptionCommand: false})
		data = c.encryption.EncryptionSecondLvl(data)

		fmt.Println(time.Now().Format(time.RFC3339) + " INFO: Sending OPCODE_BOLUS__SET_STEP_BOLUS_START - Data: " + base64.StdEncoding.EncodeToString(message))
		c.write(data)
	}

	var timePerTick = 500 * time.Millisecond
	c.bolusTicker = time.NewTicker(timePerTick)
	go func() {
		var fullDuration = getFullDuration(amount, speed)
		var bolusIndex float32 = 0
		var totalTicks = float32(fullDuration / timePerTick)

		for range c.bolusTicker.C {
			c.currentAmount = bolusIndex / totalTicks * amount
			if c.currentAmount >= amount {
				c.state.ReservoirLevel -= amount
				send(OPCODE_NOTIFY__DELIVERY_COMPLETE, int(amount*100))
				c.storeBolus(amount)

				c.bolusTicker.Stop()
				c.bolusTicker = nil
			}

			send(OPCODE_NOTIFY__DELIVERY_RATE_DISPLAY, int(c.currentAmount*100))
			bolusIndex += 1
		}
	}()
}

func (c *CommandCenter) storeBolus(amount float32) {
	var historyItem = HistoryItem{
		timestamp: time.Now(),
		code:      HISTORYBOLUS,
		value:     uint16(amount * 100),
		// param7 & param8 is used for duration and bolusType. UNIMPLEMENTED
		param7: 0,
		param8: 0,
	}

	c.state.History = append(c.state.History, historyItem)
	c.state.Save()
}

func (c *CommandCenter) currentBasal() float32 {
	var currentTime = time.Now()
	var pastHalfHours int = (currentTime.Hour() * 2) + int(currentTime.Minute()/30)

	return c.state.BasalSchedule[pastHalfHours]
}

func getFullDuration(amount float32, speed byte) time.Duration {
	switch speed {
	case 0: // 12sec/U
		return time.Duration(amount * 12 * float32(time.Second))
	case 1: // 30 sec/U
		return time.Duration(amount * 30 * float32(time.Second))
	case 2: // 60 sec/U
		return time.Duration(amount * 60 * float32(time.Second))
	}

	fmt.Println(time.Now().Format(time.RFC3339) + " ERROR: Received invalid speed: " + fmt.Sprint(speed))
	return 0
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
