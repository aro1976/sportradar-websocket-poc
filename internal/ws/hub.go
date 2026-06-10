package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 54 * time.Second
)

type Client struct {
	conn    *websocket.Conn
	send    chan []byte
	matches map[string]bool
	hub     *Hub
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	matchSubs  map[string]map[*Client]bool // matchID -> clients
	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		matchSubs:  make(map[string]map[*Client]bool),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
			ConnectionsActive.Inc()
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				for m := range c.matches {
					delete(h.matchSubs[m], c)
				}
				close(c.send)
				ConnectionsActive.Dec()
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Subscribe(c *Client, matchID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.matchSubs[matchID] == nil {
		h.matchSubs[matchID] = make(map[*Client]bool)
	}
	h.matchSubs[matchID][c] = true
	c.matches[matchID] = true
}

func (h *Hub) Broadcast(matchID string, data []byte) {
	h.mu.RLock()
	clients := h.matchSubs[matchID]
	h.mu.RUnlock()

	for c := range clients {
		select {
		case c.send <- data:
			MessagesSent.Inc()
		default:
			h.unregister <- c
		}
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) NewClient(conn *websocket.Conn) *Client {
	c := &Client{
		conn:    conn,
		send:    make(chan []byte, 256),
		matches: make(map[string]bool),
		hub:     h,
	}
	h.register <- c
	return c
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var req struct {
			Subscribe string `json:"subscribe"`
		}
		if json.Unmarshal(msg, &req) == nil && req.Subscribe != "" {
			c.hub.Subscribe(c, req.Subscribe)
			log.Printf("client subscribed to %s", req.Subscribe)
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
