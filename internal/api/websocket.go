package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"twitch-smm-bot/internal/storage"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected (total: %d)", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastBotStatus(botID, status string) {
	msg := WSMessage{
		Type: "bot_status_changed",
		Payload: BotStatusPayload{
			BotID:  botID,
			Status: status,
		},
	}
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

func (h *Hub) BroadcastBotMessage(botID, message, personality, channel string) {
	msg := WSMessage{
		Type: "bot_message_sent",
		Payload: BotMessagePayload{
			BotID:       botID,
			Message:     message,
			Personality: personality,
			Channel:     channel,
			Timestamp:   time.Now(),
		},
	}
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

func (h *Hub) BroadcastStats(stats StatsDTO) {
	msg := WSMessage{
		Type:    "stats_updated",
		Payload: stats,
	}
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

func (h *Hub) BroadcastBotLog(botID string, logEntry storage.BotLog) {
	msg := WSMessage{
		Type: "bot_log",
		Payload: BotLogPayload{
			BotID:     botID,
			LogType:   logEntry.LogType,
			Level:     logEntry.Level,
			Message:   logEntry.Message,
			Context:   logEntry.Context,
			Timestamp: logEntry.CreatedAt,
		},
	}
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

func (h *Hub) BroadcastSettingsLog(logEntry storage.SettingsLog) {
	msg := WSMessage{
		Type: "settings_log",
		Payload: SettingsLogPayload{
			Level:     logEntry.Level,
			Message:   logEntry.Message,
			Changes:   logEntry.Changes,
			Timestamp: logEntry.CreatedAt,
		},
	}
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

func (h *Hub) BroadcastAudioContext(context string) {
	log.Printf("[WebSocket] Broadcasting audio context to %d clients: %s", len(h.clients), context)
	msg := WSMessage{
		Type: "audio_context",
		Payload: map[string]interface{}{
			"context": context,
		},
	}
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}
