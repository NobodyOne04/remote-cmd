package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// End to end over loopback: run dials a tiny echo server, and what we feed to
// stdin should come back on stdout.
func TestRunConnectEcho(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn) // echo everything, then close when the client half-closes
	}()

	var out bytes.Buffer
	err = run(context.Background(),
		[]string{ln.Addr().String()},
		strings.NewReader("ping\n"),
		&out, io.Discard)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.String() != "ping\n" {
		t.Errorf("out = %q, want %q", out.String(), "ping\n")
	}
}

func TestRunRejectsBadArgs(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"no address":    {},
		"two addresses": {"a:1", "b:2"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			err := run(context.Background(), args, strings.NewReader(""), io.Discard, io.Discard)
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestRunDialTimeout(t *testing.T) {
	t.Parallel()
	// 192.0.2.0/24 is TEST-NET-1 (RFC 5737): routable syntactically, never
	// answers, so the dial hits the timeout rather than a refusal.
	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(),
			[]string{"-timeout", "150ms", "192.0.2.1:9"},
			strings.NewReader(""), io.Discard, io.Discard)
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "dial") {
			t.Errorf("err = %v, want a dial error", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("dial did not time out")
	}
}

func TestRunHelpIsClean(t *testing.T) {
	t.Parallel()
	if err := run(context.Background(), []string{"-h"}, strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Errorf("run -h = %v, want nil", err)
	}
}

// Cancelling the context while listening (before any peer connects) is a clean
// shutdown, not an error.
func TestRunListenCancelClean(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"-l", "127.0.0.1:0"}, strings.NewReader(""), io.Discard, io.Discard)
	}()
	time.Sleep(50 * time.Millisecond) // let it reach Accept
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want nil or context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}
