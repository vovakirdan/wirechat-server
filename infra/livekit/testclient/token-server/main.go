// Token server for LiveKit smoke testing
// Generates JWT tokens for connecting to LiveKit rooms
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/livekit/protocol/auth"
)

func main() {
	apiKey := os.Getenv("LIVEKIT_API_KEY")
	apiSecret := os.Getenv("LIVEKIT_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		log.Fatal("LIVEKIT_API_KEY and LIVEKIT_API_SECRET must be set")
	}

	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		// CORS headers for local testing
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		room := r.URL.Query().Get("room")
		identity := r.URL.Query().Get("identity")

		if room == "" || identity == "" {
			http.Error(w, "room and identity query params required", http.StatusBadRequest)
			return
		}

		// Create access token
		at := auth.NewAccessToken(apiKey, apiSecret)
		grant := &auth.VideoGrant{
			RoomJoin: true,
			Room:     room,
		}
		at.SetVideoGrant(grant).
			SetIdentity(identity).
			SetValidFor(time.Hour)

		token, err := at.ToJWT()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to generate token: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(token))
	})

	addr := ":8082"
	log.Printf("Token server listening on %s", addr)
	log.Printf("Usage: GET /token?room=<room>&identity=<user>")
	log.Fatal(http.ListenAndServe(addr, nil))
}
