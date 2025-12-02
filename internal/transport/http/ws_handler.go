package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/rs/zerolog"
	"github.com/vovakirdan/wirechat-server/internal/auth"
	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
	"github.com/vovakirdan/wirechat-server/internal/store"
	"github.com/vovakirdan/wirechat-server/internal/utils"
)

// WSHandler upgrades HTTP connections and bridges them to core.Client.
type WSHandler struct {
	hub         core.Hub
	authService *auth.Service
	store       store.Store
	log         *zerolog.Logger
	config      *config.Config
}

// NewWSHandler builds a new WebSocket handler.
func NewWSHandler(hub core.Hub, authService *auth.Service, st store.Store, cfg *config.Config, logger *zerolog.Logger) stdhttp.Handler {
	return &WSHandler{
		hub:         hub,
		authService: authService,
		store:       st,
		log:         logger,
		config:      cfg,
	}
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

	// Create client without user info - will be set in handleHello
	client := core.NewClient(utils.NewID(), "", 0, false)
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

	authenticated := !h.config.JWTRequired

	for {
		var inbound proto.Inbound
		// Use context deadline for idle timeout if configured
		// Note: This doesn't account for ping/pong activity, only JSON messages
		// But since we ping every 30s and timeout is 90s, there's buffer time
		if h.config.ClientIdleTimeout > 0 {
			readCtx, cancelRead := context.WithTimeout(ctx, h.config.ClientIdleTimeout)
			err := wsjson.Read(readCtx, conn, &inbound)
			cancelRead()
			if err != nil {
				if isExpectedClose(err) {
					return nil
				}
				h.log.Warn().Err(err).Str("client_id", client.ID).Msg("read ws inbound")
				return err
			}
		} else {
			if err := wsjson.Read(ctx, conn, &inbound); err != nil {
				if isExpectedClose(err) {
					return nil
				}
				h.log.Warn().Err(err).Str("client_id", client.ID).Msg("read ws inbound")
				return err
			}
		}

		cmd, protoErr, err := inboundToCommand(client, inbound)
		if inbound.Type == proto.InboundTypeHello && err == nil {
			protoErr, err = h.handleHello(client, inbound)
			if err == nil && protoErr == nil {
				authenticated = true
			}
		}

		if err != nil {
			h.log.Warn().Err(err).Str("client_id", client.ID).Msg("failed to map inbound")
			return err
		}

		if !authenticated && inbound.Type != proto.InboundTypeHello && h.config.JWTRequired {
			protoErr = &proto.Error{Code: "unauthorized", Msg: "hello with valid token required"}
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
			// For join commands, check room type and membership (Iteration 3.2: public + private rooms)
			if cmd.Kind == core.CommandJoinRoom {
				room, err := h.store.GetRoomByName(ctx, cmd.Room)
				if err != nil {
					h.log.Warn().Err(err).Str("room", cmd.Room).Msg("room not found")
					protoErr = &proto.Error{Code: "room_not_found", Msg: "room does not exist"}
					if writeErr := wsjson.Write(ctx, conn, proto.Outbound{
						Type:  "error",
						Error: protoErr,
					}); writeErr != nil {
						return writeErr
					}
					continue
				}

				// Check room access based on type
				switch room.Type {
				case store.RoomTypePublic:
					// Public room: anyone can join
					h.log.Debug().
						Str("client_id", client.ID).
						Str("room", cmd.Room).
						Int64("room_id", room.ID).
						Msg("allowing join to public room")
				case store.RoomTypePrivate:
					// Private room: check membership
					isMember, err := h.store.IsMember(ctx, client.UserID, room.ID)
					if err != nil {
						h.log.Error().Err(err).Int64("user_id", client.UserID).Int64("room_id", room.ID).Msg("failed to check membership")
						protoErr = &proto.Error{Code: "internal_error", Msg: "internal server error"}
						if writeErr := wsjson.Write(ctx, conn, proto.Outbound{
							Type:  "error",
							Error: protoErr,
						}); writeErr != nil {
							return writeErr
						}
						continue
					}
					if !isMember {
						h.log.Warn().
							Str("client_id", client.ID).
							Int64("user_id", client.UserID).
							Str("room", cmd.Room).
							Int64("room_id", room.ID).
							Msg("access denied: not a member of private room")
						protoErr = &proto.Error{Code: "access_denied", Msg: "access denied"}
						if writeErr := wsjson.Write(ctx, conn, proto.Outbound{
							Type:  "error",
							Error: protoErr,
						}); writeErr != nil {
							return writeErr
						}
						continue
					}
					h.log.Debug().
						Str("client_id", client.ID).
						Int64("user_id", client.UserID).
						Str("room", cmd.Room).
						Msg("allowing join to private room (user is member)")
				default:
					// Other room types (e.g., direct) - deny for now
					h.log.Warn().
						Str("client_id", client.ID).
						Str("room", cmd.Room).
						Str("room_type", string(room.Type)).
						Msg("access denied: unsupported room type")
					protoErr = &proto.Error{Code: "access_denied", Msg: "access denied"}
					if writeErr := wsjson.Write(ctx, conn, proto.Outbound{
						Type:  "error",
						Error: protoErr,
					}); writeErr != nil {
						return writeErr
					}
					continue
				}
			}

			h.log.Debug().
				Str("client_id", client.ID).
				Str("kind", commandKindString(cmd.Kind)).
				Str("room", cmd.Room).
				Msg("inbound command")
			client.Commands <- cmd
		}
	}
}

