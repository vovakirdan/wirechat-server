package core

import (
	"context"
	"time"
)

// Hub coordinates connected clients and message broadcasts.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	commands   chan clientCommand
	clients    map[*Client]struct{}
	rooms      map[string]*Room
}

type clientCommand struct {
	client *Client
	cmd    Command
}

// NewHub creates a new chat hub instance.
func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		commands:   make(chan clientCommand, 64),
		clients:    make(map[*Client]struct{}),
		rooms:      make(map[string]*Room),
	}
}

// Run starts the main event loop until context cancellation.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
			go h.consumeCommands(ctx, client)
		case client := <-h.unregister:
			h.removeClient(client)
		case cmd := <-h.commands:
			h.handleCommand(cmd.client, cmd.cmd)
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

func (h *Hub) consumeCommands(ctx context.Context, client *Client) {
	for {
		select {
		case cmd, ok := <-client.Commands:
			if !ok {
				h.UnregisterClient(client)
				return
			}
			h.commands <- clientCommand{
				client: client,
				cmd:    cmd,
			}
		case <-ctx.Done():
			return
		}
	}
}

func (h *Hub) handleCommand(client *Client, cmd Command) {
	switch cmd.Kind {
	case CommandJoinRoom:
		h.joinRoom(client, cmd.Room)
	case CommandLeaveRoom:
		h.leaveRoom(client, cmd.Room)
	case CommandSendRoomMessage:
		h.sendRoomMessage(client, cmd)
	default:
		// Unknown command kinds are ignored for now.
	}
}

func (h *Hub) joinRoom(client *Client, roomName string) {
	if roomName == "" {
		return
	}
	room := h.ensureRoom(roomName)
	if !room.AddClient(client) {
		return
	}
	client.Rooms[roomName] = struct{}{}
	h.broadcastToRoom(roomName, Event{
		Kind: EventUserJoined,
		Room: roomName,
		User: client.Name,
	})
}

func (h *Hub) leaveRoom(client *Client, roomName string) {
	room, ok := h.rooms[roomName]
	if !ok {
		return
	}
	if !room.RemoveClient(client) {
		return
	}
	delete(client.Rooms, roomName)
	h.broadcastToRoom(roomName, Event{
		Kind: EventUserLeft,
		Room: roomName,
		User: client.Name,
	})
	if room.Empty() {
		delete(h.rooms, roomName)
	}
}

func (h *Hub) sendRoomMessage(client *Client, cmd Command) {
	if cmd.Room == "" {
		return
	}
	if _, ok := client.Rooms[cmd.Room]; !ok {
		return
	}

	msg := cmd.Message
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if msg.From == "" {
		msg.From = client.Name
	}
	msg.Room = cmd.Room

	h.broadcastToRoom(cmd.Room, Event{
		Kind:    EventRoomMessage,
		Room:    cmd.Room,
		Message: msg,
	})
}

func (h *Hub) removeClient(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	for roomName := range client.Rooms {
		h.leaveRoom(client, roomName)
	}
	delete(h.clients, client)
	close(client.Events)
}

func (h *Hub) shutdown() {
	for client := range h.clients {
		close(client.Events)
	}
}

func (h *Hub) ensureRoom(name string) *Room {
	room, ok := h.rooms[name]
	if ok {
		return room
	}
	room = NewRoom(name)
	h.rooms[name] = room
	return room
}

func (h *Hub) broadcastToRoom(roomName string, event Event) {
	room, ok := h.rooms[roomName]
	if !ok {
		return
	}
	room.Broadcast(event)
}
