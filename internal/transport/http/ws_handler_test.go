package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/rs/zerolog"
	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
)

func startTestServer(t *testing.T) (*httptest.Server, context.CancelFunc) {
	t.Helper()

	// Create test store with schema
	store := createTestStore(t)
	t.Cleanup(func() { store.Close() })

	// Create auth service
	authService := createTestAuthService(t, store, "test-secret")

	hub := core.NewHub(store)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	disabledLogger := zerolog.New(io.Discard)

	cfg := config.Config{
		Addr:              ":0",
		ReadHeaderTimeout: time.Second,
		ShutdownTimeout:   time.Second,
		MaxMessageBytes:   1 << 20,
	}

	server := NewServer(hub, authService, store, &cfg, &disabledLogger)

	ts := httptest.NewServer(server.Handler)
	t.Cleanup(ts.Close)

	return ts, cancel
}

func startTestServerWithConfig(t *testing.T, cfg config.Config) (*httptest.Server, context.CancelFunc) {
	t.Helper()

	// Create test store with schema
	store := createTestStore(t)
	t.Cleanup(func() { store.Close() })

	// Create auth service
	authService := createTestAuthService(t, store, cfg.JWTSecret)

	hub := core.NewHub(store)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	disabledLogger := zerolog.New(io.Discard)

	server := NewServer(hub, authService, store, &cfg, &disabledLogger)

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
		payload, marshalErr := json.Marshal(proto.HelloData{User: user, Protocol: proto.ProtocolVersion})
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
		Addr:                ":0",
		ReadHeaderTimeout:   time.Second,
		ShutdownTimeout:     time.Second,
		MaxMessageBytes:     32, // very small
		RateLimitJoinPerMin: 100,
		RateLimitMsgPerMin:  100,
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

func TestWebSocketRateLimitMessage(t *testing.T) {
	cfg := config.Config{
		Addr:                ":0",
		ReadHeaderTimeout:   time.Second,
		ShutdownTimeout:     time.Second,
		MaxMessageBytes:     1 << 20,
		RateLimitJoinPerMin: 10,
		RateLimitMsgPerMin:  1, // allow only one message
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

	payload, _ := json.Marshal(proto.HelloData{User: "alice"})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "hello", Data: payload}); writeErr != nil {
		t.Fatalf("send hello: %v", writeErr)
	}
	joinPayload, _ := json.Marshal(proto.JoinData{Room: "general"})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "join", Data: joinPayload}); writeErr != nil {
		t.Fatalf("send join: %v", writeErr)
	}

	msgPayload, _ := json.Marshal(proto.MsgData{Room: "general", Text: "first"})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "msg", Data: msgPayload}); writeErr != nil {
		t.Fatalf("send msg1: %v", writeErr)
	}

	msgPayload2, _ := json.Marshal(proto.MsgData{Room: "general", Text: "second"})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "msg", Data: msgPayload2}); writeErr != nil {
		t.Fatalf("send msg2: %v", writeErr)
	}

	for {
		var outbound proto.Outbound
		if err := wsjson.Read(ctx, conn, &outbound); err != nil {
			t.Fatalf("read outbound: %v", err)
		}
		if outbound.Type == "error" && outbound.Error != nil {
			if outbound.Error.Code != "rate_limited" {
				t.Fatalf("expected rate_limited error, got %+v", outbound)
			}
			return
		}
	}
}

