package core

import (
	"context"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/store"
)

// Hub coordinates connected clients and message broadcasts.
type Hub interface {
	RegisterClient(*Client)
	UnregisterClient(*Client)
	Run(ctx context.Context)
}

type coreHub struct {
	register   chan *Client
	unregister chan *Client
	commands   chan clientCommand
	clients    map[*Client]struct{}
	rooms      map[string]*Room
	store      store.Store
}

type clientCommand struct {
	client *Client
	cmd    *Command
}

// NewHub creates a new chat hub instance.
func NewHub(st store.Store) Hub {
	return &coreHub{
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		commands:   make(chan clientCommand, 64),
		clients:    make(map[*Client]struct{}),
		rooms:      make(map[string]*Room),
		store:      st,
	}
}

// Run starts the main event loop until context cancellation.
func (h *coreHub) Run(ctx context.Context) {
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
// Non-blocking: если канал заполнен или hub остановлен, регистрация пропускается.
func (h *coreHub) RegisterClient(client *Client) {
	select {
	case h.register <- client:
	default:
		// Канал заполнен или hub остановлен - пропускаем регистрацию.
	}
}

// UnregisterClient schedules a client for removal.
// Non-blocking: если канал заполнен или hub остановлен, отправка пропускается.
// Это предотвращает блокировку во время graceful shutdown.
func (h *coreHub) UnregisterClient(client *Client) {
	select {
	case h.unregister <- client:
	default:
		// Канал заполнен или hub остановлен - пропускаем отправку.
		// Во время shutdown hub сам закроет все клиенты через shutdown().
	}
}

func (h *coreHub) consumeCommands(ctx context.Context, client *Client) {
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

func (h *coreHub) handleCommand(client *Client, cmd *Command) {
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

func (h *coreHub) joinRoom(client *Client, roomName string) {
	if roomName == "" {
		client.Events <- &Event{
			Kind:  EventError,
			Room:  roomName,
			Error: coreError(ErrCodeBadRequest, ErrBadRequest.Error()),
		}
		return
	}
	room := h.ensureRoom(roomName)
	if !room.AddClient(client) {
		client.Events <- &Event{
			Kind:  EventError,
			Room:  roomName,
			Error: coreError(ErrCodeAlreadyJoined, ErrAlreadyJoined.Error()),
		}
		return
	}
	client.Rooms[roomName] = struct{}{}
	h.broadcastToRoom(roomName, &Event{
		Kind: EventUserJoined,
		Room: roomName,
		User: client.Name,
	})
}

func (h *coreHub) leaveRoom(client *Client, roomName string) {
	room, ok := h.rooms[roomName]
	if !ok {
		client.Events <- &Event{
			Kind:  EventError,
			Room:  roomName,
			Error: coreError(ErrCodeRoomNotFound, ErrRoomNotFound.Error()),
		}
		return
	}
	if !room.RemoveClient(client) {
		client.Events <- &Event{
			Kind:  EventError,
			Room:  roomName,
			Error: coreError(ErrCodeNotInRoom, ErrNotInRoom.Error()),
		}
		return
	}
	delete(client.Rooms, roomName)
	h.broadcastToRoom(roomName, &Event{
		Kind: EventUserLeft,
		Room: roomName,
		User: client.Name,
	})
	if room.Empty() {
		delete(h.rooms, roomName)
	}
}

func (h *coreHub) sendRoomMessage(client *Client, cmd *Command) {
	if cmd.Room == "" {
		client.Events <- &Event{
			Kind:  EventError,
			Room:  cmd.Room,
			Error: coreError(ErrCodeBadRequest, ErrBadRequest.Error()),
		}
		return
	}
	if _, ok := client.Rooms[cmd.Room]; !ok {
		client.Events <- &Event{
			Kind:  EventError,
			Room:  cmd.Room,
			Error: coreError(ErrCodeNotInRoom, ErrNotInRoom.Error()),
		}
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

	// Save to database if authenticated user and store is available
	if !client.IsGuest && client.UserID > 0 && h.store != nil {
		ctx := context.Background()

		// Get room from database to obtain room ID
		room, err := h.store.GetRoomByName(ctx, cmd.Room)
		if err == nil {
			// Room exists in database, save message
			storeMsg := &store.Message{
				RoomID:    room.ID,
				UserID:    client.UserID,
				Body:      msg.Text,
				CreatedAt: msg.CreatedAt,
			}

			if err := h.store.SaveMessage(ctx, storeMsg); err == nil {
				// Message saved successfully, use real ID from database
				msg.ID = storeMsg.ID
			}
			// If save fails, continue with ID=0 (message will still be broadcast)
		}
		// If room not found in database, continue without saving (in-memory only room)
	}

	h.broadcastToRoom(cmd.Room, &Event{
		Kind:    EventRoomMessage,
		Room:    cmd.Room,
		Message: msg,
	})
}

func (h *coreHub) removeClient(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	for roomName := range client.Rooms {
		h.leaveRoom(client, roomName)
	}
	delete(h.clients, client)
	close(client.Events)
}

func (h *coreHub) shutdown() {
	for client := range h.clients {
		close(client.Events)
	}
}

func (h *coreHub) ensureRoom(name string) *Room {
	room, ok := h.rooms[name]
	if ok {
		return room
	}
	room = NewRoom(name)
	h.rooms[name] = room
	return room
}

func (h *coreHub) broadcastToRoom(roomName string, event *Event) {
	room, ok := h.rooms[roomName]
	if !ok {
		return
	}
	room.Broadcast(event)
}
