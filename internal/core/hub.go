package core

import "context"

// Hub coordinates connected clients and message broadcasts.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan Event
	clients    map[*Client]struct{}
}

// NewHub creates a new chat hub instance.
func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		broadcast:  make(chan Event, 32),
		clients:    make(map[*Client]struct{}),
	}
}

// Run starts the main event loop until context cancellation.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
		case client := <-h.unregister:
			h.removeClient(client)
		case event := <-h.broadcast:
			h.broadcastToAll(event)
		case <-ctx.Done():
			h.shutdown()
			return
		}
	}
}

// RegisterClient schedules a client for registration.
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient schedules a client for removal.
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// Broadcast enqueues an event to all connected clients.
func (h *Hub) Broadcast(event Event) {
	h.broadcast <- event
}

func (h *Hub) broadcastToAll(event Event) {
	for client := range h.clients {
		select {
		case client.Events <- event:
		default:
			// Drop the event if the client is not keeping up to avoid blocking the hub.
		}
	}
}

func (h *Hub) removeClient(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	delete(h.clients, client)
	close(client.Events)
}

func (h *Hub) shutdown() {
	for client := range h.clients {
		close(client.Events)
	}
}
