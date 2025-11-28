package http

import (
	"context"
	"encoding/json"
	"io"
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

func startTestServer(t *testing.T) (*httptest.Server, context.CancelFunc) {
	t.Helper()

	hub := core.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	disabledLogger := zerolog.New(io.Discard)

	cfg := config.Config{
		Addr:              ":0",
		ReadHeaderTimeout: time.Second,
		ShutdownTimeout:   time.Second,
		MaxMessageBytes:   1 << 20,
	}

	server := NewServer(hub, cfg, &disabledLogger)

	ts := httptest.NewServer(server.Handler)
	t.Cleanup(ts.Close)

	return ts, cancel
}

func startTestServerWithConfig(t *testing.T, cfg config.Config) (*httptest.Server, context.CancelFunc) {
	t.Helper()

	hub := core.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	disabledLogger := zerolog.New(io.Discard)

	server := NewServer(hub, cfg, &disabledLogger)

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
		payload, marshalErr := json.Marshal(proto.HelloData{User: user})
		if marshalErr != nil {
			t.Fatalf("marshal hello: %v", marshalErr)
		}
		if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "hello", Data: payload}); writeErr != nil {
			t.Fatalf("send hello: %v", writeErr)
		}
	}

	sendJoin := func(conn *websocket.Conn, room string) {
		payload, marshalErr := json.Marshal(proto.JoinData{Room: room})
		if marshalErr != nil {
			t.Fatalf("marshal join: %v", marshalErr)
		}
		if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "join", Data: payload}); writeErr != nil {
			t.Fatalf("send join: %v", writeErr)
		}
	}

	sendMsg := func(conn *websocket.Conn, room, text string) {
		payload, marshalErr := json.Marshal(proto.MsgData{Room: room, Text: text})
		if marshalErr != nil {
			t.Fatalf("marshal msg: %v", marshalErr)
		}
		if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "msg", Data: payload}); writeErr != nil {
			t.Fatalf("send msg: %v", writeErr)
		}
	}

	sendHello(connA, "alice")
	sendHello(connB, "bob")
	sendJoin(connA, "general")
	sendJoin(connB, "general")

	waitFor := func(conn *websocket.Conn, expectEvent string) proto.Outbound {
		for {
			var outbound proto.Outbound
			if readErr := wsjson.Read(ctx, conn, &outbound); readErr != nil {
				t.Fatalf("read outbound: %v", readErr)
			}
			if outbound.Type != "event" || outbound.Event != expectEvent {
				continue
			}
			return outbound
		}
	}

	// Ensure B's join processed before sending messages.
	joinOutbound := waitFor(connB, "user_joined")
	joinData, joinMarshalErr := json.Marshal(joinOutbound.Data)
	if joinMarshalErr != nil {
		t.Fatalf("marshal join outbound: %v", joinMarshalErr)
	}
	var joinEvent proto.EventUserJoined
	if unmarshalErr := json.Unmarshal(joinData, &joinEvent); unmarshalErr != nil {
		t.Fatalf("unmarshal join event: %v", unmarshalErr)
	}
	if joinEvent.User != "bob" || joinEvent.Room != "general" {
		t.Fatalf("unexpected join event: %+v", joinEvent)
	}

	sendMsg(connA, "general", "hi there")

	msgOutbound := waitFor(connB, "message")
	msgData, msgMarshalErr := json.Marshal(msgOutbound.Data)
	if msgMarshalErr != nil {
		t.Fatalf("marshal message outbound: %v", msgMarshalErr)
	}
	var event proto.EventMessage
	if unmarshalErr := json.Unmarshal(msgData, &event); unmarshalErr != nil {
		t.Fatalf("unmarshal event data: %v", unmarshalErr)
	}
	if event.User != "alice" || event.Text != "hi there" || event.Room != "general" {
		t.Fatalf("unexpected event payload: %+v", event)
	}
}

func TestWebSocketMessageTooLarge(t *testing.T) {
	cfg := config.Config{
		Addr:              ":0",
		ReadHeaderTimeout: time.Second,
		ShutdownTimeout:   time.Second,
		MaxMessageBytes:   32, // very small
	}
	ts, cancel := startTestServerWithConfig(t, cfg)
	defer cancel()

	wsURL := strings.Replace(ts.URL, "http", "ws", 1) + "/ws"

	ctx, closeCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCtx()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	payload, _ := json.Marshal(proto.HelloData{User: "big"})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "hello", Data: payload}); writeErr != nil {
		t.Fatalf("send hello: %v", writeErr)
	}

	largeMsg := proto.MsgData{Room: "general", Text: strings.Repeat("a", 1024)}
	data, _ := json.Marshal(largeMsg)
	_ = wsjson.Write(ctx, conn, proto.Inbound{Type: "msg", Data: data})

	var outbound proto.Outbound
	err = wsjson.Read(ctx, conn, &outbound)
	if err == nil {
		t.Fatalf("expected error due to message too large, got %+v", outbound)
	}
	if s := websocket.CloseStatus(err); s != websocket.StatusMessageTooBig && s != websocket.StatusInternalError {
		t.Fatalf("expected StatusMessageTooBig, got %v (err=%v)", s, err)
	}
}

func TestServerShutdownClosesConnections(t *testing.T) {
	ts, cancel := startTestServer(t)
	defer cancel()

	wsURL := strings.Replace(ts.URL, "http", "ws", 1) + "/ws"

	ctx, closeCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCtx()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	payload, _ := json.Marshal(proto.HelloData{User: "alice"})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "hello", Data: payload}); writeErr != nil {
		t.Fatalf("send hello: %v", writeErr)
	}

	// trigger shutdown
	cancel()

	var outbound proto.Outbound
	err = wsjson.Read(ctx, conn, &outbound)
	if err == nil {
		t.Fatalf("expected connection close, got message %+v", outbound)
	}
	status := websocket.CloseStatus(err)
	if status == 0 {
		t.Fatalf("expected close status, got err=%v", err)
	}
}
