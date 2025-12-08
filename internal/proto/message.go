package proto

import "encoding/json"

// Inbound is the envelope for messages coming from the client.
type Inbound struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

const (
	ProtocolVersion = 1

	// Chat inbound types
	InboundTypeHello = "hello"
	InboundTypeJoin  = "join"
	InboundTypeLeave = "leave"
	InboundTypeMsg   = "msg"

	// Call inbound types
	InboundTypeCallInvite = "call.invite"
	InboundTypeCallAccept = "call.accept"
	InboundTypeCallReject = "call.reject"
	InboundTypeCallJoin   = "call.join"
	InboundTypeCallLeave  = "call.leave"
	InboundTypeCallEnd    = "call.end"

	OutboundTypeEvent = "event"
	OutboundTypeError = "error"

	// Call event types (used in Outbound.Event field)
	EventTypeCallIncoming          = "call.incoming"
	EventTypeCallRinging           = "call.ringing"
	EventTypeCallAccepted          = "call.accepted"
	EventTypeCallRejected          = "call.rejected"
	EventTypeCallJoinInfo          = "call.join-info"
	EventTypeCallParticipantJoined = "call.participant-joined"
	EventTypeCallParticipantLeft   = "call.participant-left"
	EventTypeCallEnded             = "call.ended"
)

// HelloData is sent by the client to introduce itself.
type HelloData struct {
	User     string `json:"user"`
	Token    string `json:"token,omitempty"`
	Protocol int    `json:"protocol,omitempty"`
}

// JoinData requests to join a specific room.
type JoinData struct {
	Room string `json:"room"`
}

// MsgData is a chat message from the client.
type MsgData struct {
	Room string `json:"room"`
	Text string `json:"text"`
}

// Outbound is the envelope for messages sent to the client.
type Outbound struct {
	Type  string `json:"type"`
	Event string `json:"event,omitempty"`
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

// EventMessage is emitted to all clients for now (rooms later).
type EventMessage struct {
	ID   int64  `json:"id,omitempty"`
	Room string `json:"room,omitempty"`
	User string `json:"user"`
	Text string `json:"text"`
	TS   int64  `json:"ts"`
}

// EventUserJoined notifies that a user joined a room.
type EventUserJoined struct {
	Room string `json:"room"`
	User string `json:"user"`
}

// EventUserLeft notifies that a user left a room.
type EventUserLeft struct {
	Room string `json:"room"`
	User string `json:"user"`
}

// EventHistory delivers message history upon joining a room.
type EventHistory struct {
	Room     string         `json:"room"`
	Messages []EventMessage `json:"messages"`
}

// Error describes a protocol-level error response.
type Error struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// --- Call Inbound Data Types ---

// CallInviteData is sent to initiate a call.
type CallInviteData struct {
	CallType string `json:"call_type"`           // "direct" or "room"
	ToUserID int64  `json:"to_user_id,omitempty"` // For direct calls
	RoomID   int64  `json:"room_id,omitempty"`    // For room calls
}

// CallActionData is used for accept, reject, join, leave, end commands.
type CallActionData struct {
	CallID string `json:"call_id"`
	Reason string `json:"reason,omitempty"` // For reject/leave
}

// --- Call Outbound Event Types ---

// EventCallIncoming notifies target user(s) of an incoming call.
type EventCallIncoming struct {
	CallID       string `json:"call_id"`
	CallType     string `json:"call_type"`
	FromUserID   int64  `json:"from_user_id"`
	FromUsername string `json:"from_username"`
	RoomID       int64  `json:"room_id,omitempty"`
	RoomName     string `json:"room_name,omitempty"`
	CreatedAt    int64  `json:"created_at"`
}

// EventCallRinging confirms to initiator that call is ringing.
type EventCallRinging struct {
	CallID     string `json:"call_id"`
	ToUserID   int64  `json:"to_user_id"`
	ToUsername string `json:"to_username"`
}

// EventCallAccepted notifies initiator that call was accepted.
type EventCallAccepted struct {
	CallID             string `json:"call_id"`
	AcceptedByUserID   int64  `json:"accepted_by_user_id"`
	AcceptedByUsername string `json:"accepted_by_username"`
}

// EventCallRejected notifies initiator that call was rejected.
type EventCallRejected struct {
	CallID           string `json:"call_id"`
	RejectedByUserID int64  `json:"rejected_by_user_id"`
	Reason           string `json:"reason,omitempty"`
}

// EventCallJoinInfo delivers LiveKit credentials.
type EventCallJoinInfo struct {
	CallID   string `json:"call_id"`
	URL      string `json:"url"`
	Token    string `json:"token"`
	RoomName string `json:"room_name"`
	Identity string `json:"identity"`
}

// EventCallParticipantJoined notifies that someone joined the call.
type EventCallParticipantJoined struct {
	CallID   string `json:"call_id"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
}

// EventCallParticipantLeft notifies that someone left the call.
type EventCallParticipantLeft struct {
	CallID   string `json:"call_id"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Reason   string `json:"reason,omitempty"`
}

// EventCallEnded notifies all participants that the call ended.
type EventCallEnded struct {
	CallID        string `json:"call_id"`
	EndedByUserID int64  `json:"ended_by_user_id"`
	Reason        string `json:"reason"`
}
