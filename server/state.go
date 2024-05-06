package server

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
)

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

	HISTORYBOLUS       = 0x02
	HISTORY_DAILY      = 0x03
	HISTORY_PRIME      = 0x04
	HISTORY_REFILL     = 0x05
	HISTORY_GLUCOSE    = 0x06
	HISTORY_CARBO      = 0x07
	HISTORY_SUSPEND    = 0x09
	HISTORY_ALARM      = 0x0a
	HISTORY_BASALHOUR  = 0x0b
	HISTORY_TEMP_BASAL = 0x99
)

type SimulatorState struct {
	// Base information
	Name     string
	PumpType int
	Status   int

	// Pump time
	PumpTimeSkewInSeconds       int
	PumpTimeZoneOffsetInSeconds int

	// Technical settings
	ReservoirLevel   float32
	BatteryRemaining int
	IsSuspended      bool

	// Basal
	BasalSchedule []float32

	// temp basal
	TempBasalActiveTill *time.Time
	TempBasalPercentage int

	// History
	IsInHistoryUploadMode bool
	History               []HistoryItem

	// User options
	LowReservoirWarning  int
	TimeDisplayIn12H     bool
	ButtonScroll         bool
	BeepAndAlarm         int
	LcdOnInSeconds       int
	BacklightOnInSeconds int
	SelectedLanguage     int
	Units                int
	ShutdownInHours      int
	CannulaVolume        int
	RefillAmount         int
	TargetBg             int
}

func (s *SimulatorState) Save() {
	json, err := json.Marshal(s)
	if err != nil {
		fmt.Println(err)
		return
	}

	if err := os.WriteFile("state.json", []byte(json), 0666); err != nil {
		fmt.Println(err)
	}
}

type HistoryItem struct {
	timestamp time.Time
	code      byte
	param7    byte
	param8    byte
	value     uint16
}

func GetDefaultState() SimulatorState {
	if content, err := os.ReadFile("state.json"); err == nil {
		var payload SimulatorState
		err = json.Unmarshal(content, &payload)
		if err == nil {
			return payload
		}
	}

	// For every 30 min add 1U/hr as schedule
	basalSchedule := make([]float32, 48)
	for i := range basalSchedule {
		basalSchedule[i] = 1
	}

	var state = SimulatorState{
		Status:   STATUS_IDLE,
		PumpType: PUMP_TYPE_DANA_I,
		Name:     randomName(),

		PumpTimeSkewInSeconds:       0,
		PumpTimeZoneOffsetInSeconds: timeZoneOffset,

		ReservoirLevel:      300,
		BatteryRemaining:    100, // Only 100, 75, 50 & 25 are valid values
		IsSuspended:         false,
		BasalSchedule:       basalSchedule,
		TempBasalActiveTill: nil,
		TempBasalPercentage: 100,

		TimeDisplayIn12H:     false,
		ButtonScroll:         true,
		BeepAndAlarm:         ALARM_TYPE_SOUND, // Assuming the pump is in Silent tone mode
		LcdOnInSeconds:       15,
		BacklightOnInSeconds: 15,
		SelectedLanguage:     1,
		Units:                UNITS_MMOL,
		LowReservoirWarning:  20,
		CannulaVolume:        1,
		ShutdownInHours:      0,
		RefillAmount:         300,
		TargetBg:             5,
	}
	state.Save()

	return state
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

func randomInt(min, max int) uint8 {
	return uint8(min + rand.Intn(max-min))
}
