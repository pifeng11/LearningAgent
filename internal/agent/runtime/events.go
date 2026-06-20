package runtime

import "time"

type Event struct {
	Type      string
	Message   string
	Timestamp time.Time
}

type Result struct {
	Answer string
	Events []Event
}
