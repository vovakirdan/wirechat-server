package proto

import "encoding/json"

// Inbound is the envelope for messages coming from the client.
type Inbound struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// HelloData is sent by the client to introduce itself.
type HelloData struct {
	User string `json:"user"`
}

// MsgData is a chat message from the client.
type MsgData struct {
	Room string `json:"room"`
	Text string `json:"text"`
}

// Outbound is the envelope for messages sent to the client.
type Outbound struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
	Err  string `json:"error,omitempty"`
}

// EventMessage is emitted to all clients for now (rooms later).
type EventMessage struct {
	Room string `json:"room,omitempty"`
	User string `json:"user"`
	Text string `json:"text"`
	Ts   int64  `json:"ts"`
}
