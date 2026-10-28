package game

import (
	"fmt"
	"strings"
)

// EventHandler handles one event; ref is the part of the key after the first ':' (empty for global events).
type EventHandler func(w World, ref string) error

var handlers = map[string]EventHandler{}

// registerHandler is called from init() in the file that owns each event kind.
func registerHandler(kind string, h EventHandler) {
	handlers[kind] = h
}

var eventSpy func(key string)

// SpyEvents observes every dispatched key before it is handled (tests only). It returns a restore func.
func SpyEvents(fn func(key string)) (restore func()) {
	prev := eventSpy
	eventSpy = fn
	return func() { eventSpy = prev }
}

// Dispatch runs the handler of an event key `kind:ref`.
func Dispatch(w World, key string) error {
	kind, ref, _ := strings.Cut(key, ":")
	h, ok := handlers[kind]
	if !ok {
		return fmt.Errorf("advance: unknown event kind %q", kind)
	}
	if eventSpy != nil {
		eventSpy(key)
	}
	if Ended(w) {
		return nil
	}
	return h(w, ref)
}

// NextGridTime is the first grid point off + k·interval strictly after now.
func NextGridTime(now, off, interval int64) int64 {
	return off + (floorDiv(now-off, interval)+1)*interval
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}
