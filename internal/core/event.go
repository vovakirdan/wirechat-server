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

	// Call events
	// EventCallIncoming notifies target user(s) of an incoming call.
	EventCallIncoming
	// EventCallRinging confirms to initiator that the call is ringing.
	EventCallRinging
	// EventCallAccepted notifies initiator that call was accepted.
	EventCallAccepted
	// EventCallRejected notifies initiator that call was rejected.
	EventCallRejected
	// EventCallJoinInfo delivers LiveKit credentials to join the call.
	EventCallJoinInfo
	// EventCallParticipantJoined notifies participants that someone joined.
	EventCallParticipantJoined
	// EventCallParticipantLeft notifies participants that someone left.
	EventCallParticipantLeft
	// EventCallEnded notifies all participants that the call has ended.
	EventCallEnded
)

// Event is sent to clients to describe what happened in the system.
type Event struct {
	Kind     EventKind
	Room     string
	User     string
	Message  Message
	Messages []Message // For EventHistory
	Error    *CoreError
	Call     *CallEvent // non-nil for call events
}

// CallEvent holds data specific to call events.
type CallEvent struct {
	CallID       string
	CallType     string // "direct" or "room"
	FromUserID   int64
	FromUsername string
	ToUserID     int64
	ToUsername   string
	RoomID       int64
	RoomName     string
	Reason       string        // For rejected/ended events
	JoinInfo     *CallJoinInfo // For EventCallJoinInfo
	CreatedAt    int64         // Unix timestamp
}

// CallJoinInfo contains LiveKit connection credentials.
type CallJoinInfo struct {
	URL      string // LiveKit WebSocket URL
	Token    string // LiveKit JWT token
	RoomName string // LiveKit room name
	Identity string // User's identity in the room
}
