package core

// EventKind is a notification the core emits to clients.
type EventKind int

const (
	// EventRoomMessage notifies clients about a chat message in a room.
	EventRoomMessage EventKind = iota
	// EventUserJoined notifies clients about a user joining a room.
	EventUserJoined
	// EventUserLeft notifies clients about a user leaving a room.
	EventUserLeft
	// EventHistory delivers message history to a client upon joining a room.
	EventHistory
	// EventError notifies clients about a domain error.
	EventError
)

// Event is sent to clients to describe what happened in the system.
type Event struct {
	Kind     EventKind
	Room     string
	User     string
	Message  Message
	Messages []Message // For EventHistory
	Error    *CoreError
}
