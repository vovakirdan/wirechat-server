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
	register    chan *Client
	unregister  chan *Client
	commands    chan clientCommand
	clients     map[*Client]struct{}
	rooms       map[string]*Room
	store       store.Store
	userClients map[int64]*Client // Maps authenticated user IDs to their connected clients
	callService CallService       // For processing call commands (nil if calls disabled)
}

type clientCommand struct {
	client *Client
	cmd    *Command
}

// NewHub creates a new chat hub instance.
// callSvc can be nil if calls are disabled.
func NewHub(st store.Store, callSvc CallService) Hub {
	return &coreHub{
		register:    make(chan *Client, 16),
		unregister:  make(chan *Client, 16),
		commands:    make(chan clientCommand, 64),
		clients:     make(map[*Client]struct{}),
		rooms:       make(map[string]*Room),
		store:       st,
		userClients: make(map[int64]*Client),
		callService: callSvc,
	}
}

// Run starts the main event loop until context cancellation.
func (h *coreHub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
			// Track authenticated users by userID for targeted events (e.g., calls)
			if client.UserID > 0 {
				h.userClients[client.UserID] = client
			}
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
	// Call commands
	case CommandCallInvite:
		h.handleCallInvite(client, cmd.Call)
	case CommandCallAccept:
		h.handleCallAccept(client, cmd.Call)
	case CommandCallReject:
		h.handleCallReject(client, cmd.Call)
	case CommandCallJoin:
		h.handleCallJoin(client, cmd.Call)
	case CommandCallLeave:
		h.handleCallLeave(client, cmd.Call)
	case CommandCallEnd:
		h.handleCallEnd(client, cmd.Call)
	default:
		// Unknown command kinds are ignored for now.
	}
}

