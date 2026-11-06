// Package pipe copies bytes in both directions between a network connection
// and a local reader/writer pair (usually stdin and stdout), the way a
// netcat-style tool does.
//
// It knows nothing about flags, signals, or how the connection was made, so
// it is small, has one job, and is easy to test with net.Pipe.
package pipe

import (
	"context"
	"errors"
	"io"
	"net"
)

// halfCloser is implemented by connections that can close only their writing
// half while still reading — for example *net.TCPConn.CloseWrite. When the
// local input reaches EOF we use it to tell the peer "no more data from me"
// without tearing down the whole connection, so the peer can still send its
// reply back.
type halfCloser interface {
	CloseWrite() error
}

// Join streams data between conn and the local in/out until the peer closes
// the connection, or ctx is cancelled. It returns nil on a normal close.
//
// Both directions run at once:
//
//   - in  -> conn : local input is forwarded to the peer; when it reaches EOF
//     the connection's write half is closed (if conn supports it) so the peer
//     sees EOF too.
//   - conn -> out : the peer's output is forwarded to out; when it ends the
//     peer has gone away, and Join returns.
//
// Join deliberately does not wait for the in -> conn direction: a reader such
// as os.Stdin can block forever with no way to interrupt it. Closing conn
// (which the caller does on return, and ctx cancellation does too) unblocks
// that goroutine so it does not leak.
func Join(ctx context.Context, conn io.ReadWriter, in io.Reader, out io.Writer) error {
	fromPeer := make(chan error, 1)
	go func() { fromPeer <- copyData(out, conn) }()

	go func() {
		// The result is intentionally dropped: once our input ends the only
		// thing left to do is signal the peer. Join's lifetime follows the
		// peer -> out direction below, not this one.
		_ = copyData(conn, in)
		if hc, ok := conn.(halfCloser); ok {
			_ = hc.CloseWrite()
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-fromPeer:
		return err
	}
}

// copyData is io.Copy with the ordinary end-of-stream conditions flattened to
// nil. io.Copy already stops cleanly on EOF; on top of that, the "use of
// closed network connection" error that appears when conn is closed from
// another goroutine is expected here, not a failure.
func copyData(dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, src)
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}
