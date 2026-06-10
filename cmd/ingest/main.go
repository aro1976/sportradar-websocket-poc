package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/team/websocket-poc/internal/models"
	"github.com/team/websocket-poc/internal/odds"
	"github.com/team/websocket-poc/internal/pubsub"
	"github.com/team/websocket-poc/internal/sportradar"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client := sportradar.NewClient()
	redisAddr := envOr("REDIS_URL", "localhost:6379")
	recordingID := os.Getenv("RECORDING_ID")

	// List available recordings
	recordings, err := client.ListSoccerRecordings(ctx)
	if err != nil {
		log.Fatalf("failed to list recordings: %v", err)
	}
	fmt.Printf("Available Soccer recordings: %d\n", len(recordings))

	// Auto-select recording with push feeds
	if recordingID == "" {
		for _, r := range recordings {
			for _, api := range r.APIs {
				if api.APIType == "push" && api.Name == "events" {
					recordingID = r.ID
					fmt.Printf("Auto-selected: %s [%s]\n", r.Title, r.ID)
					break
				}
			}
			if recordingID != "" {
				break
			}
		}
	}
	if recordingID == "" {
		log.Fatal("no recordings with push events available")
	}

	// Init odds engine and Redis publisher
	engine := odds.NewEngine()
	rps := pubsub.NewRedisPubSub(redisAddr)
	defer rps.Close()

	fmt.Printf("Streaming events (recording=%s, redis=%s)...\n\n", recordingID, redisAddr)

	err = client.StreamEvents(ctx, recordingID, func(msg models.PushMessage) {
		if msg.Heartbeat != nil {
			return
		}
		if msg.Payload == nil || msg.Payload.Event == nil {
			return
		}

		matchID := ""
		if msg.Metadata != nil {
			matchID = msg.Metadata.SportEventID
		}
		if matchID == "" {
			return
		}

		// Generate odds from event
		updates := engine.ProcessEvent(matchID, msg.Payload.Event, msg.Payload.SportEventStatus)

		// Publish to Redis
		for _, u := range updates {
			if err := rps.Publish(ctx, u); err != nil {
				log.Printf("redis publish error: %v", err)
				continue
			}
			data, _ := json.Marshal(u)
			fmt.Printf("[%s] %d' %s → %s\n", matchID, msg.Payload.Event.MatchTime, msg.Payload.Event.Type, string(data))
		}
	})

	if err != nil && ctx.Err() == nil {
		log.Fatalf("stream error: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
