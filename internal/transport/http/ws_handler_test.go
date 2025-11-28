package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func startTestServer(t *testing.T) (*httptest.Server, context.CancelFunc) {
	t.Helper()

	hub := core.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	server := NewServer(hub, config.Config{
		Addr:              ":0",
		ReadHeaderTimeout: time.Second,
		ShutdownTimeout:   time.Second,
	})

	ts := httptest.NewServer(server.Handler)
	t.Cleanup(ts.Close)

	return ts, cancel
}

func TestHealthEndpoint(t *testing.T) {
	ts, cancel := startTestServer(t)
	defer cancel()

	resp, err := ts.Client().Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestWebSocketHelloAndMessage(t *testing.T) {
	ts, cancel := startTestServer(t)
	defer cancel()

	wsURL := strings.Replace(ts.URL, "http", "ws", 1) + "/ws"

	ctx, closeCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCtx()

	connA, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial A: %v", err)
	}
	defer connA.Close(websocket.StatusNormalClosure, "done")

	connB, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial B: %v", err)
	}
	defer connB.Close(websocket.StatusNormalClosure, "done")

	sendHello := func(conn *websocket.Conn, user string) {
		payload, _ := json.Marshal(proto.HelloData{User: user})
		_ = wsjson.Write(ctx, conn, proto.Inbound{Type: "hello", Data: payload})
	}

	sendMsg := func(conn *websocket.Conn, room, text string) {
		payload, _ := json.Marshal(proto.MsgData{Room: room, Text: text})
		_ = wsjson.Write(ctx, conn, proto.Inbound{Type: "msg", Data: payload})
	}

	sendHello(connA, "alice")
	sendHello(connB, "bob")

	sendMsg(connA, "general", "hi there")

	var outbound struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
		Err  string          `json:"error,omitempty"`
	}

	if err := wsjson.Read(ctx, connB, &outbound); err != nil {
		t.Fatalf("read outbound: %v", err)
	}

	if outbound.Type != "event" {
		t.Fatalf("unexpected outbound type: %s", outbound.Type)
	}

	var event proto.EventMessage
	if err := json.Unmarshal(outbound.Data, &event); err != nil {
		t.Fatalf("unmarshal event data: %v", err)
	}

	if event.User != "alice" || event.Text != "hi there" || event.Room != "general" {
		t.Fatalf("unexpected event payload: %+v", event)
	}
}
