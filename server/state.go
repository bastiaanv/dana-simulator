package server

type SimulatorState struct {
	name     string
	pumpType int
	status   int
}

const (
	DanaI    int = 2
	DanaRSv3 int = 1
	DanaRSv1 int = 0
)

const (
	Idle    int = 0
	Running int = 1
)
