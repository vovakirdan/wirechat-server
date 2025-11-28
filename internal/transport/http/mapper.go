package http

import (
	"encoding/json"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
	"github.com/vovakirdan/wirechat-server/internal/utils"
)

func inboundToCommand(client *core.Client, inbound proto.Inbound) (*core.Command, error) {
	switch inbound.Type {
	case "hello":
		var hello proto.HelloData
		if err := json.Unmarshal(inbound.Data, &hello); err != nil {
			return nil, err
		}
		if hello.User != "" {
			client.Name = hello.User
		}
		return nil, nil
	case "join":
		var join proto.JoinData
		if err := json.Unmarshal(inbound.Data, &join); err != nil {
			return nil, err
		}
		return &core.Command{
			Kind: core.CommandJoinRoom,
			Room: join.Room,
		}, nil
	case "leave":
		var leave proto.JoinData
		if err := json.Unmarshal(inbound.Data, &leave); err != nil {
			return nil, err
		}
		return &core.Command{
			Kind: core.CommandLeaveRoom,
			Room: leave.Room,
		}, nil
	case "msg":
		var msg proto.MsgData
		if err := json.Unmarshal(inbound.Data, &msg); err != nil {
			return nil, err
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
		}, nil
	default:
		// Unknown message types are ignored for now.
		return nil, nil
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
	default:
		return proto.Outbound{Type: "event"}
	}
}