// sendToUser sends an event to a specific user by their ID.
// Returns true if the user is connected and the event was sent.
func (h *coreHub) sendToUser(userID int64, event *Event) bool {
	client, ok := h.userClients[userID]
	if !ok {
		return false // User not connected
	}
	select {
	case client.Events <- event:
		return true
	default:
		return false // Channel full
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

	// Send message history to the joining client (optional, best-effort)
	if h.store != nil {
		ctx := context.Background()

		// Try to get room from database
		dbRoom, err := h.store.GetRoomByName(ctx, roomName)
		if err == nil {
			// Room exists in database, fetch last 20 messages
			messages, err := h.store.ListMessages(ctx, dbRoom.ID, 20, nil)
			if err == nil && len(messages) > 0 {
				// Convert store.Message to core.Message
				coreMessages := make([]Message, 0, len(messages))
				for _, msg := range messages {
					// Get username for the message
					user, err := h.store.GetUserByID(ctx, msg.UserID)
					username := "unknown"
					if err == nil {
						username = user.Username
					}

					coreMessages = append(coreMessages, Message{
						ID:        msg.ID,
						Room:      roomName,
						From:      username,
						Text:      msg.Body,
						CreatedAt: msg.CreatedAt,
					})
				}

				// Send history event to this client only
				client.Events <- &Event{
					Kind:     EventHistory,
					Room:     roomName,
					Messages: coreMessages,
				}
			}
		}
		// Ignore errors - history is optional, don't fail join if history fetch fails
	}
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
	// Remove from userClients map
	if client.UserID > 0 {
		delete(h.userClients, client.UserID)
	}
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

// --- Call command handlers ---

// sendCallError sends a call-related error to the client.
func (h *coreHub) sendCallError(client *Client, code, msg string) {
	client.Events <- &Event{
		Kind:  EventError,
		Error: coreError(code, msg),
	}
}

func (h *coreHub) handleCallInvite(client *Client, callCmd *CallCommand) {
	// Check if call service is available
	if h.callService == nil {
		h.sendCallError(client, ErrCodeCallsDisabled, "calls are not enabled")
		return
	}

	// Require authentication
	if client.IsGuest || client.UserID == 0 {
		h.sendCallError(client, ErrCodeUnauthorized, "authentication required for calls")
		return
	}

	ctx := context.Background()

	if callCmd.CallType == "direct" {
		// Create direct call
		call, err := h.callService.CreateDirectCall(ctx, client.UserID, callCmd.ToUserID)
		if err != nil {
			h.sendCallError(client, ErrCodeCallError, err.Error())
			return
		}

		// Get target user info
		toUsername, err := h.callService.GetTargetUser(ctx, callCmd.ToUserID)
		if err != nil {
			toUsername = "unknown"
		}

		// Send call.ringing to initiator
		client.Events <- &Event{
			Kind: EventCallRinging,
			Call: &CallEvent{
				CallID:     call.ID,
				ToUserID:   callCmd.ToUserID,
				ToUsername: toUsername,
			},
		}

		// Send call.incoming to target user
		h.sendToUser(callCmd.ToUserID, &Event{
			Kind: EventCallIncoming,
			Call: &CallEvent{
				CallID:       call.ID,
				CallType:     "direct",
				FromUserID:   client.UserID,
				FromUsername: client.Name,
				CreatedAt:    call.CreatedAt.Unix(),
			},
		})
	} else if callCmd.CallType == "room" {
		// Create room call
		call, err := h.callService.CreateRoomCall(ctx, client.UserID, callCmd.RoomID)
		if err != nil {
			h.sendCallError(client, ErrCodeCallError, err.Error())
			return
		}

		// Get room info
		roomName, _ := h.callService.GetRoomInfo(ctx, callCmd.RoomID)

		// Get room members
		memberIDs, err := h.callService.ListRoomMembers(ctx, callCmd.RoomID)
		if err != nil {
			h.sendCallError(client, ErrCodeCallError, err.Error())
			return
		}

		// Send call.incoming to all room members except initiator
		for _, memberID := range memberIDs {
			if memberID != client.UserID {
				h.sendToUser(memberID, &Event{
					Kind: EventCallIncoming,
					Call: &CallEvent{
						CallID:       call.ID,
						CallType:     "room",
						FromUserID:   client.UserID,
						FromUsername: client.Name,
						RoomID:       callCmd.RoomID,
						RoomName:     roomName,
						CreatedAt:    call.CreatedAt.Unix(),
					},
				})
			}
		}
	}
}

func (h *coreHub) handleCallAccept(client *Client, callCmd *CallCommand) {
	if h.callService == nil {
		h.sendCallError(client, ErrCodeCallsDisabled, "calls are not enabled")
		return
	}

	if client.IsGuest || client.UserID == 0 {
		h.sendCallError(client, ErrCodeUnauthorized, "authentication required for calls")
		return
	}

	ctx := context.Background()

	// Get join info (this also marks the call as active)
	joinInfo, err := h.callService.GetJoinInfo(ctx, callCmd.CallID, client.UserID)
	if err != nil {
		h.sendCallError(client, ErrCodeCallError, err.Error())
		return
	}

	// Get call to find initiator
	call, err := h.callService.GetCall(ctx, callCmd.CallID)
	if err != nil {
		h.sendCallError(client, ErrCodeCallNotFound, "call not found")
		return
	}

	// Send call.join-info to acceptor
	client.Events <- &Event{
		Kind: EventCallJoinInfo,
		Call: &CallEvent{
			CallID: callCmd.CallID,
			JoinInfo: &CallJoinInfo{
				URL:      joinInfo.URL,
				Token:    joinInfo.Token,
				RoomName: joinInfo.RoomName,
				Identity: joinInfo.Identity,
			},
		},
	}

	// Send call.accepted to initiator
	h.sendToUser(call.InitiatorUserID, &Event{
		Kind: EventCallAccepted,
		Call: &CallEvent{
			CallID:       callCmd.CallID,
			FromUserID:   client.UserID,
			FromUsername: client.Name,
		},
	})

	// Send call.join-info to initiator
	initiatorJoinInfo, err := h.callService.GetJoinInfo(ctx, callCmd.CallID, call.InitiatorUserID)
	if err == nil {
		h.sendToUser(call.InitiatorUserID, &Event{
			Kind: EventCallJoinInfo,
			Call: &CallEvent{
				CallID: callCmd.CallID,
				JoinInfo: &CallJoinInfo{
					URL:      initiatorJoinInfo.URL,
					Token:    initiatorJoinInfo.Token,
					RoomName: initiatorJoinInfo.RoomName,
					Identity: initiatorJoinInfo.Identity,
				},
			},
		})
	}
}

func (h *coreHub) handleCallReject(client *Client, callCmd *CallCommand) {
	if h.callService == nil {
		h.sendCallError(client, ErrCodeCallsDisabled, "calls are not enabled")
		return
	}

	if client.IsGuest || client.UserID == 0 {
		h.sendCallError(client, ErrCodeUnauthorized, "authentication required for calls")
		return
	}

	ctx := context.Background()

	// Get call to find initiator before rejecting
	call, err := h.callService.GetCall(ctx, callCmd.CallID)
	if err != nil {
		h.sendCallError(client, ErrCodeCallNotFound, "call not found")
		return
	}

	// Reject the call
	if err := h.callService.RejectCall(ctx, callCmd.CallID, client.UserID, callCmd.Reason); err != nil {
		h.sendCallError(client, ErrCodeCallError, err.Error())
		return
	}

	// Send call.rejected to initiator
	h.sendToUser(call.InitiatorUserID, &Event{
		Kind: EventCallRejected,
		Call: &CallEvent{
			CallID:     callCmd.CallID,
			FromUserID: client.UserID,
			Reason:     callCmd.Reason,
		},
	})

	// Send call.ended to both parties
	reason := "rejected"
	if callCmd.Reason != "" {
		reason = callCmd.Reason
	}

	h.sendToUser(call.InitiatorUserID, &Event{
		Kind: EventCallEnded,
		Call: &CallEvent{
			CallID:     callCmd.CallID,
			FromUserID: client.UserID,
			Reason:     reason,
		},
	})
}

func (h *coreHub) handleCallJoin(client *Client, callCmd *CallCommand) {
	if h.callService == nil {
		h.sendCallError(client, ErrCodeCallsDisabled, "calls are not enabled")
		return
	}

	if client.IsGuest || client.UserID == 0 {
		h.sendCallError(client, ErrCodeUnauthorized, "authentication required for calls")
		return
	}

	ctx := context.Background()

	// Get join info
	joinInfo, err := h.callService.GetJoinInfo(ctx, callCmd.CallID, client.UserID)
	if err != nil {
		h.sendCallError(client, ErrCodeCallError, err.Error())
		return
	}

	// Send call.join-info to the joining user
	client.Events <- &Event{
		Kind: EventCallJoinInfo,
		Call: &CallEvent{
			CallID: callCmd.CallID,
			JoinInfo: &CallJoinInfo{
				URL:      joinInfo.URL,
				Token:    joinInfo.Token,
				RoomName: joinInfo.RoomName,
				Identity: joinInfo.Identity,
			},
		},
	}

	// TODO: Send call.participant-joined to other participants
	// This requires tracking active call participants in the hub
}

func (h *coreHub) handleCallLeave(client *Client, callCmd *CallCommand) {
	if h.callService == nil {
		h.sendCallError(client, ErrCodeCallsDisabled, "calls are not enabled")
		return
	}

	if client.IsGuest || client.UserID == 0 {
		h.sendCallError(client, ErrCodeUnauthorized, "authentication required for calls")
		return
	}

	ctx := context.Background()

	if err := h.callService.LeaveCall(ctx, callCmd.CallID, client.UserID); err != nil {
		h.sendCallError(client, ErrCodeCallError, err.Error())
		return
	}

	// TODO: Send call.participant-left to other participants
	// This requires tracking active call participants in the hub
}

func (h *coreHub) handleCallEnd(client *Client, callCmd *CallCommand) {
	if h.callService == nil {
		h.sendCallError(client, ErrCodeCallsDisabled, "calls are not enabled")
		return
	}

	if client.IsGuest || client.UserID == 0 {
		h.sendCallError(client, ErrCodeUnauthorized, "authentication required for calls")
		return
	}

	ctx := context.Background()

	// Get call participants before ending
	call, err := h.callService.GetCall(ctx, callCmd.CallID)
	if err != nil {
		h.sendCallError(client, ErrCodeCallNotFound, "call not found")
		return
	}

	// End the call
	if err := h.callService.EndCall(ctx, callCmd.CallID, client.UserID); err != nil {
		h.sendCallError(client, ErrCodeCallError, err.Error())
		return
	}

	// Send call.ended to initiator (if not the one who ended)
	if call.InitiatorUserID != client.UserID {
		h.sendToUser(call.InitiatorUserID, &Event{
			Kind: EventCallEnded,
			Call: &CallEvent{
				CallID:     callCmd.CallID,
				FromUserID: client.UserID,
				Reason:     "ended",
			},
		})
	}

	// For direct calls, also send to the other participant
	// For room calls, we'd need to iterate through participants
	// This is a simplified implementation - full implementation would track all participants
}
