package websocket

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Disable CORS for Tailscale (safe within VPN)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan interface{}
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan interface{}
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan interface{}, 256),
	}
}

func (h *Hub) Run() {
	log.Println("🔌 WebSocket hub started")

	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("✅ Client connected: %d active", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("❌ Client disconnected: %d active", len(h.clients))
			}

		case message := <-h.broadcast:
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("⚠️  Error broadcasting message: %v", err)
				continue
			}

			for client := range h.clients {
				select {
				case client.send <- data:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) Broadcast(message interface{}) {
	select {
	case h.broadcast <- message:
	default:
		log.Println("⚠️  Broadcast channel full, dropping message")
	}
}

func (h *Hub) ClientCount() int {
	return len(h.clients)
}

func HandleWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan interface{}, 256),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("⚠️  WebSocket error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, ok := message.([]byte)
			if !ok {
				// Try to marshal if it's not already bytes
				var err error
				data, err = json.Marshal(message)
				if err != nil {
					log.Printf("⚠️  Error marshaling message: %v", err)
					continue
				}
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("⚠️  Write error: %v", err)
				return
			}
		}
	}
}
