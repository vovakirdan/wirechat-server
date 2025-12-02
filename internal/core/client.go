package core

// Client is a chat participant as seen by the core layer.
type Client struct {
	ID       string
	UserID   int64 // Database user ID (0 for unauthenticated)
	Name     string
	IsGuest  bool
	Commands chan *Command
	Events   chan *Event
	Rooms    map[string]struct{}
}

// NewClient constructs a client with initialized channels.
func NewClient(id, name string, userID int64, isGuest bool) *Client {
	if name == "" {
		name = id
	}
	return &Client{
		ID:       id,
		UserID:   userID,
		Name:     name,
		IsGuest:  isGuest,
		Commands: make(chan *Command, 8),
		Events:   make(chan *Event, 8),
		Rooms:    make(map[string]struct{}),
	}
}
