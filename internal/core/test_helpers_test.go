package core

import (
	"testing"
	"time"
)

func mustEvent(t *testing.T, ch <-chan *Event, kind EventKind) *Event {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case ev := <-ch:
			if ev == nil {
				continue
			}
			if ev.Kind == kind {
				return ev
			}
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	t.Fatalf("expected event kind %v not received", kind)
	return nil
}
