package proto

import "encoding/json"

// Inbound is the envelope for messages coming from the client.
type Inbound struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

const (
	ProtocolVersion = 1

	InboundTypeHello = "hello"
	InboundTypeJoin  = "join"
	InboundTypeLeave = "leave"
	InboundTypeMsg   = "msg"

	OutboundTypeEvent = "event"
	OutboundTypeError = "error"
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

// Error describes a protocol-level error response.
type Error struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}
