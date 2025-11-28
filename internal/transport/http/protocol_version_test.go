package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestProtocolVersionMismatch(t *testing.T) {
	hub := core.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	disabledLogger := zerolog.New(nil)
	cfg := config.Default()

	server := NewServer(hub, &cfg, &disabledLogger)
	ts := httptest.NewServer(server.Handler)
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http", "ws", 1) + "/ws"
	cctx, closeCtx := context.WithTimeout(context.Background(), 3*time.Second)
	defer closeCtx()

	conn, _, err := websocket.Dial(cctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	helloPayload, _ := json.Marshal(proto.HelloData{User: "alice", Protocol: proto.ProtocolVersion + 1})
	if writeErr := wsjson.Write(cctx, conn, proto.Inbound{Type: proto.InboundTypeHello, Data: helloPayload}); writeErr != nil {
		t.Fatalf("send hello: %v", writeErr)
	}

	var outbound proto.Outbound
	if err := wsjson.Read(cctx, conn, &outbound); err != nil {
		t.Fatalf("read outbound: %v", err)
	}
	if outbound.Type != proto.OutboundTypeError || outbound.Error == nil || outbound.Error.Code != "unsupported_version" {
		t.Fatalf("expected unsupported_version error, got %+v", outbound)
	}
}
