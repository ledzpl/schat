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
		// Drop queued messages when the receiver is too slow; keeps the room responsive.
	}
}

// Room manages the set of connected clients and message fan-out.
type Room struct {
	mu      sync.RWMutex
	clients map[string]*Client

	sequence atomic.Uint64
	clock    func() time.Time
	colors   ColorPicker
}

const colorReset = "\033[0m"

var defaultColorPalette = []string{
	"\033[31m", // Red
	"\033[32m", // Green
	"\033[33m", // Yellow
	"\033[34m", // Blue
	"\033[35m", // Magenta
	"\033[36m", // Cyan
}

// ColorPicker represents a strategy for choosing a display color for new clients.
type ColorPicker interface {
	Next() string
}

// RoomOption customises room construction.
type RoomOption func(*Room)

// NewRoom constructs an empty chat room.
func NewRoom(opts ...RoomOption) *Room {
	room := &Room{
		clients: make(map[string]*Client),
		clock:   time.Now,
		colors:  newRandomColorPicker(defaultColorPalette),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(room)
		}
	}

	return room
}

// WithClock overrides the clock used for timestamps. Primarily useful in tests.
func WithClock(clock func() time.Time) RoomOption {
	return func(r *Room) {
		if clock != nil {
			r.clock = clock
		}
	}
}

// WithColorPicker injects a custom color picker implementation.
func WithColorPicker(picker ColorPicker) RoomOption {
	return func(r *Room) {
		if picker != nil {
			r.colors = picker
		}
	}
}

// ClientCount returns the number of active clients in the room.
func (r *Room) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
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
		Color:    r.nextColor(),
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
	ts := r.timestamp()

	r.mu.RLock()
	defer r.mu.RUnlock()

	msg := fmt.Sprintf("[%s] %s: %s", ts, r.senderLabelLocked(senderID, senderName), text)
	r.deliverLocked(senderID, msg)

	return msg
}

func (r *Room) broadcastSystem(text string) {
	ts := r.timestamp()
	msg := fmt.Sprintf("[%s] [system] %s", ts, text)

	r.mu.RLock()
	defer r.mu.RUnlock()
	r.deliverLocked("", msg)
}

func (r *Room) senderLabelLocked(senderID, fallbackName string) string {
	if sender, ok := r.clients[senderID]; ok {
		if sender.Color == "" {
			return sender.Username
		}
		return fmt.Sprintf("%s%s%s", sender.Color, sender.Username, colorReset)
	}
	return fallbackName
}

func (r *Room) deliverLocked(excludeID, msg string) {
	for id, client := range r.clients {
		if id == excludeID {
			continue
		}
		client.tryDeliver(msg)
	}
}

func (r *Room) nextColor() string {
	if r.colors == nil {
		return ""
	}
	return r.colors.Next()
}

func (r *Room) timestamp() string {
	if r.clock == nil {
		return time.Now().Format("2006-01-02 15:04:05")
	}
	return r.clock().Format("2006-01-02 15:04:05")
}

type randomColorPicker struct {
	mu      sync.Mutex
	palette []string
	rng     *rand.Rand
}

func newRandomColorPicker(palette []string) *randomColorPicker {
	if len(palette) == 0 {
		return nil
	}
	return &randomColorPicker{
		palette: append([]string(nil), palette...),
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (p *randomColorPicker) Next() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.palette) == 0 {
		return ""
	}
	return p.palette[p.rng.Intn(len(p.palette))]
}
