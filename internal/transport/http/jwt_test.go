package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/proto"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func startJWTTestServer(t *testing.T, cfg config.Config) (*httptest.Server, context.CancelFunc) {
	t.Helper()

	hub := core.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	disabledLogger := zerolog.New(nil)

	server := NewServer(hub, &cfg, &disabledLogger)

	ts := httptest.NewServer(server.Handler)
	t.Cleanup(ts.Close)

	return ts, cancel
}

func makeJWT(secret, aud, iss, sub, name string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": sub,
		"exp": time.Now().Add(ttl).Unix(),
	}
	if aud != "" {
		claims["aud"] = aud
	}
	if iss != "" {
		claims["iss"] = iss
	}
	if name != "" {
		claims["name"] = name
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func TestWebSocketJWTSuccess(t *testing.T) {
	secret := "testsecret"
	cfg := config.Config{
		Addr:              ":0",
		ReadHeaderTimeout: time.Second,
		ShutdownTimeout:   time.Second,
		MaxMessageBytes:   1 << 20,
		JWTSecret:         secret,
		JWTRequired:       true,
	}
	ts, cancel := startJWTTestServer(t, cfg)
	defer cancel()

	token, err := makeJWT(secret, "", "", "user1", "Alice", time.Minute)
	if err != nil {
		t.Fatalf("make jwt: %v", err)
	}

	wsURL := strings.Replace(ts.URL, "http", "ws", 1) + "/ws"
	ctx, closeCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCtx()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	helloPayload, _ := json.Marshal(proto.HelloData{Token: token})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "hello", Data: helloPayload}); writeErr != nil {
		t.Fatalf("send hello: %v", writeErr)
	}

	joinPayload, _ := json.Marshal(proto.JoinData{Room: "general"})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "join", Data: joinPayload}); writeErr != nil {
		t.Fatalf("send join: %v", writeErr)
	}

	msgPayload, _ := json.Marshal(proto.MsgData{Room: "general", Text: "hi"})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "msg", Data: msgPayload}); writeErr != nil {
		t.Fatalf("send msg: %v", writeErr)
	}
}

func TestWebSocketJWTInvalid(t *testing.T) {
	secret := "testsecret"
	cfg := config.Config{
		Addr:              ":0",
		ReadHeaderTimeout: time.Second,
		ShutdownTimeout:   time.Second,
		MaxMessageBytes:   1 << 20,
		JWTSecret:         secret,
		JWTRequired:       true,
	}
	ts, cancel := startJWTTestServer(t, cfg)
	defer cancel()

	wsURL := strings.Replace(ts.URL, "http", "ws", 1) + "/ws"
	ctx, closeCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCtx()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	helloPayload, _ := json.Marshal(proto.HelloData{Token: "invalid"})
	if writeErr := wsjson.Write(ctx, conn, proto.Inbound{Type: "hello", Data: helloPayload}); writeErr != nil {
		t.Fatalf("send hello: %v", writeErr)
	}

	var outbound proto.Outbound
	if err := wsjson.Read(ctx, conn, &outbound); err != nil {
		t.Fatalf("read outbound: %v", err)
	}
	if outbound.Type != "error" || outbound.Error == nil || outbound.Error.Code != "unauthorized" {
		t.Fatalf("expected unauthorized error, got %+v", outbound)
	}
}
