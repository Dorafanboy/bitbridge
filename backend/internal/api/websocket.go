package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketManager manages WebSocket connections and broadcasts
type WebSocketManager struct {
	clients    map[*websocket.Conn]*WebSocketClient
	broadcast  chan []byte
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mutex      sync.RWMutex
}

// WebSocketClient represents a WebSocket client connection
type WebSocketClient struct {
	conn     *websocket.Conn
	send     chan []byte
	manager  *WebSocketManager
	clientID string
	topics   map[string]bool
	mutex    sync.RWMutex
}

// WebSocketMessage represents incoming WebSocket messages
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Action    string                 `json:"action"`
	Topic     string                 `json:"topic,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin in development
		// In production, implement proper origin checking
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		clients:    make(map[*websocket.Conn]*WebSocketClient),
		broadcast:  make(chan []byte),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
}

// Start starts the WebSocket manager
func (m *WebSocketManager) Start(ctx context.Context) {
	go m.run(ctx)
}

// run handles WebSocket client management
func (m *WebSocketManager) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-m.register:
			m.mutex.Lock()
			m.clients[client.conn] = client
			m.mutex.Unlock()
			log.Printf("WebSocket client connected: %s", client.clientID)
			
			// Send welcome message
			welcome := NewWebSocketResponse("system", "connected", map[string]interface{}{
				"client_id": client.clientID,
				"message":   "Connected to UTXO-EVM Gateway WebSocket",
			})
			client.sendMessage(welcome)
			
		case client := <-m.unregister:
			m.mutex.Lock()
			if _, ok := m.clients[client.conn]; ok {
				delete(m.clients, client.conn)
				close(client.send)
				log.Printf("WebSocket client disconnected: %s", client.clientID)
			}
			m.mutex.Unlock()
			
		case message := <-m.broadcast:
			m.mutex.RLock()
			for conn, client := range m.clients {
				select {
				case client.send <- message:
				default:
					delete(m.clients, conn)
					close(client.send)
				}
			}
			m.mutex.RUnlock()
		}
	}
}

// HandleWebSocket handles WebSocket connection upgrades
func (m *WebSocketManager) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	
	clientID := c.Query("client_id")
	if clientID == "" {
		clientID = generateClientID()
	}
	
	client := &WebSocketClient{
		conn:     conn,
		send:     make(chan []byte, 256),
		manager:  m,
		clientID: clientID,
		topics:   make(map[string]bool),
	}
	
	client.manager.register <- client
	
	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump handles reading messages from the WebSocket connection
func (c *WebSocketClient) readPump() {
	defer func() {
		c.manager.unregister <- c
		c.conn.Close()
	}()
	
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	
	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		
		var message WebSocketMessage
		if err := json.Unmarshal(messageBytes, &message); err != nil {
			log.Printf("WebSocket message parse error: %v", err)
			continue
		}
		
		c.handleMessage(&message)
	}
}

// writePump handles writing messages to the WebSocket connection
func (c *WebSocketClient) writePump() {
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
				log.Printf("WebSocket write error: %v", err)
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

// handleMessage processes incoming WebSocket messages
func (c *WebSocketClient) handleMessage(message *WebSocketMessage) {
	switch message.Action {
	case "subscribe":
		if message.Topic != "" {
			c.mutex.Lock()
			c.topics[message.Topic] = true
			c.mutex.Unlock()
			
			response := NewWebSocketResponse("system", "subscribed", map[string]interface{}{
				"topic":   message.Topic,
				"message": "Successfully subscribed to topic",
			})
			response.RequestID = message.RequestID
			c.sendMessage(response)
		}
		
	case "unsubscribe":
		if message.Topic != "" {
			c.mutex.Lock()
			delete(c.topics, message.Topic)
			c.mutex.Unlock()
			
			response := NewWebSocketResponse("system", "unsubscribed", map[string]interface{}{
				"topic":   message.Topic,
				"message": "Successfully unsubscribed from topic",
			})
			response.RequestID = message.RequestID
			c.sendMessage(response)
		}
		
	case "ping":
		response := NewWebSocketResponse("system", "pong", map[string]interface{}{
			"message": "pong",
		})
		response.RequestID = message.RequestID
		c.sendMessage(response)
		
	default:
		response := NewWebSocketResponse("error", "unknown_action", map[string]interface{}{
			"message": "Unknown action: " + message.Action,
		})
		response.RequestID = message.RequestID
		c.sendMessage(response)
	}
}

// sendMessage sends a message to the WebSocket client
func (c *WebSocketClient) sendMessage(message *WebSocketResponse) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("WebSocket message marshal error: %v", err)
		return
	}
	
	select {
	case c.send <- data:
	default:
		close(c.send)
	}
}

// BroadcastToTopic broadcasts a message to all clients subscribed to a topic
func (m *WebSocketManager) BroadcastToTopic(topic string, eventType, event string, data interface{}) {
	message := NewWebSocketResponse(eventType, event, data)
	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("WebSocket broadcast marshal error: %v", err)
		return
	}
	
	m.mutex.RLock()
	for _, client := range m.clients {
		client.mutex.RLock()
		if client.topics[topic] {
			select {
			case client.send <- messageBytes:
			default:
				close(client.send)
			}
		}
		client.mutex.RUnlock()
	}
	m.mutex.RUnlock()
}

// BroadcastToAll broadcasts a message to all connected clients
func (m *WebSocketManager) BroadcastToAll(eventType, event string, data interface{}) {
	message := NewWebSocketResponse(eventType, event, data)
	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("WebSocket broadcast marshal error: %v", err)
		return
	}
	
	m.broadcast <- messageBytes
}

// GetClientCount returns the number of connected clients
func (m *WebSocketManager) GetClientCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.clients)
}

// GetTopicSubscribers returns the number of subscribers for a topic
func (m *WebSocketManager) GetTopicSubscribers(topic string) int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	count := 0
	for _, client := range m.clients {
		client.mutex.RLock()
		if client.topics[topic] {
			count++
		}
		client.mutex.RUnlock()
	}
	
	return count
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return "client_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

// WebSocket event types and topics
const (
	// Event Types
	EventTypeSystem      = "system"
	EventTypeTransaction = "transaction"
	EventTypeBlock       = "block"
	EventTypeProof       = "proof"
	EventTypeSwap        = "swap"
	EventTypeError       = "error"
	
	// Topics
	TopicBitcoinBlocks       = "bitcoin.blocks"
	TopicBitcoinTransactions = "bitcoin.transactions"
	TopicEthereumBlocks      = "ethereum.blocks"
	TopicEthereumTransactions = "ethereum.transactions"
	TopicUTXOEvents          = "utxo.events"
	TopicProofGeneration     = "proof.generation"
	TopicSwapEvents          = "swap.events"
	TopicContractEvents      = "contract.events"
)