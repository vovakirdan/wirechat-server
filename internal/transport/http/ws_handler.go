package http

import (
	"context"
	"errors"
	"io"
	stdhttp "net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
	"github.com/vovakirdan/wirechat-server/internal/utils"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// WSHandler upgrades HTTP connections and bridges them to core.Client.
type WSHandler struct {
	hub    core.Hub
	log    *zerolog.Logger
	config config.Config
}

// NewWSHandler builds a new WebSocket handler.
func NewWSHandler(hub core.Hub, cfg config.Config, logger *zerolog.Logger) stdhttp.Handler {
	return &WSHandler{hub: hub, log: logger, config: cfg}
}

func (h *WSHandler) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	ctx := r.Context()
	remote := r.RemoteAddr

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("ws accept error")
		return
	}
	defer conn.Close(websocket.StatusInternalError, "internal error")

	if h.config.MaxMessageBytes > 0 {
		conn.SetReadLimit(h.config.MaxMessageBytes)
	}

	client := core.NewClient(utils.NewID(), "")
	h.hub.RegisterClient(client)
	defer h.hub.UnregisterClient(client)

	h.log.Info().
		Str("client_id", client.ID).
		Str("remote", remote).
		Msg("ws connected")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	stopRate := make(chan struct{})
	go func() {
		errCh <- h.readLoop(ctx, conn, client, stopRate)
	}()
	go func() {
		errCh <- h.writeLoop(ctx, conn, client)
	}()

	err = <-errCh
	cancel() // stop the other goroutine
	<-errCh
	close(stopRate)

	status := websocket.StatusNormalClosure
	reason := "closing"
	if err != nil && !errors.Is(err, context.Canceled) {
		if errors.Is(err, io.EOF) {
			err = nil
		}
		if s := websocket.CloseStatus(err); s != 0 {
			status = s
		}
		if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
			err = nil
		}
		if err != nil {
			if status == websocket.StatusNormalClosure {
				status = websocket.StatusInternalError
			}
			reason = err.Error()
			h.log.Warn().
				Err(err).
				Str("client_id", client.ID).
				Str("remote", remote).
				Int("status", int(status)).
				Msg("ws connection closed with error")
		}
	}

	conn.Close(status, reason)
	h.log.Info().
		Str("client_id", client.ID).
		Str("remote", remote).
		Int("status", int(status)).
		Str("reason", reason).
		Msg("ws disconnected")
}

func (h *WSHandler) readLoop(ctx context.Context, conn *websocket.Conn, client *core.Client, stopRate <-chan struct{}) error {
	joinLimiter := newRateLimiter(h.config.RateLimitJoinPerMin)
	msgLimiter := newRateLimiter(h.config.RateLimitMsgPerMin)
	joinLimiter.startReset(stopRate)
	msgLimiter.startReset(stopRate)

	for {
		var inbound proto.Inbound
		if h.config.ClientIdleTimeout > 0 {
			readCtx, cancelRead := context.WithDeadline(ctx, time.Now().Add(h.config.ClientIdleTimeout))
			if err := wsjson.Read(readCtx, conn, &inbound); err != nil {
				cancelRead()
				h.log.Warn().Err(err).Str("client_id", client.ID).Msg("read ws inbound")
				return err
			}
			cancelRead()
		} else {
			if err := wsjson.Read(ctx, conn, &inbound); err != nil {
				h.log.Warn().Err(err).Str("client_id", client.ID).Msg("read ws inbound")
				return err
			}
		}

		cmd, protoErr, err := inboundToCommand(client, inbound)
		if err != nil {
			h.log.Warn().Err(err).Str("client_id", client.ID).Msg("failed to map inbound")
			return err
		}
		if cmd != nil {
			switch cmd.Kind {
			case core.CommandJoinRoom:
				if !joinLimiter.allow() {
					protoErr = &proto.Error{Code: "rate_limited", Msg: "too many join requests"}
				}
			case core.CommandSendRoomMessage:
				if !msgLimiter.allow() {
					protoErr = &proto.Error{Code: "rate_limited", Msg: "too many messages"}
				}
			}
		}
		if protoErr != nil {
			h.log.Warn().
				Str("client_id", client.ID).
				Str("code", protoErr.Code).
				Str("msg", protoErr.Msg).
				Msg("protocol error")
			if writeErr := wsjson.Write(ctx, conn, proto.Outbound{
				Type:  "error",
				Error: protoErr,
			}); writeErr != nil {
				return writeErr
			}
			continue
		}
		if cmd != nil {
			h.log.Debug().
				Str("client_id", client.ID).
				Str("kind", commandKindString(cmd.Kind)).
				Str("room", cmd.Room).
				Msg("inbound command")
			client.Commands <- cmd
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
			outbound := outboundFromEvent(event)
			h.log.Debug().
				Str("client_id", client.ID).
				Str("event", outbound.Event).
				Str("type", outbound.Type).
				Str("room", event.Room).
				Msg("outbound event")
			if err := wsjson.Write(ctx, conn, outbound); err != nil {
				h.log.Error().Err(err).Str("client_id", client.ID).Msg("write ws event")
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func commandKindString(kind core.CommandKind) string {
	switch kind {
	case core.CommandJoinRoom:
		return "join"
	case core.CommandLeaveRoom:
		return "leave"
	case core.CommandSendRoomMessage:
		return "msg"
	default:
		return "unknown"
	}
}
