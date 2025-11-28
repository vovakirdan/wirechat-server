package core

// Client is a chat participant as seen by the core layer.
type Client struct {
	ID       string
	Name     string
	Commands chan Command
	Events   chan Event
	Rooms    map[string]struct{}
}

// NewClient constructs a client with initialized channels.
func NewClient(id, name string) *Client {
	if name == "" {
		name = id
	}
	return &Client{
		ID:       id,
		Name:     name,
		Commands: make(chan Command, 8),
		Events:   make(chan Event, 8),
		Rooms:    make(map[string]struct{}),
	}
}
