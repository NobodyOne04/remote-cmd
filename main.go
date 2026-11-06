// Command tcp-pipe connects the local stdin and stdout to a single TCP
// connection and copies bytes in both directions, like a minimal netcat.
//
// It either dials a peer or listens for one, then hands the connection to
// package pipe. It transfers data only; it never runs commands.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NobodyOne04/remote-cmd/internal/pipe"
)

const usage = `tcp-pipe — pipe stdin/stdout over a TCP connection, both directions.

Usage:
  tcp-pipe [flags] <host:port>      dial host:port and pipe
  tcp-pipe -l [flags] <host:port>   listen on host:port, accept one connection, pipe

Examples:
  tcp-pipe -l :9090                 # terminal 1: wait for a connection on port 9090
  tcp-pipe localhost:9090           # terminal 2: connect, then type and watch both sides
  tcp-pipe -l :9090 > got.bin       # receive a file...
  tcp-pipe host:9090 < send.bin     # ...streamed from here

Flags:
`

type config struct {
	listen      bool
	addr        string
	dialTimeout time.Duration
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "tcp-pipe:", err)
		os.Exit(1)
	}
}

// run holds all the logic so it can be tested without a real process: streams
// and arguments are passed in, and it returns an error instead of exiting.
func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	cfg, err := parseFlags(args, errOut)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // -h is a clean exit, not a failure
		}
		return err
	}

	// Ctrl-C / SIGTERM cancels ctx; the goroutine below then closes the
	// connection, which unblocks the copy and ends the pipe cleanly.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := connect(ctx, cfg, errOut)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	if err := pipe.Join(ctx, conn, in, out); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func parseFlags(args []string, errOut io.Writer) (config, error) {
	fs := flag.NewFlagSet("tcp-pipe", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { fmt.Fprint(errOut, usage); fs.PrintDefaults() }

	var cfg config
	fs.BoolVar(&cfg.listen, "l", false, "listen for one incoming connection instead of dialing")
	fs.DurationVar(&cfg.dialTimeout, "timeout", 10*time.Second, "dial timeout (connect mode only)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return config{}, errors.New("exactly one address (host:port) is required")
	}
	cfg.addr = fs.Arg(0)
	return cfg, nil
}

// connect returns a live connection, either by listening for one peer or by
// dialing. Both paths honour ctx cancellation.
func connect(ctx context.Context, cfg config, errOut io.Writer) (net.Conn, error) {
	if cfg.listen {
		var lc net.ListenConfig
		ln, err := lc.Listen(ctx, "tcp", cfg.addr)
		if err != nil {
			return nil, fmt.Errorf("listen on %s: %w", cfg.addr, err)
		}
		defer ln.Close()

		// Accept does not watch ctx on its own; closing the listener makes it
		// return so Ctrl-C works while we are still waiting for a connection.
		go func() {
			<-ctx.Done()
			ln.Close()
		}()

		fmt.Fprintf(errOut, "listening on %s\n", ln.Addr())
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("accept: %w", err)
		}
		fmt.Fprintf(errOut, "connection from %s\n", conn.RemoteAddr())
		return conn, nil
	}

	d := net.Dialer{Timeout: cfg.dialTimeout}
	conn, err := d.DialContext(ctx, "tcp", cfg.addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", cfg.addr, err)
	}
	return conn, nil
}
