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

const (
	ctrlC      = 0x03
	ctrlD      = 0x04
	backspace  = '\b'
	deleteChar = 0x7f
)

var errSessionTerminated = errors.New("session terminated")

// HandleSession wires an SSH channel to the chat room.
func HandleSession(room *Room, conn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	newSession(room, conn.User(), channel, requests).run()
}

type session struct {
	room     *Room
	username string

	channel  ssh.Channel
	requests <-chan *ssh.Request

	client *Client
	writer *sessionWriter

	lineMu sync.RWMutex
	line   []rune

	outboundDone chan struct{}
	cleanup      sync.Once
}

func newSession(room *Room, username string, channel ssh.Channel, requests <-chan *ssh.Request) *session {
	return &session{
		room:     room,
		username: username,
		channel:  channel,
		requests: requests,
		line:     make([]rune, 0, 128),
	}
}

func (s *session) run() {
	s.client = s.room.AddClient(s.username)
	s.writer = newSessionWriter(s.channel)
	defer s.cleanupSession()

	if err := s.waitForShell(); err != nil {
		_ = s.printMessage(fmt.Sprintf("[system] failed to start shell: %v", err))
		return
	}

	s.startOutboundPump()

	if err := s.sendGreeting(); err != nil {
		return
	}

	if err := s.readLoop(); err != nil && !errors.Is(err, errSessionTerminated) {
		if !errors.Is(err, io.EOF) {
			_ = s.printMessage(fmt.Sprintf("[system] read error: %v", err))
		}
	}
}

func (s *session) waitForShell() error {
	shellReady := make(chan struct{})

	go func() {
		var once sync.Once
		signalReady := func() { once.Do(func() { close(shellReady) }) }

		for req := range s.requests {
			switch req.Type {
			case "shell":
				req.Reply(true, nil)
				signalReady()
			case "pty-req", "env", "window-change", "signal":
				req.Reply(true, nil)
			default:
				req.Reply(false, nil)
			}
		}

		signalReady()
	}()

	<-shellReady
	return nil
}

func (s *session) startOutboundPump() {
	s.outboundDone = make(chan struct{})

	go func() {
		defer close(s.outboundDone)
		for msg := range s.client.Send() {
			if err := s.printMessage(msg); err != nil {
				return
			}
		}
	}()
}

func (s *session) sendGreeting() error {
	if err := s.printMessage(fmt.Sprintf("Welcome to schat, %s!", s.client.Username)); err != nil {
		return err
	}
	return s.printMessage("Type messages and press enter to chat. Ctrl+D to exit.")
}

func (s *session) readLoop() error {
	reader := bufio.NewReader(s.channel)

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return s.handleEOF()
			}
			return err
		}

		switch r {
		case '\r', '\n':
			if r == '\r' && reader.Buffered() > 0 {
				if next, _, err := reader.ReadRune(); err == nil {
					if next != '\n' {
						_ = reader.UnreadRune()
					}
				}
			}
			if err := s.submitLine(); err != nil {
				return err
			}
		case ctrlC:
			if err := s.handleControl("^C"); err != nil {
				return err
			}
			return errSessionTerminated
		case ctrlD:
			if err := s.handleControl("^D"); err != nil {
				return err
			}
			return errSessionTerminated
		case backspace, deleteChar:
			s.trimLastRune()
			if err := s.renderPrompt(); err != nil {
				return err
			}
		default:
			if unicode.IsPrint(r) {
				s.appendRune(r)
				if err := s.renderPrompt(); err != nil {
					return err
				}
			}
		}
	}
}

func (s *session) submitLine() error {
	text := s.drainLine()
	if strings.TrimSpace(text) == "" {
		return s.renderPrompt()
	}
	return s.broadcastLine(text)
}

func (s *session) handleEOF() error {
	return s.broadcastLine(s.drainLine())
}

func (s *session) handleControl(label string) error {
	s.resetLine()
	return s.writer.writeString(fmt.Sprintf("\r\033[K%s\r\n", label))
}

func (s *session) broadcastLine(text string) error {
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		if msg := s.room.Broadcast(s.client.ID, s.client.Username, trimmed); msg != "" {
			return s.printMessage(msg)
		}
	}
	return nil
}

func (s *session) renderPrompt() error {
	current := s.snapshotLine()

	var b strings.Builder
	b.Grow(4 + len(current))
	b.WriteString("\r> ")
	b.WriteString(current)
	b.WriteString("\033[K")

	return s.writer.writeString(b.String())
}

func (s *session) printMessage(msg string) error {
	var b strings.Builder
	b.Grow(4 + len(msg))
	b.WriteString("\r\033[K")
	b.WriteString(msg)
	b.WriteString("\r\n")

	if err := s.writer.writeString(b.String()); err != nil {
		return err
	}

	return s.renderPrompt()
}

func (s *session) appendRune(r rune) {
	s.lineMu.Lock()
	s.line = append(s.line, r)
	s.lineMu.Unlock()
}

func (s *session) trimLastRune() {
	s.lineMu.Lock()
	if len(s.line) > 0 {
		s.line = s.line[:len(s.line)-1]
	}
	s.lineMu.Unlock()
}

func (s *session) resetLine() {
	s.lineMu.Lock()
	s.line = s.line[:0]
	s.lineMu.Unlock()
}

func (s *session) drainLine() string {
	s.lineMu.Lock()
	text := string(s.line)
	s.line = s.line[:0]
	s.lineMu.Unlock()
	return text
}

func (s *session) snapshotLine() string {
	s.lineMu.RLock()
	defer s.lineMu.RUnlock()
	return string(s.line)
}

func (s *session) cleanupSession() {
	s.cleanup.Do(func() {
		if s.client != nil {
			s.room.RemoveClient(s.client.ID)
		}
		if s.outboundDone != nil {
			<-s.outboundDone
		}
	})
}

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
