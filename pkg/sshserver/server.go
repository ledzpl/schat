package sshserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

// SessionHandler handles an accepted SSH "session" channel.
type SessionHandler func(conn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request)

// Server wraps the SSH listener lifecycle.
type Server struct {
	Addr   string
	Config *ssh.ServerConfig

	logger *log.Logger
}

// New creates a Server with the provided host signer.
func New(addr string, signer ssh.Signer, logger *log.Logger) *Server {
	cfg := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	cfg.AddHostKey(signer)

	if logger == nil {
		logger = log.Default()
	}

	return &Server{
		Addr:   addr,
		Config: cfg,
		logger: logger,
	}
}

// ListenAndServe starts the SSH server until the context is cancelled or an error occurs.
func (s *Server) ListenAndServe(ctx context.Context, handler SessionHandler) error {
	if handler == nil {
		return errors.New("sshserver: session handler required")
	}

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("sshserver: listen %q: %w", s.Addr, err)
	}
	defer listener.Close()

	shutdown := make(chan struct{})
	defer close(shutdown)

	go func() {
		select {
		case <-ctx.Done():
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				s.logger.Printf("sshserver: listener close error: %v", err)
			}
		case <-shutdown:
		}
	}()

	s.logger.Printf("sshserver: listening on %s", listener.Addr())

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			s.logger.Printf("sshserver: accept error: %v", err)
			continue
		}

		go s.handleConn(ctx, conn, handler)
	}
}

func (s *Server) handleConn(ctx context.Context, tcpConn net.Conn, handler SessionHandler) {
	defer tcpConn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, s.Config)
	if err != nil {
		s.logger.Printf("sshserver: handshake failed: %v", err)
		return
	}
	defer sshConn.Close()

	s.logger.Printf("sshserver: new connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())

	go ssh.DiscardRequests(reqs)

	for {
		select {
		case <-ctx.Done():
			return
		case newChannel, ok := <-chans:
			if !ok {
				return
			}
			if newChannel.ChannelType() != "session" {
				newChannel.Reject(ssh.UnknownChannelType, "only session channels are supported")
				continue
			}

			channel, requests, err := newChannel.Accept()
			if err != nil {
				s.logger.Printf("sshserver: channel accept failed: %v", err)
				continue
			}

			go handler(sshConn, channel, requests)
		}
	}
}

// EphemeralSigner creates a temporary RSA host key for development environments.
func EphemeralSigner() (ssh.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("sshserver: generate host key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, fmt.Errorf("sshserver: create signer: %w", err)
	}

	return signer, nil
}
