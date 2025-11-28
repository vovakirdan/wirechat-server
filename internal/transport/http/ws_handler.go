package http

import (
	stdhttp "net/http"

	"github.com/vovakirdan/wirechat-server/internal/core"
)

// WSHandler is a placeholder for the WebSocket upgrade flow.
type WSHandler struct {
	hub *core.Hub
}

// NewWSHandler builds a new stub WebSocket handler.
func NewWSHandler(hub *core.Hub) stdhttp.Handler {
	return &WSHandler{hub: hub}
}

func (h *WSHandler) ServeHTTP(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	// Placeholder: will be replaced with real WebSocket upgrade and client wiring.
	w.WriteHeader(stdhttp.StatusNotImplemented)
	_, _ = w.Write([]byte("websocket endpoint not implemented yet"))
}
