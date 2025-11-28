package core

// CommandKind describes what the client wants to do.
type CommandKind int

const (
	// CommandSendRoomMessage delivers a chat message to room participants.
	CommandSendRoomMessage CommandKind = iota
	// CommandJoinRoom subscribes the client to a room.
	CommandJoinRoom
	// CommandLeaveRoom unsubscribes the client from a room.
	CommandLeaveRoom
)

// Command represents an action requested by a client.
type Command struct {
	Kind    CommandKind
	Room    string
	Message Message
}
