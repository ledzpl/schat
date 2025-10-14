package chat

// Client represents a connected participant in the chat room.
type Client struct {
	ID       string
	Username string
	Color    string

	send chan string
}

func newClient(id, username, color string) *Client {
	return &Client{
		ID:       id,
		Username: username,
		Color:    color,
		send:     make(chan string, 16),
	}
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
