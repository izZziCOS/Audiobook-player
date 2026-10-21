package media

import "time"

const SkipInterval = 30 * time.Second

type Command int

const (
	Play Command = iota
	Pause
	TogglePlay
	Next
	Previous
	SkipForward
	SkipBackward
	Seek
)

type Event struct {
	Command  Command
	Position time.Duration
}

type Track struct {
	Title    string
	Artist   string
	Cover    []byte
	Duration time.Duration
}

type Status struct {
	Playing  bool
	Position time.Duration
	Rate     float64
}
