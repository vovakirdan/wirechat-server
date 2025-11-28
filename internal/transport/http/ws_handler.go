package http

import (
	"context"
	"errors"
	"io"
	"log"
	stdhttp "net/http"

	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
	"github.com/vovakirdan/wirechat-server/pkg/utils"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// WSHandler upgrades HTTP connections and bridges them to core.Client.
type WSHandler struct {
	hub core.Hub
}

// NewWSHandler builds a new WebSocket handler.
func NewWSHandler(hub core.Hub) stdhttp.Handler {
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
	<-errCh

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

		cmd, err := inboundToCommand(client, inbound)
		if err != nil {
			return err
		}
		if cmd != nil {
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
			if err := wsjson.Write(ctx, conn, outboundFromEvent(event)); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
