package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/store"
)

func TestCreateRoom(t *testing.T) {
	// Create test store with schema
	testStore := createTestStore(t)
	defer testStore.Close()

	// Create auth service
	authService := createTestAuthService(t, testStore, "test-secret")

	hub := core.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	disabledLogger := zerolog.New(nil)

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

	// Register a test user
	token, err := authService.Register(context.Background(), "testuser", "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Test 1: Create room with valid token
	reqBody := bytes.NewBufferString(`{"name":"my-test-room"}`)
	req := httptest.NewRequest(http.MethodPost, ts.URL+"/api/rooms", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var roomResp RoomResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &roomResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if roomResp.Name != "my-test-room" {
		t.Errorf("expected room name 'my-test-room', got '%s'", roomResp.Name)
	}
	if roomResp.Type != "public" {
		t.Errorf("expected room type 'public', got '%s'", roomResp.Type)
	}
	if roomResp.OwnerID == nil || *roomResp.OwnerID != 1 {
		t.Errorf("expected owner_id 1, got %v", roomResp.OwnerID)
	}

	// Test 2: Create room without token
	reqBody = bytes.NewBufferString(`{"name":"should-fail"}`)
	req = httptest.NewRequest(http.MethodPost, ts.URL+"/api/rooms", reqBody)
	req.Header.Set("Content-Type", "application/json")

	resp = httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.Code)
	}

	// Test 3: Create duplicate room name
	reqBody = bytes.NewBufferString(`{"name":"my-test-room"}`)
	req = httptest.NewRequest(http.MethodPost, ts.URL+"/api/rooms", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp = httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestListRooms(t *testing.T) {
	// Create test store with schema
	testStore := createTestStore(t)
	defer testStore.Close()

	// Create auth service
	authService := createTestAuthService(t, testStore, "test-secret")

	hub := core.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	disabledLogger := zerolog.New(nil)

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

	// Register a test user
	token, err := authService.Register(context.Background(), "testuser", "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Create additional rooms
	ownerID := int64(1)
	_, err = testStore.CreateRoom(context.Background(), "room1", store.RoomTypePublic, &ownerID)
	if err != nil {
		t.Fatalf("failed to create room1: %v", err)
	}
	_, err = testStore.CreateRoom(context.Background(), "room2", store.RoomTypePublic, &ownerID)
	if err != nil {
		t.Fatalf("failed to create room2: %v", err)
	}

	// Test: List rooms
	req := httptest.NewRequest(http.MethodGet, ts.URL+"/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var rooms []RoomResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &rooms); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Should have 3 rooms: general (default) + room1 + room2
	if len(rooms) != 3 {
		t.Errorf("expected 3 rooms, got %d", len(rooms))
	}

	// Verify room names
	roomNames := make(map[string]bool)
	for _, room := range rooms {
		roomNames[room.Name] = true
	}

	expectedNames := []string{"general", "room1", "room2"}
	for _, name := range expectedNames {
		if !roomNames[name] {
			t.Errorf("expected room '%s' not found in list", name)
		}
	}

	// Test: List rooms without token
	req = httptest.NewRequest(http.MethodGet, ts.URL+"/api/rooms", nil)
	resp = httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.Code)
	}
}