func isExpectedClose(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return true
	}
	switch websocket.CloseStatus(err) {
	case websocket.StatusNormalClosure, websocket.StatusGoingAway:
		return true
	default:
		return false
	}
}

func (h *WSHandler) writeLoop(ctx context.Context, conn *websocket.Conn, client *core.Client) error {
	// Setup ping ticker if ping interval is configured
	var pingTicker *time.Ticker
	var pingCh <-chan time.Time
	if h.config.PingInterval > 0 {
		pingTicker = time.NewTicker(h.config.PingInterval)
		defer pingTicker.Stop()
		pingCh = pingTicker.C
	}

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
		case <-pingCh:
			// Send WebSocket ping to keep connection alive
			if err := conn.Ping(ctx); err != nil {
				h.log.Debug().Err(err).Str("client_id", client.ID).Msg("ping failed")
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

func (h *WSHandler) handleHello(client *core.Client, inbound proto.Inbound) (*proto.Error, error) {
	var hello proto.HelloData
	if err := json.Unmarshal(inbound.Data, &hello); err != nil {
		return nil, err
	}

	if hello.Protocol != 0 && hello.Protocol != proto.ProtocolVersion {
		return &proto.Error{Code: "unsupported_version", Msg: "unsupported protocol version"}, nil
	}

	// Try to validate JWT token
	if hello.Token != "" {
		claims, err := h.authService.ValidateToken(hello.Token)
		if err != nil {
			h.log.Warn().Err(err).Msg("invalid jwt token")
			if h.config.JWTRequired {
				return &proto.Error{Code: "unauthorized", Msg: "invalid token"}, nil
			}
			// If JWT not required, fall through to guest mode
		} else {
			// Valid token - set user info from claims
			client.UserID = claims.UserID
			client.Name = claims.Username
			client.IsGuest = claims.IsGuest
			h.log.Info().
				Str("client_id", client.ID).
				Int64("user_id", client.UserID).
				Str("username", client.Name).
				Bool("is_guest", client.IsGuest).
				Msg("authenticated via jwt")
			return nil, nil
		}
	} else if h.config.JWTRequired {
		return &proto.Error{Code: "unauthorized", Msg: "token required"}, nil
	}

	// Guest mode: use provided username or generate one
	if hello.User != "" {
		client.Name = hello.User
	} else {
		client.Name = "guest-" + client.ID[:8]
	}
	client.IsGuest = true
	h.log.Info().
		Str("client_id", client.ID).
		Str("username", client.Name).
		Msg("connected as guest")

	return nil, nil
}
