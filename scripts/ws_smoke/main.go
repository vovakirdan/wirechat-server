package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/proto"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func main() {
	if err := run(); err != nil {
		log.Printf("ws_smoke: %v", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "ws://localhost:8080/ws", "WebSocket address")
	user := flag.String("user", "tester", "username to announce with hello")
	room := flag.String("room", "general", "room name")
	text := flag.String("text", "hello from smoke test", "message text to send")
	timeout := flag.Duration("timeout", 5*time.Second, "total timeout for the run")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, *addr, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	mustSend := func(v interface{}) error {
		if err := wsjson.Write(ctx, conn, v); err != nil {
			return fmt.Errorf("send: %w", err)
		}
		return nil
	}

	if helloPayload, marshalErr := json.Marshal(proto.HelloData{User: *user}); marshalErr != nil {
		return fmt.Errorf("marshal hello: %w", marshalErr)
	} else {
		if err := mustSend(proto.Inbound{Type: "hello", Data: helloPayload}); err != nil {
			return err
		}
	}

	if joinPayload, marshalErr := json.Marshal(proto.JoinData{Room: *room}); marshalErr != nil {
		return fmt.Errorf("marshal join: %w", marshalErr)
	} else {
		if err := mustSend(proto.Inbound{Type: "join", Data: joinPayload}); err != nil {
			return err
		}
	}

	if msgPayload, marshalErr := json.Marshal(proto.MsgData{Room: *room, Text: *text}); marshalErr != nil {
		return fmt.Errorf("marshal msg: %w", marshalErr)
	} else {
		if err := mustSend(proto.Inbound{Type: "msg", Data: msgPayload}); err != nil {
			return err
		}
	}

	for {
		var outbound proto.Outbound
		if err := wsjson.Read(ctx, conn, &outbound); err != nil {
			return fmt.Errorf("read: %w", err)
		}

		fmt.Printf("Received outbound: type=%s", outbound.Type)
		if outbound.Event != "" {
			fmt.Printf(" event=%s", outbound.Event)
		}
		fmt.Println()

		if outbound.Err != "" {
			fmt.Printf("Error: %s\n", outbound.Err)
		}

		raw, err := json.Marshal(outbound.Data)
		if err != nil {
			return fmt.Errorf("marshal outbound data: %w", err)
		}

		switch outbound.Event {
		case "message":
			var evt proto.EventMessage
			if unmarshalErr := json.Unmarshal(raw, &evt); unmarshalErr != nil {
				fmt.Printf("Raw data: %s\n", string(raw))
				return fmt.Errorf("unmarshal message: %w", unmarshalErr)
			}
			fmt.Printf("EventMessage: room=%s user=%s text=%q ts=%d\n", evt.Room, evt.User, evt.Text, evt.TS)
			return nil
		case "user_joined":
			var evt proto.EventUserJoined
			if err := json.Unmarshal(raw, &evt); err == nil {
				fmt.Printf("Join: room=%s user=%s\n", evt.Room, evt.User)
			}
		case "user_left":
			var evt proto.EventUserLeft
			if err := json.Unmarshal(raw, &evt); err == nil {
				fmt.Printf("Left: room=%s user=%s\n", evt.Room, evt.User)
			}
		default:
			// keep looping for message
		}
	}
}
