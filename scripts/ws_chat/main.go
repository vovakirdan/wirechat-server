package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/vovakirdan/wirechat-server/internal/proto"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func main() {
	if err := run(); err != nil {
		log.Printf("ws_chat: %v", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "ws://localhost:8080/ws", "WebSocket address")
	user := flag.String("user", "cli-user", "username")
	room := flag.String("room", "general", "room to join")
	flag.Parse()

	baseCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(baseCtx)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, *addr, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	send := func(v interface{}) {
		if writeErr := wsjson.Write(ctx, conn, v); writeErr != nil {
			cancel()
			log.Printf("send: %v", writeErr)
		}
	}

	helloPayload, err := json.Marshal(proto.HelloData{User: *user})
	if err != nil {
		return fmt.Errorf("marshal hello: %w", err)
	}
	send(proto.Inbound{Type: "hello", Data: helloPayload})

	joinPayload, err := json.Marshal(proto.JoinData{Room: *room})
	if err != nil {
		return fmt.Errorf("marshal join: %w", err)
	}
	send(proto.Inbound{Type: "join", Data: joinPayload})

	fmt.Printf("Connected to %s as %s in room %s\n", *addr, *user, *room)
	fmt.Println("Type messages and press Enter to send. Ctrl+C to exit.")

	go func() {
		defer cancel()
		readLoop(ctx, conn)
	}()

	writeLoop(ctx, conn, *room)

	stop()
	cancel()
	_ = conn.Close(websocket.StatusNormalClosure, "bye")
	return nil
}

func readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		var outbound proto.Outbound
		if err := wsjson.Read(ctx, conn, &outbound); err != nil {
			// Treat expected shutdowns quietly.
			if errors.Is(err, context.Canceled) {
				return
			}
			switch websocket.CloseStatus(err) {
			case websocket.StatusNormalClosure, websocket.StatusGoingAway:
				return
			}
			log.Printf("read error: %v", err)
			return
		}

		switch outbound.Event {
		case "message":
			raw, err := json.Marshal(outbound.Data)
			if err != nil {
				log.Printf("marshal outbound data: %v", err)
				continue
			}
			var evt proto.EventMessage
			if err := json.Unmarshal(raw, &evt); err != nil {
				log.Printf("unmarshal message: %v", err)
				continue
			}
			fmt.Printf("[%s] %s: %s\n", evt.Room, evt.User, evt.Text)
		case "user_joined":
			raw, err := json.Marshal(outbound.Data)
			if err != nil {
				log.Printf("marshal outbound data: %v", err)
				continue
			}
			var evt proto.EventUserJoined
			if err := json.Unmarshal(raw, &evt); err != nil {
				log.Printf("unmarshal user_joined: %v", err)
				continue
			}
			fmt.Printf("[room %s] %s joined\n", evt.Room, evt.User)
		case "user_left":
			raw, err := json.Marshal(outbound.Data)
			if err != nil {
				log.Printf("marshal outbound data: %v", err)
				continue
			}
			var evt proto.EventUserLeft
			if err := json.Unmarshal(raw, &evt); err != nil {
				log.Printf("unmarshal user_left: %v", err)
				continue
			}
			fmt.Printf("[room %s] %s left\n", evt.Room, evt.User)
		default:
			fmt.Printf("event=%s data=%v\n", outbound.Event, outbound.Data)
		}
	}
}

func writeLoop(ctx context.Context, conn *websocket.Conn, room string) {
	lines := make(chan string)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-lines:
			if !ok {
				return
			}
			text := strings.TrimSpace(line)
			if text == "" {
				continue
			}

			payload, err := json.Marshal(proto.MsgData{Room: room, Text: text})
			if err != nil {
				log.Printf("marshal msg: %v", err)
				return
			}
			if err := wsjson.Write(ctx, conn, proto.Inbound{Type: "msg", Data: payload}); err != nil {
				log.Printf("send error: %v", err)
				return
			}
		}
	}
}
