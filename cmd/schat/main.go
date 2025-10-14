package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"

	"github.com/ledzpl/schat/internal/chat"
	"github.com/ledzpl/schat/pkg/sshserver"
)

func main() {
	addr := flag.String("addr", ":2222", "TCP address for the SSH chat server")
	hostKeyPath := flag.String("host-key", "configs/ssh_host_rsa", "Path to the SSH host private key (auto-generated if missing)")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.LstdFlags)

	signer, err := sshserver.LoadOrGenerateSigner(*hostKeyPath)
	if err != nil {
		logger.Fatalf("failed to prepare host key: %v", err)
	}

	room := chat.NewRoom()
	server := sshserver.New(*addr, signer, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err = server.ListenAndServe(ctx, func(conn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request) {
		chat.HandleSession(room, conn, channel, requests)
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatalf("server stopped with error: %v", err)
	}
}
