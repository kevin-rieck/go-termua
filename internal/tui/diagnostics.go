package tui

import "time"

type DiagnosticEvent struct {
	Timestamp time.Time
	Message   string
}

type DiagnosticLog struct {
	capacity int
	events   []DiagnosticEvent
	now      func() time.Time
}

func NewDiagnosticLog(capacity int) *DiagnosticLog {
	if capacity < 1 {
		capacity = 1
	}
	return &DiagnosticLog{capacity: capacity, now: time.Now}
}

func (l *DiagnosticLog) Add(message string) {
	if l == nil || message == "" {
		return
	}
	event := DiagnosticEvent{Timestamp: l.now(), Message: message}
	if len(l.events) >= l.capacity {
		copy(l.events, l.events[1:])
		l.events[len(l.events)-1] = event
		return
	}
	l.events = append(l.events, event)
}

func (l *DiagnosticLog) Events() []DiagnosticEvent {
	if l == nil {
		return nil
	}
	events := make([]DiagnosticEvent, len(l.events))
	copy(events, l.events)
	return events
}
