package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/team/websocket-poc/internal/odds"
	"github.com/team/websocket-poc/internal/pubsub"
	"github.com/team/websocket-poc/internal/ws"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	redisAddr := envOr("REDIS_URL", "localhost:6379")
	addr := envOr("WS_ADDR", ":8080")

	hub := ws.NewHub()
	go hub.Run()

	// Subscribe to all odds channels and broadcast to WS clients
	rps := pubsub.NewRedisPubSub(redisAddr)
	defer rps.Close()

	go func() {
		err := rps.SubscribePattern(ctx, "odds:*", func(update odds.OddsUpdate) {
			start := time.Now()
			data, _ := json.Marshal(update)
			hub.Broadcast(update.MatchID, data)
			ws.FanoutLatency.Observe(time.Since(start).Seconds())
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("redis subscribe error: %v", err)
		}
	}()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := hub.NewClient(conn)
		go client.WritePump()
		go client.ReadPump()
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	http.Handle("/metrics", promhttp.Handler())

	log.Printf("WebSocket server starting on %s", addr)
	server := &http.Server{Addr: addr}
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
