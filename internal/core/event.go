package core

// EventKind is a notification the core emits to clients.
type EventKind int

const (
	// EventMessage notifies clients about a chat message.
	EventMessage EventKind = iota
)

// Event is sent to clients to describe what happened in the system.
type Event struct {
	Kind    EventKind
	Message Message
}
