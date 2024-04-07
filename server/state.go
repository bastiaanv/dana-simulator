package server

type SimulatorState struct {
	name     string
	pumpType int
	status   int
}

const (
	DanaI    int = 0
	DanaRSv3 int = 1
)

const (
	Idle    int = 0
	Running int = 1
)