// TestWebSocketDirectRoomJoin tests WebSocket join functionality for direct message rooms
func TestWebSocketDirectRoomJoin(t *testing.T) {
	// Create test store with schema
	testStore := createTestStore(t)
	defer testStore.Close()

	// Create auth service
	authService := createTestAuthService(t, testStore, "test-secret")

	hub := core.NewHub(testStore)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	disabledLogger := zerolog.New(io.Discard)

	cfg := config.Config{
		Addr:              ":0",
		ReadHeaderTimeout: time.Second,
		ShutdownTimeout:   time.Second,
		MaxMessageBytes:   1 << 20,
		JWTSecret:         "test-secret",
	}

	server := NewServer(hub, authService, testStore, &cfg, &disabledLogger)
	ts := httptest.NewServer(server.Handler)
	defer ts.Close()

	// Register three test users
	token1, err := authService.Register(context.Background(), "user1", "password123")
	if err != nil {
		t.Fatalf("failed to register user1: %v", err)
	}

	token2, err := authService.Register(context.Background(), "user2", "password123")
	if err != nil {
		t.Fatalf("failed to register user2: %v", err)
	}

	token3, err := authService.Register(context.Background(), "user3", "password123")
	if err != nil {
		t.Fatalf("failed to register user3: %v", err)
	}

	// Create direct room between user1 and user2 via REST API
	reqBody := strings.NewReader(`{"user_id":2}`)
	httpReq, err := http.NewRequest("POST", ts.URL+"/api/rooms/direct", reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token1)

	resp, err := ts.Client().Do(httpReq)
	if err != nil {
		t.Fatalf("failed to create direct room: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var roomResp struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	err = json.NewDecoder(resp.Body).Decode(&roomResp)
	if err != nil {
		t.Fatalf("failed to decode room response: %v", err)
	}

	if roomResp.Type != "direct" {
		t.Fatalf("expected room type 'direct', got '%s'", roomResp.Type)
	}

	roomName := roomResp.Name

	// Test 1: User1 can join the direct room via WebSocket
	wsURL := strings.Replace(ts.URL, "http", "ws", 1) + "/ws"
	wsCtx, wsCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer wsCancel()

	conn1, _, err := websocket.Dial(wsCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial ws: %v", err)
	}
	defer conn1.Close(websocket.StatusNormalClosure, "test done")

	// Send hello with user1 token
	helloData1, _ := json.Marshal(proto.HelloData{User: "user1", Token: token1, Protocol: 1})
	err = wsjson.Write(wsCtx, conn1, proto.Inbound{Type: "hello", Data: helloData1})
	if err != nil {
		t.Fatalf("send hello user1: %v", err)
	}

	// Send join to direct room
	joinData1, _ := json.Marshal(proto.JoinData{Room: roomName})
	err = wsjson.Write(wsCtx, conn1, proto.Inbound{Type: "join", Data: joinData1})
	if err != nil {
		t.Fatalf("send join user1: %v", err)
	}

	// Read user_joined event
	var outbound1 proto.Outbound
	err = wsjson.Read(wsCtx, conn1, &outbound1)
	if err != nil {
		t.Fatalf("read user1 join event: %v", err)
	}

	if outbound1.Type != "event" || outbound1.Event != "user_joined" {
		t.Fatalf("expected user_joined event, got type=%s event=%s", outbound1.Type, outbound1.Event)
	}

	// Test 2: User2 can also join the direct room via WebSocket
	conn2, _, err := websocket.Dial(wsCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial ws for user2: %v", err)
	}
	defer conn2.Close(websocket.StatusNormalClosure, "test done")

	// Send hello with user2 token
	helloData2, _ := json.Marshal(proto.HelloData{User: "user2", Token: token2, Protocol: 1})
	err = wsjson.Write(wsCtx, conn2, proto.Inbound{Type: "hello", Data: helloData2})
	if err != nil {
		t.Fatalf("send hello user2: %v", err)
	}

	// Send join to direct room
	joinData2, _ := json.Marshal(proto.JoinData{Room: roomName})
	err = wsjson.Write(wsCtx, conn2, proto.Inbound{Type: "join", Data: joinData2})
	if err != nil {
		t.Fatalf("send join user2: %v", err)
	}

	// Read user_joined event for user2
	var outbound2 proto.Outbound
	err = wsjson.Read(wsCtx, conn2, &outbound2)
	if err != nil {
		t.Fatalf("read user2 join event: %v", err)
	}

	if outbound2.Type != "event" || outbound2.Event != "user_joined" {
		t.Fatalf("expected user_joined event for user2, got type=%s event=%s", outbound2.Type, outbound2.Event)
	}

	// Also read the user_joined event that user1 receives about user2
	var outbound1b proto.Outbound
	err = wsjson.Read(wsCtx, conn1, &outbound1b)
	if err != nil {
		t.Fatalf("read user2 join notification on user1: %v", err)
	}

	if outbound1b.Type != "event" || outbound1b.Event != "user_joined" {
		t.Fatalf("expected user_joined event notification on user1, got type=%s event=%s", outbound1b.Type, outbound1b.Event)
	}

	// Test 3: User3 (not a member) CANNOT join the direct room via WebSocket
	conn3, _, err := websocket.Dial(wsCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial ws for user3: %v", err)
	}
	defer conn3.Close(websocket.StatusNormalClosure, "test done")

	// Send hello with user3 token
	helloData3, _ := json.Marshal(proto.HelloData{User: "user3", Token: token3, Protocol: 1})
	if err := wsjson.Write(wsCtx, conn3, proto.Inbound{Type: "hello", Data: helloData3}); err != nil {
		t.Fatalf("send hello user3: %v", err)
	}

	// Send join to direct room - should be denied
	joinData3, _ := json.Marshal(proto.JoinData{Room: roomName})
	if err := wsjson.Write(wsCtx, conn3, proto.Inbound{Type: "join", Data: joinData3}); err != nil {
		t.Fatalf("send join user3: %v", err)
	}

	// Read error response
	var outbound3 proto.Outbound
	if err := wsjson.Read(wsCtx, conn3, &outbound3); err != nil {
		t.Fatalf("read user3 join error: %v", err)
	}

	if outbound3.Type != "error" || outbound3.Error == nil {
		t.Fatalf("expected error response for user3, got type=%s", outbound3.Type)
	}

	if outbound3.Error.Code != "access_denied" {
		t.Fatalf("expected access_denied error, got code=%s msg=%s", outbound3.Error.Code, outbound3.Error.Msg)
	}

	t.Log("TestWebSocketDirectRoomJoin: All tests passed")
}
