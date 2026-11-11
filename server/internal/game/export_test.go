package game

// JSONNear is jsonNear (combat_test.go) for the external test package.
var JSONNear = jsonNear

// NobleClaimer is nobleClaimer for the external test package.
var NobleClaimer = nobleClaimer

// RegisterTestHandler registers a handler for a test-only event kind and returns a func that removes it.
func RegisterTestHandler(kind string, h EventHandler) (restore func()) {
	prev, had := handlers[kind]
	registerHandler(kind, h)
	return func() {
		if had {
			handlers[kind] = prev
		} else {
			delete(handlers, kind)
		}
	}
}
