package analyze

import "fmt"

type ProgressEvent struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
	Current int    `json:"current,omitempty"`
	Total   int    `json:"total,omitempty"`
}

type ProgressEmitter func(event ProgressEvent)

func noopEmitter(_ ProgressEvent) {}

func (e ProgressEmitter) phase(phase, msg string) {
	e(ProgressEvent{Phase: phase, Message: msg})
}

func (e ProgressEmitter) progress(phase string, current, total int, msg string) {
	e(ProgressEvent{Phase: phase, Message: msg, Current: current, Total: total})
}

func (e ProgressEmitter) step(phase, template string, args ...any) {
	e(ProgressEvent{Phase: phase, Message: fmt.Sprintf(template, args...)})
}
