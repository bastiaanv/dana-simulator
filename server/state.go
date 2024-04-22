package server

import "time"

const (
	PUMP_TYPE_DANA_I     int = 2
	PUMP_TYPE_DANA_RS_V3 int = 1
	PUMP_TYPE_DANA_RS_V1 int = 0

	STATUS_IDLE    int = 0
	STATUS_RUNNING int = 1

	ALARM_TYPE_SOUND     = 1
	ALARM_TYPE_VIBRATION = 2
	ALARM_TYPE_BOTH      = 3

	UNITS_MG   = 0
	UNITS_MMOL = 1
)

type SimulatorState struct {
	// Base information
	name     string
	pumpType int
	status   int

	// Pump time
	pumpTimeSkewInSeconds       int
	pumpTimeZoneOffsetInSeconds int

	// Technical settings
	reservoirLevel   float32
	batteryRemaining int
	isSuspended      bool

	// temp basal
	tempBasalActiveTill *time.Time
	tempBasalPercentage int

	// User options
	lowReservoirWarning  int
	timeDisplayIn24H     bool
	buttonScroll         bool
	beepAndAlarm         int
	lcdOnInSeconds       int
	backlightOnInSeconds int
	selectedLanguage     int
	units                int
	shutdownInHours      int
	cannulaVolume        int
	refillAmount         int
	targetBg             int
}

func GetDefaultState() SimulatorState {
	return SimulatorState{
		status:   STATUS_IDLE,
		pumpType: PUMP_TYPE_DANA_I,
		name:     randomName(),

		pumpTimeSkewInSeconds:       0,
		pumpTimeZoneOffsetInSeconds: timeZoneOffset,

		reservoirLevel:      300,
		batteryRemaining:    100, // Only 100, 75, 50, 25 & 0 are valid values
		isSuspended:         false,
		tempBasalActiveTill: nil,
		tempBasalPercentage: 100,

		timeDisplayIn24H:     true,
		buttonScroll:         true,
		beepAndAlarm:         ALARM_TYPE_SOUND, // Assuming the pump is in Silent tone mode
		lcdOnInSeconds:       15,
		backlightOnInSeconds: 15,
		selectedLanguage:     1,
		units:                UNITS_MMOL,
		lowReservoirWarning:  20,
		cannulaVolume:        1,
		shutdownInHours:      0,
		refillAmount:         300,
		targetBg:             5,
	}
}
