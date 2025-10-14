package chat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLineBufferBasicOperations(t *testing.T) {
	buf := newLineBuffer(2)

	buf.Append('a')
	buf.Append('b')
	require.Equal(t, "ab", buf.Snapshot())

	buf.TrimLast()
	require.Equal(t, "a", buf.Snapshot())

	drained := buf.Drain()
	require.Equal(t, "a", drained)
	require.Equal(t, "", buf.Snapshot())

	buf.Append('c')
	require.Equal(t, "c", buf.Snapshot())

	buf.Reset()
	require.Equal(t, "", buf.Snapshot())
}
