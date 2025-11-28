package http

import (
	"encoding/json"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
	"github.com/vovakirdan/wirechat-server/internal/utils"
)

func inboundToCommand(client *core.Client, inbound proto.Inbound) (*core.Command, *proto.Error, error) {
	switch inbound.Type {
	case "join":
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
	case "leave":
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
	case "msg":
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
				ID:        utils.NewID(),
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
