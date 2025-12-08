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

	// --- Call commands ---
	case proto.InboundTypeCallInvite:
		var invite proto.CallInviteData
		if err := json.Unmarshal(inbound.Data, &invite); err != nil {
			return nil, nil, err
		}
		if invite.CallType != "direct" && invite.CallType != "room" {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "call_type must be 'direct' or 'room'"}, nil
		}
		if invite.CallType == "direct" && invite.ToUserID == 0 {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "to_user_id is required for direct calls"}, nil
		}
		if invite.CallType == "room" && invite.RoomID == 0 {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "room_id is required for room calls"}, nil
		}
		return &core.Command{
			Kind: core.CommandCallInvite,
			Call: &core.CallCommand{
				CallType: invite.CallType,
				ToUserID: invite.ToUserID,
				RoomID:   invite.RoomID,
			},
		}, nil, nil

	case proto.InboundTypeCallAccept:
		var action proto.CallActionData
		if err := json.Unmarshal(inbound.Data, &action); err != nil {
			return nil, nil, err
		}
		if action.CallID == "" {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "call_id is required"}, nil
		}
		return &core.Command{
			Kind: core.CommandCallAccept,
			Call: &core.CallCommand{CallID: action.CallID},
		}, nil, nil

	case proto.InboundTypeCallReject:
		var action proto.CallActionData
		if err := json.Unmarshal(inbound.Data, &action); err != nil {
			return nil, nil, err
		}
		if action.CallID == "" {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "call_id is required"}, nil
		}
		return &core.Command{
			Kind: core.CommandCallReject,
			Call: &core.CallCommand{CallID: action.CallID, Reason: action.Reason},
		}, nil, nil

	case proto.InboundTypeCallJoin:
		var action proto.CallActionData
		if err := json.Unmarshal(inbound.Data, &action); err != nil {
			return nil, nil, err
		}
		if action.CallID == "" {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "call_id is required"}, nil
		}
		return &core.Command{
			Kind: core.CommandCallJoin,
			Call: &core.CallCommand{CallID: action.CallID},
		}, nil, nil

	case proto.InboundTypeCallLeave:
		var action proto.CallActionData
		if err := json.Unmarshal(inbound.Data, &action); err != nil {
			return nil, nil, err
		}
		if action.CallID == "" {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "call_id is required"}, nil
		}
		return &core.Command{
			Kind: core.CommandCallLeave,
			Call: &core.CallCommand{CallID: action.CallID},
		}, nil, nil

	case proto.InboundTypeCallEnd:
		var action proto.CallActionData
		if err := json.Unmarshal(inbound.Data, &action); err != nil {
			return nil, nil, err
		}
		if action.CallID == "" {
			return nil, &proto.Error{Code: core.ErrCodeBadRequest, Msg: "call_id is required"}, nil
		}
		return &core.Command{
			Kind: core.CommandCallEnd,
			Call: &core.CallCommand{CallID: action.CallID},
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

	// --- Call events ---
	case core.EventCallIncoming:
		return proto.Outbound{
			Type:  proto.OutboundTypeEvent,
			Event: proto.EventTypeCallIncoming,
			Data: proto.EventCallIncoming{
				CallID:       event.Call.CallID,
				CallType:     event.Call.CallType,
				FromUserID:   event.Call.FromUserID,
				FromUsername: event.Call.FromUsername,
				RoomID:       event.Call.RoomID,
				RoomName:     event.Call.RoomName,
				CreatedAt:    event.Call.CreatedAt,
			},
		}
	case core.EventCallRinging:
		return proto.Outbound{
			Type:  proto.OutboundTypeEvent,
			Event: proto.EventTypeCallRinging,
			Data: proto.EventCallRinging{
				CallID:     event.Call.CallID,
				ToUserID:   event.Call.ToUserID,
				ToUsername: event.Call.ToUsername,
			},
		}
	case core.EventCallAccepted:
		return proto.Outbound{
			Type:  proto.OutboundTypeEvent,
			Event: proto.EventTypeCallAccepted,
			Data: proto.EventCallAccepted{
				CallID:             event.Call.CallID,
				AcceptedByUserID:   event.Call.FromUserID,
				AcceptedByUsername: event.Call.FromUsername,
			},
		}
	case core.EventCallRejected:
		return proto.Outbound{
			Type:  proto.OutboundTypeEvent,
			Event: proto.EventTypeCallRejected,
			Data: proto.EventCallRejected{
				CallID:           event.Call.CallID,
				RejectedByUserID: event.Call.FromUserID,
				Reason:           event.Call.Reason,
			},
		}
	case core.EventCallJoinInfo:
		return proto.Outbound{
			Type:  proto.OutboundTypeEvent,
			Event: proto.EventTypeCallJoinInfo,
			Data: proto.EventCallJoinInfo{
				CallID:   event.Call.CallID,
				URL:      event.Call.JoinInfo.URL,
				Token:    event.Call.JoinInfo.Token,
				RoomName: event.Call.JoinInfo.RoomName,
				Identity: event.Call.JoinInfo.Identity,
			},
		}
	case core.EventCallParticipantJoined:
		return proto.Outbound{
			Type:  proto.OutboundTypeEvent,
			Event: proto.EventTypeCallParticipantJoined,
			Data: proto.EventCallParticipantJoined{
				CallID:   event.Call.CallID,
				UserID:   event.Call.FromUserID,
				Username: event.Call.FromUsername,
			},
		}
	case core.EventCallParticipantLeft:
		return proto.Outbound{
			Type:  proto.OutboundTypeEvent,
			Event: proto.EventTypeCallParticipantLeft,
			Data: proto.EventCallParticipantLeft{
				CallID:   event.Call.CallID,
				UserID:   event.Call.FromUserID,
				Username: event.Call.FromUsername,
				Reason:   event.Call.Reason,
			},
		}
	case core.EventCallEnded:
		return proto.Outbound{
			Type:  proto.OutboundTypeEvent,
			Event: proto.EventTypeCallEnded,
			Data: proto.EventCallEnded{
				CallID:        event.Call.CallID,
				EndedByUserID: event.Call.FromUserID,
				Reason:        event.Call.Reason,
			},
		}

	default:
		return proto.Outbound{Type: "event"}
	}
}
