package core

import "time"

// Message is the domain model for a chat message.
type Message struct {
	ID        int64
	Room      string
	From      string
	Text      string
	CreatedAt time.Time
}
