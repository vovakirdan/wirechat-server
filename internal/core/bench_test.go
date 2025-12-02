package core

import (
	"context"
	"testing"
)

func benchmarkRoomBroadcast(b *testing.B, recipients int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub()
	go hub.Run(ctx)

	sender := NewClient("sender", "sender", 0, false)
	hub.RegisterClient(sender)
	sender.Commands <- &Command{Kind: CommandJoinRoom, Room: "bench"}

	clients := make([]*Client, 0, recipients)
	for i := range recipients {
		c := NewClient("c"+string(rune('a'+i)), "client", 0, false)
		hub.RegisterClient(c)
		c.Commands <- &Command{Kind: CommandJoinRoom, Room: "bench"}
		clients = append(clients, c)
	}

	// Drain events for all but the first recipient to avoid channel backpressure.
	target := clients[0]
	for _, c := range clients[1:] {
		go func(cl *Client) {
			for range cl.Events {
			}
		}(c)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sender.Commands <- &Command{
			Kind: CommandSendRoomMessage,
			Room: "bench",
			Message: Message{
				Text: "payload",
			},
		}
		<-target.Events
	}
}

func BenchmarkRoomBroadcast_10(b *testing.B)  { benchmarkRoomBroadcast(b, 10) }
func BenchmarkRoomBroadcast_100(b *testing.B) { benchmarkRoomBroadcast(b, 100) }
func BenchmarkRoomBroadcast_500(b *testing.B) { benchmarkRoomBroadcast(b, 500) }
