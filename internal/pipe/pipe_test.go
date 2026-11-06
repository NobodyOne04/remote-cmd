package pipe

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// Join should forward local input to the peer and the peer's reply back out,
// then return nil once the peer closes.
func TestJoinBothDirections(t *testing.T) {
	t.Parallel()
	local, peer := net.Pipe()
	defer local.Close()

	var got bytes.Buffer
	gotFromLocal := make(chan []byte, 1)

	// The peer reads one line from us, sends a reply, then closes.
	go func() {
		defer peer.Close()
		buf := make([]byte, len("hello\n"))
		io.ReadFull(peer, buf)
		gotFromLocal <- buf
		peer.Write([]byte("world\n"))
	}()

	in := bytes.NewBufferString("hello\n")
	if err := Join(context.Background(), local, in, &got); err != nil {
		t.Fatalf("Join: %v", err)
	}
	if g := <-gotFromLocal; string(g) != "hello\n" {
		t.Errorf("peer received %q, want %q", g, "hello\n")
	}
	if got.String() != "world\n" {
		t.Errorf("out = %q, want %q", got.String(), "world\n")
	}
}

// When ctx is cancelled, Join returns the context error promptly instead of
// blocking on input that never arrives.
func TestJoinContextCancel(t *testing.T) {
	t.Parallel()
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Join(ctx, local, blockingReader{}, io.Discard) }()

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Join did not return after cancel")
	}
}

// On a connection that supports half-close, Join must close the write half
// once local input hits EOF, so the peer learns there is no more data.
func TestJoinHalfClosesOnInputEOF(t *testing.T) {
	t.Parallel()
	local, peer := net.Pipe()
	defer peer.Close()

	rc := &recordingConn{Conn: local, writeClosed: make(chan struct{})}
	closed := make(chan error, 1)
	go func() { closed <- Join(context.Background(), rc, bytes.NewBufferString("hi"), io.Discard) }()

	// Drain what we sent so the in->conn copy can reach EOF.
	go io.Copy(io.Discard, peer)

	select {
	case <-rc.writeClosed:
	case <-time.After(time.Second):
		t.Fatal("CloseWrite was not called after input EOF")
	}

	peer.Close() // let Join finish
	if err := <-closed; err != nil {
		t.Errorf("Join: %v", err)
	}
}

type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) { select {} }

// recordingConn adds CloseWrite to a net.Conn and records that it was called.
type recordingConn struct {
	net.Conn
	once        sync.Once
	writeClosed chan struct{}
}

func (c *recordingConn) CloseWrite() error {
	c.once.Do(func() { close(c.writeClosed) })
	return nil
}
