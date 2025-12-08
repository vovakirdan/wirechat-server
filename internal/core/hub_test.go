package core

import (
	"context"
	"testing"
	"time"
)

func TestHubJoinBroadcastAndLeave(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	hub := NewHub(nil, nil) // No store or call service needed for this test
	go hub.Run(ctx)

	alice := NewClient("a", "alice", 0, false)
	bob := NewClient("b", "bob", 0, false)

	hub.RegisterClient(alice)
	hub.RegisterClient(bob)

	alice.Commands <- &Command{Kind: CommandJoinRoom, Room: "general"}
	bob.Commands <- &Command{Kind: CommandJoinRoom, Room: "general"}

	// Bob should see his own join event (broadcasted to room).
	joinEv := mustEvent(t, bob.Events, EventUserJoined)
	if joinEv.User != "bob" || joinEv.Room != "general" {
		t.Fatalf("unexpected join event: %+v", joinEv)
	}

	// Broadcast message from Alice.
	alice.Commands <- &Command{
		Kind: CommandSendRoomMessage,
		Room: "general",
		Message: Message{
			Text: "hi",
		},
	}

	msgEv := mustEvent(t, bob.Events, EventRoomMessage)
	if msgEv.Message.Text != "hi" || msgEv.Message.Room != "general" || msgEv.Message.From != "alice" {
		t.Fatalf("unexpected message event: %+v", msgEv)
	}

	// Alice leaves; Bob should see user_left.
	alice.Commands <- &Command{Kind: CommandLeaveRoom, Room: "general"}
	leftEv := mustEvent(t, bob.Events, EventUserLeft)
	if leftEv.User != "alice" || leftEv.Room != "general" {
		t.Fatalf("unexpected leave event: %+v", leftEv)
	}
}

func TestHubDoubleJoinProducesError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hub := NewHub(nil, nil) // No store or call service needed for this test
	go hub.Run(ctx)

	alice := NewClient("a", "alice", 0, false)
	hub.RegisterClient(alice)

	alice.Commands <- &Command{Kind: CommandJoinRoom, Room: "general"}
	alice.Commands <- &Command{Kind: CommandJoinRoom, Room: "general"}

	ev := mustEvent(t, alice.Events, EventError)
	if ev.Error == nil || ev.Error.Code != ErrCodeAlreadyJoined {
		t.Fatalf("expected already_joined error, got %+v", ev)
	}
}

func TestHubSendWithoutJoinProducesError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hub := NewHub(nil, nil) // No store or call service needed for this test
	go hub.Run(ctx)

	alice := NewClient("a", "alice", 0, false)
	hub.RegisterClient(alice)

	alice.Commands <- &Command{
		Kind:    CommandSendRoomMessage,
		Room:    "general",
		Message: Message{Text: "hi"},
	}

	ev := mustEvent(t, alice.Events, EventError)
	if ev.Error == nil || ev.Error.Code != ErrCodeNotInRoom {
		t.Fatalf("expected not_in_room error, got %+v", ev)
	}
}

func TestHubLeaveUnknownRoomError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hub := NewHub(nil, nil) // No store or call service needed for this test
	go hub.Run(ctx)

	alice := NewClient("a", "alice", 0, false)
	hub.RegisterClient(alice)

	alice.Commands <- &Command{Kind: CommandLeaveRoom, Room: "ghost"}

	ev := mustEvent(t, alice.Events, EventError)
	if ev.Error == nil || ev.Error.Code != ErrCodeRoomNotFound {
		t.Fatalf("expected room_not_found error, got %+v", ev)
	}
}
