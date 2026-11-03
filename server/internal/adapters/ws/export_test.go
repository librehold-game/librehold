package ws

// StallAfterHello holds every new connection right after its hello until release is closed; restore undoes it.
func StallAfterHello(release <-chan struct{}) (restore func()) {
	prev := helloSent
	helloSent = func() { <-release }
	return func() { helloSent = prev }
}
