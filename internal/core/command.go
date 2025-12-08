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

	// Call commands
	// CommandCallInvite initiates a call (direct or room).
	CommandCallInvite
	// CommandCallAccept accepts an incoming call.
	CommandCallAccept
	// CommandCallReject rejects an incoming call.
	CommandCallReject
	// CommandCallJoin joins or rejoins an active call.
	CommandCallJoin
	// CommandCallLeave leaves an active call.
	CommandCallLeave
	// CommandCallEnd ends a call for all participants.
	CommandCallEnd
)

// Command represents an action requested by a client.
type Command struct {
	Kind    CommandKind
	Room    string
	Message Message
	Call    *CallCommand // non-nil for call commands
}

// CallCommand holds data specific to call commands.
type CallCommand struct {
	CallID   string // UUID of the call (for accept/reject/join/leave/end)
	CallType string // "direct" or "room" (for invite)
	ToUserID int64  // Target user ID (for direct invite)
	RoomID   int64  // Room ID (for room invite)
	Reason   string // Reason for reject/leave
}
