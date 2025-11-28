package http

import (
	"context"
	"encoding/json"
	"log"
	stdhttp "net/http"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
	"github.com/vovakirdan/wirechat-server/pkg/utils"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// WSHandler upgrades HTTP connections and bridges them to core.Client.
type WSHandler struct {
	hub *core.Hub
}

// NewWSHandler builds a new WebSocket handler.
func NewWSHandler(hub *core.Hub) stdhttp.Handler {
	return &WSHandler{hub: hub}
}

func (h *WSHandler) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx := r.Context()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("ws accept error: %v", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "internal error")

	client := core.NewClient(utils.NewID(), "")
	h.hub.RegisterClient(client)
	defer h.hub.UnregisterClient(client)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- h.readLoop(ctx, conn, client)
	}()
	go func() {
		errCh <- h.writeLoop(ctx, conn, client)
	}()

	err = <-errCh
	cancel() // stop the other goroutine
	_ = <-errCh

	status := websocket.StatusNormalClosure
	reason := "closing"
	if err != nil && err != context.Canceled {
		if s := websocket.CloseStatus(err); s != 0 {
			status = s
		} else {
			status = websocket.StatusInternalError
		}
		reason = err.Error()
		if status != websocket.StatusNormalClosure {
			log.Printf("ws connection closed with error: %v", err)
		}
	}

	conn.Close(status, reason)
}

func (h *WSHandler) readLoop(ctx context.Context, conn *websocket.Conn, client *core.Client) error {
	for {
		var inbound proto.Inbound
		if err := wsjson.Read(ctx, conn, &inbound); err != nil {
			return err
		}

		switch inbound.Type {
		case "hello":
			var hello proto.HelloData
			if err := json.Unmarshal(inbound.Data, &hello); err != nil {
				return err
			}
			if hello.User != "" {
				client.Name = hello.User
			}
		case "join":
			var join proto.JoinData
			if err := json.Unmarshal(inbound.Data, &join); err != nil {
				return err
			}
			client.Commands <- core.Command{
				Kind: core.CommandJoinRoom,
				Room: join.Room,
			}
		case "leave":
			var leave proto.JoinData
			if err := json.Unmarshal(inbound.Data, &leave); err != nil {
				return err
			}
			client.Commands <- core.Command{
				Kind: core.CommandLeaveRoom,
				Room: leave.Room,
			}
		case "msg":
			var msg proto.MsgData
			if err := json.Unmarshal(inbound.Data, &msg); err != nil {
				return err
			}
			client.Commands <- core.Command{
				Kind: core.CommandSendRoomMessage,
				Room: msg.Room,
				Message: core.Message{
					ID:        utils.NewID(),
					Room:      msg.Room,
					From:      client.Name,
					Text:      msg.Text,
					CreatedAt: time.Now(),
				},
			}
		default:
			// Unknown types are ignored for now.
		}
	}
}

func (h *WSHandler) writeLoop(ctx context.Context, conn *websocket.Conn, client *core.Client) error {
	for {
		select {
		case event, ok := <-client.Events:
			if !ok {
				return nil
			}
			if err := wsjson.Write(ctx, conn, eventToOutbound(event)); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func eventToOutbound(event core.Event) proto.Outbound {
	switch event.Kind {
	case core.EventRoomMessage:
		return proto.Outbound{
			Type:  "event",
			Event: "message",
			Data: proto.EventMessage{
				Room: event.Message.Room,
				User: event.Message.From,
				Text: event.Message.Text,
				Ts:   event.Message.CreatedAt.Unix(),
			},
		}
	case core.EventUserJoined:
		return proto.Outbound{
			Type:  "event",
			Event: "user_joined",
			Data: proto.EventUserJoined{
				Room: event.Room,
				User: event.User,
			},
		}
	case core.EventUserLeft:
		return proto.Outbound{
			Type:  "event",
			Event: "user_left",
			Data: proto.EventUserLeft{
				Room: event.Room,
				User: event.User,
			},
		}
	default:
		return proto.Outbound{Type: "event"}
	}
}
