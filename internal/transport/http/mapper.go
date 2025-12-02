package http

import (
	"encoding/json"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
)

func inboundToCommand(client *core.Client, inbound proto.Inbound) (*core.Command, *proto.Error, error) {
	switch inbound.Type {
	case proto.InboundTypeJoin:
		var join proto.JoinData
		if err := json.Unmarshal(inbound.Data, &join); err != nil {
			return nil, nil, err
		}
		if join.Room == "" {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "room is required"}, nil
		}
		return &core.Command{
			Kind: core.CommandJoinRoom,
			Room: join.Room,
		}, nil, nil
	case proto.InboundTypeLeave:
		var leave proto.JoinData
		if err := json.Unmarshal(inbound.Data, &leave); err != nil {
			return nil, nil, err
		}
		if leave.Room == "" {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "room is required"}, nil
		}
		return &core.Command{
			Kind: core.CommandLeaveRoom,
			Room: leave.Room,
		}, nil, nil
	case proto.InboundTypeMsg:
		var msg proto.MsgData
		if err := json.Unmarshal(inbound.Data, &msg); err != nil {
			return nil, nil, err
		}
		if msg.Room == "" {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "room is required"}, nil
		}
		return &core.Command{
			Kind: core.CommandSendRoomMessage,
			Room: msg.Room,
			Message: core.Message{
				// ID will be set by hub after saving to DB
				Room:      msg.Room,
				From:      client.Name,
				Text:      msg.Text,
				CreatedAt: time.Now(),
			},
		}, nil, nil
	default:
		return nil, &proto.Error{Code: "invalid_message", Msg: "unknown message type"}, nil
	}
}

func outboundFromEvent(event *core.Event) proto.Outbound {
	switch event.Kind {
	case core.EventRoomMessage:
		return proto.Outbound{
			Type:  "event",
			Event: "message",
			Data: proto.EventMessage{
				ID:   event.Message.ID,
				Room: event.Message.Room,
				User: event.Message.From,
				Text: event.Message.Text,
				TS:   event.Message.CreatedAt.Unix(),
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
	case core.EventHistory:
		// Convert core.Message slice to proto.EventMessage slice
		messages := make([]proto.EventMessage, 0, len(event.Messages))
		for _, msg := range event.Messages {
			messages = append(messages, proto.EventMessage{
				ID:   msg.ID,
				Room: msg.Room,
				User: msg.From,
				Text: msg.Text,
				TS:   msg.CreatedAt.Unix(),
			})
		}
		return proto.Outbound{
			Type:  "event",
			Event: "history",
			Data: proto.EventHistory{
				Room:     event.Room,
				Messages: messages,
			},
		}
	case core.EventError:
		if event.Error == nil {
			return proto.Outbound{Type: "error", Error: &proto.Error{Code: "unknown", Msg: "unknown error"}}
		}
		return proto.Outbound{
			Type:  "error",
			Error: &proto.Error{Code: event.Error.Code, Msg: event.Error.Message},
		}
	default:
		return proto.Outbound{Type: "event"}
	}
}
