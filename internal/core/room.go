package core

// Room groups clients subscribed to the same channel.
type Room struct {
	Name    string
	clients map[*Client]struct{}
}

// NewRoom constructs a room with no clients.
func NewRoom(name string) *Room {
	return &Room{
		Name:    name,
		clients: make(map[*Client]struct{}),
	}
}

// AddClient inserts a client into the room. Returns true if newly added.
func (r *Room) AddClient(c *Client) bool {
	if _, exists := r.clients[c]; exists {
		return false
	}
	r.clients[c] = struct{}{}
	return true
}

// RemoveClient deletes a client from the room. Returns true if removed.
func (r *Room) RemoveClient(c *Client) bool {
	if _, exists := r.clients[c]; !exists {
		return false
	}
	delete(r.clients, c)
	return true
}

// Broadcast sends an event to all clients in the room.
func (r *Room) Broadcast(event *Event) {
	for client := range r.clients {
		select {
		case client.Events <- event:
		default:
			// Drop if slow consumer.
		}
	}
}

// Empty returns true if no clients are in the room.
func (r *Room) Empty() bool {
	return len(r.clients) == 0
}
