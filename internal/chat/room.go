package chat

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Client represents a connected participant in the chat room.
type Client struct {
	ID       string
	Username string
	Color    string
	send     chan string
}

// Send returns the outbound message channel for the client.
func (c *Client) Send() <-chan string {
	return c.send
}

// tryDeliver places a message onto the outbound channel without blocking.
func (c *Client) tryDeliver(msg string) {
	select {
	case c.send <- msg:
	default:
		// Drop when the receiver is too slow; keeps the room responsive.
	}
}

// Room manages the set of connected clients and message fan-out.
type Room struct {
	mu      sync.RWMutex
	clients map[string]*Client

	sequence atomic.Uint64
}

const colorReset = "\033[0m"

var (
	colorPalette = []string{
		"\033[31m", // Red
		"\033[32m", // Green
		"\033[33m", // Yellow
		"\033[34m", // Blue
		"\033[35m", // Magenta
		"\033[36m", // Cyan
	}
	rng   = rand.New(rand.NewSource(time.Now().UnixNano()))
	rngMu sync.Mutex
)

// NewRoom constructs an empty chat room.
func NewRoom() *Room {
	return &Room{
		clients: make(map[string]*Client),
	}
}

// AddClient registers a new client and returns it. The caller is responsible for
// removing the client when the session ends.
func (r *Room) AddClient(username string) *Client {
	id := fmt.Sprintf("user-%03d", r.sequence.Add(1))
	if username == "" {
		username = id
	}

	client := &Client{
		ID:       id,
		Username: username,
		Color:    pickColor(),
		send:     make(chan string, 16),
	}

	r.mu.Lock()
	r.clients[id] = client
	r.mu.Unlock()

	r.broadcastSystem(fmt.Sprintf("%s joined the chat", client.Username))
	return client
}

// RemoveClient unregisters the client and closes its outbound channel.
func (r *Room) RemoveClient(id string) {
	var client *Client

	r.mu.Lock()
	if existing, ok := r.clients[id]; ok {
		client = existing
		delete(r.clients, id)
	}
	r.mu.Unlock()

	if client != nil {
		close(client.send)
		r.broadcastSystem(fmt.Sprintf("%s left the chat", client.Username))
	}
}

// Broadcast delivers a message from the sender to all connected clients and returns the formatted line.
func (r *Room) Broadcast(senderID, senderName, text string) string {
	ts := time.Now().Format("2006-01-02 15:04:05")
	var (
		coloredName = senderName
		msg         string
	)

	r.mu.RLock()
	if sender, ok := r.clients[senderID]; ok {
		coloredName = fmt.Sprintf("%s%s%s", sender.Color, sender.Username, colorReset)
	}

	msg = fmt.Sprintf("[%s] %s: %s", ts, coloredName, text)
	for id, client := range r.clients {
		if id == senderID {
			continue
		}
		client.tryDeliver(msg)
	}
	r.mu.RUnlock()

	return msg
}

func (r *Room) broadcastSystem(text string) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("[%s] [system] %s", ts, text)

	r.mu.RLock()
	for _, client := range r.clients {
		client.tryDeliver(msg)
	}
	r.mu.RUnlock()
}

func pickColor() string {
	rngMu.Lock()
	defer rngMu.Unlock()
	return colorPalette[rng.Intn(len(colorPalette))]
}
