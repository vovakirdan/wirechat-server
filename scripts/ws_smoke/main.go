package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/proto"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func main() {
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
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	mustSend := func(v interface{}) {
		if err := wsjson.Write(ctx, conn, v); err != nil {
			log.Fatalf("send: %v", err)
		}
	}

	helloPayload, _ := json.Marshal(proto.HelloData{User: *user})
	mustSend(proto.Inbound{Type: "hello", Data: helloPayload})

	joinPayload, _ := json.Marshal(proto.JoinData{Room: *room})
	mustSend(proto.Inbound{Type: "join", Data: joinPayload})

	msgPayload, _ := json.Marshal(proto.MsgData{Room: *room, Text: *text})
	mustSend(proto.Inbound{Type: "msg", Data: msgPayload})

	for {
		var outbound proto.Outbound
		if err := wsjson.Read(ctx, conn, &outbound); err != nil {
			log.Fatalf("read: %v", err)
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
			log.Fatalf("marshal outbound data: %v", err)
		}

		switch outbound.Event {
		case "message":
			var evt proto.EventMessage
			if err := json.Unmarshal(raw, &evt); err != nil {
				fmt.Printf("Raw data: %s\n", string(raw))
				return
			}
			fmt.Printf("EventMessage: room=%s user=%s text=%q ts=%d\n", evt.Room, evt.User, evt.Text, evt.Ts)
			return
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
