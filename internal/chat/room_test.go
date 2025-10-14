package chat

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRoomBroadcastDeliversToOtherClients(t *testing.T) {
	t.Helper()

	rngMu.Lock()
	rng = rand.New(rand.NewSource(1))
	rngMu.Unlock()

	room := NewRoom()

	alice := room.AddClient("alice")
	drainChannel(alice.Send())

	bob := room.AddClient("bob")
	drainChannel(alice.Send())
	drainChannel(bob.Send())

	msg := room.Broadcast(alice.ID, alice.Username, "hello world")
	require.Contains(t, msg, "hello world")
	require.Contains(t, msg, "alice")

	select {
	case delivered := <-bob.Send():
		require.Equal(t, msg, delivered)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast")
	}

	select {
	case unexpected := <-alice.Send():
		t.Fatalf("sender should not receive message, got %q", unexpected)
	default:
	}
}

func TestRoomRemoveClientClosesChannel(t *testing.T) {
	room := NewRoom()
	client := room.AddClient("carol")
	drainChannel(client.Send())

	room.RemoveClient(client.ID)

	select {
	case _, ok := <-client.Send():
		require.False(t, ok, "channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for channel closure")
	}
}

func drainChannel(ch <-chan string) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
