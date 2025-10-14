package chat

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/crypto/ssh"
)

type sessionWriter struct {
	mu sync.Mutex
	ch ssh.Channel
}

func newSessionWriter(ch ssh.Channel) *sessionWriter {
	return &sessionWriter{ch: ch}
}

func (w *sessionWriter) writeString(s string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, err := io.WriteString(w.ch, s)
	return err
}

func (w *sessionWriter) printf(format string, args ...interface{}) error {
	return w.writeString(fmt.Sprintf(format, args...))
}

// HandleSession wires an SSH channel to the chat room.
func HandleSession(room *Room, conn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	client := room.AddClient(conn.User())

	var cleanup sync.Once
	defer cleanup.Do(func() {
		room.RemoveClient(client.ID)
	})

	writer := newSessionWriter(channel)

	shellReady := make(chan struct{})

	go func() {
		var once sync.Once
		for req := range requests {
			switch req.Type {
			case "shell":
				req.Reply(true, nil)
				once.Do(func() { close(shellReady) })
			case "pty-req", "env", "window-change", "signal":
				req.Reply(true, nil)
			default:
				req.Reply(false, nil)
			}
		}
		once.Do(func() { close(shellReady) })
	}()

	<-shellReady

	reader := bufio.NewReader(channel)
	var (
		line   []rune
		lineMu sync.RWMutex
	)

	renderPrompt := func() error {
		lineMu.RLock()
		current := string(line)
		lineMu.RUnlock()

		var b strings.Builder
		b.Grow(4 + len(current))
		b.WriteString("\r> ")
		b.WriteString(current)
		b.WriteString("\033[K")

		return writer.writeString(b.String())
	}

	printMessage := func(msg string) error {
		var b strings.Builder
		b.Grow(4 + len(msg))
		b.WriteString("\r\033[K")
		b.WriteString(msg)
		b.WriteString("\r\n")

		if err := writer.writeString(b.String()); err != nil {
			return err
		}
		return renderPrompt()
	}

	drainLine := func() string {
		lineMu.Lock()
		text := strings.TrimSpace(string(line))
		line = line[:0]
		lineMu.Unlock()
		return text
	}

	outboundDone := make(chan struct{})
	go func() {
		for msg := range client.Send() {
			if err := printMessage(msg); err != nil {
				break
			}
		}
		close(outboundDone)
	}()

	if err := printMessage(fmt.Sprintf("Welcome to schat, %s!", client.Username)); err != nil {
		goto exit
	}
	if err := printMessage("Type messages and press enter to chat. Ctrl+D to exit."); err != nil {
		goto exit
	}

loop:
	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if text := drainLine(); text != "" {
					if msg := room.Broadcast(client.ID, client.Username, text); msg != "" {
						if err := printMessage(msg); err != nil {
							break loop
						}
					}
				}
				break loop
			}

			_ = printMessage(fmt.Sprintf("[system] read error: %v", err))
			break loop
		}

		switch r {
		case '\r', '\n':
			if r == '\r' && reader.Buffered() > 0 {
				// Consume a buffered '\n' without blocking when the client sends "\r\n".
				if next, _, err := reader.ReadRune(); err == nil {
					if next != '\n' {
						_ = reader.UnreadRune()
					}
				}
			}

			if text := drainLine(); text != "" {
				if msg := room.Broadcast(client.ID, client.Username, text); msg != "" {
					if err := printMessage(msg); err != nil {
						break loop
					}
				}
				continue
			}
			if err := renderPrompt(); err != nil {
				break loop
			}
		case 0x03: // Ctrl+C
			lineMu.Lock()
			line = line[:0]
			lineMu.Unlock()
			if err := writer.writeString("\r\033[K^C\r\n"); err != nil {
				break loop
			}
			goto exit
		case 0x04: // Ctrl+D
			lineMu.Lock()
			line = line[:0]
			lineMu.Unlock()
			if err := writer.writeString("\r\033[K^D\r\n"); err != nil {
				break loop
			}
			goto exit
		case '\b', 0x7f:
			lineMu.Lock()
			if len(line) == 0 {
				lineMu.Unlock()
				continue
			}
			line = line[:len(line)-1]
			lineMu.Unlock()
			// Erase the character visually.
			if err := renderPrompt(); err != nil {
				break loop
			}
		default:
			if unicode.IsPrint(r) {
				lineMu.Lock()
				line = append(line, r)
				lineMu.Unlock()
				if err := renderPrompt(); err != nil {
					break loop
				}
			}
		}
	}

exit:
	cleanup.Do(func() {
		room.RemoveClient(client.ID)
	})
	<-outboundDone
}
