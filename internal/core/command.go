package core

// CommandKind describes what the client wants to do.
type CommandKind int

const (
	// CommandSendMessage delivers a chat message to other participants.
	CommandSendMessage CommandKind = iota
)

// Command represents an action requested by a client.
type Command struct {
	Kind    CommandKind
	Message Message
}
