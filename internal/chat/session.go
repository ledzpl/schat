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

// errShellNotRequested indicates the SSH client closed the request stream without asking for a shell.
var errShellNotRequested = errors.New("shell request not received before channel closed")

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

	workers sync.WaitGroup
	cleanup sync.Once
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
	defer s.cleanupSession()

	if err := s.setup(); err != nil {
		if errors.Is(err, errShellNotRequested) {
			return
		}
		s.printSystemError(err)
		return
	}

	if err := s.readLoop(); err != nil {
		s.handleReadError(err)
	}
}

func (s *session) setup() error {
	s.writer = newSessionWriter(s.channel)
	s.ui = newTerminalUI(s.writer)

	if err := s.awaitShell(); err != nil {
		return fmt.Errorf("await shell: %w", err)
	}

	s.client = s.room.AddClient(s.username)

	if err := s.ui.ClearScreen(); err != nil {
		return fmt.Errorf("prepare terminal: %w", err)
	}

	s.startOutboundRelay()

	if err := s.sendGreeting(); err != nil {
		return fmt.Errorf("send greeting: %w", err)
	}

	return nil
}

// awaitShell drains SSH channel requests and blocks until the client requests a shell.
func (s *session) awaitShell() error {
	for req := range s.requests {
		if !s.handleRequest(req) {
			continue
		}

		s.startRequestPump()
		return nil
	}
	return errShellNotRequested
}

func (s *session) handleRequest(req *ssh.Request) bool {
	switch req.Type {
	case "shell":
		req.Reply(true, nil)
		return true
	case "pty-req", "env", "window-change", "signal":
		req.Reply(true, nil)
	default:
		req.Reply(false, nil)
	}
	return false
}

func (s *session) startRequestPump() {
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		for req := range s.requests {
			s.handleRequest(req)
		}
	}()
}

func (s *session) startOutboundRelay() {
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
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

		if err := s.processRune(reader, r); err != nil {
			return err
		}
	}
}

// processRune handles interactive input keeping the buffer, screen, and control flow in sync.
func (s *session) processRune(reader *bufio.Reader, r rune) error {
	switch r {
	case '\r', '\n':
		return s.handleNewline(reader, r)
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
		return s.renderPrompt()
	default:
		if unicode.IsPrint(r) {
			s.buffer.Append(r)
			return s.renderPrompt()
		}
		return nil
	}
}

func (s *session) handleNewline(reader *bufio.Reader, r rune) error {
	if r == '\r' && reader.Buffered() > 0 {
		if next, _, err := reader.ReadRune(); err == nil {
			if next != '\n' {
				_ = reader.UnreadRune()
			}
		}
	}
	return s.submitLine()
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
		if s.channel != nil {
			_ = s.channel.Close()
		}
		s.workers.Wait()
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

func (s *session) handleReadError(err error) {
	switch {
	case errors.Is(err, errSessionTerminated):
		return
	case errors.Is(err, io.EOF):
		return
	default:
		s.printSystemError(fmt.Errorf("read error: %w", err))
	}
}

func (s *session) printSystemError(err error) {
	_ = s.printMessage(fmt.Sprintf("[system] %v", err))
}
