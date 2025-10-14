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
	buffer *lineBuffer
	writer *sessionWriter
	ui     *terminalUI

	outboundDone chan struct{}
	cleanup      sync.Once
}

func newSession(room *Room, username string, channel ssh.Channel, requests <-chan *ssh.Request) *session {
	return &session{
		room:     room,
		username: username,
		channel:  channel,
		requests: requests,
		buffer:   newLineBuffer(128),
	}
}

func (s *session) run() {
	s.client = s.room.AddClient(s.username)
	s.writer = newSessionWriter(s.channel)
	s.ui = newTerminalUI(s.writer)
	defer s.cleanupSession()

	if err := s.waitForShell(); err != nil {
		_ = s.printMessage(fmt.Sprintf("[system] failed to start shell: %v", err))
		return
	}

	if err := s.ui.ClearScreen(); err != nil {
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
			s.buffer.TrimLast()
			if err := s.renderPrompt(); err != nil {
				return err
			}
		default:
			if unicode.IsPrint(r) {
				s.buffer.Append(r)
				if err := s.renderPrompt(); err != nil {
					return err
				}
			}
		}
	}
}

func (s *session) submitLine() error {
	text := s.buffer.Drain()
	if strings.TrimSpace(text) == "" {
		return s.renderPrompt()
	}
	return s.broadcastLine(text)
}

func (s *session) handleEOF() error {
	return s.broadcastLine(s.buffer.Drain())
}

func (s *session) handleControl(label string) error {
	s.buffer.Reset()
	if err := s.ui.DisplayControlAck(label); err != nil {
		return err
	}
	return s.renderPrompt()
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
	header := fmt.Sprintf("Users online: %d", s.room.ClientCount())
	return s.ui.UpdatePrompt(header, s.buffer.Snapshot())
}

func (s *session) printMessage(msg string) error {
	if err := s.ui.DisplayMessage(msg); err != nil {
		return err
	}

	return s.renderPrompt()
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
