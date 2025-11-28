package core

import (
	"context"
	"testing"
	"time"
)

func TestHubRoomBroadcast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hub := NewHub()
	go hub.Run(ctx)

	alice := NewClient("a", "alice")
	bob := NewClient("b", "bob")

	hub.RegisterClient(alice)
	hub.RegisterClient(bob)

	alice.Commands <- &Command{Kind: CommandJoinRoom, Room: "general"}
	bob.Commands <- &Command{Kind: CommandJoinRoom, Room: "general"}

	alice.Commands <- &Command{
		Kind: CommandSendRoomMessage,
		Room: "general",
		Message: Message{
			Text: "hi",
		},
	}

	for {
		select {
		case event := <-bob.Events:
			if event.Kind != EventRoomMessage {
				continue
			}
			if event.Message.Text != "hi" || event.Message.Room != "general" || event.Message.From != "alice" {
				t.Fatalf("unexpected event: %+v", event)
			}
			return
		case <-ctx.Done():
			t.Fatalf("did not receive broadcast in time")
		}
	}
}
