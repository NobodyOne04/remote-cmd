# tcp-pipe

A tiny netcat-style tool: it connects your terminal's stdin and stdout to one
TCP connection and copies bytes in **both directions** at once. Use it to send
a file, receive one, or hold a two-way byte stream between two machines.

It moves data only — it never runs commands it receives. (This repo began as a
Go learning exercise that bound a shell to a socket; it was rewritten into a
safe data pipe that exercises the same ideas: `net`, goroutines, `io.Copy`,
`context` cancellation, flags, and clean error handling.)

## Build

```bash
go build -o tcp-pipe .     # or: make build
```

## Use

One side listens, the other dials. Then whatever you type on one end appears on
the other, and vice versa.

```bash
# terminal 1 — wait for a connection on port 9090
tcp-pipe -l :9090

# terminal 2 — connect to it
tcp-pipe localhost:9090
```

Because stdin and stdout are just streams, redirection turns it into a file
transfer:

```bash
tcp-pipe -l :9090 > received.bin     # receiver
tcp-pipe host:9090 < send.bin        # sender (exits when the file is sent)
```

When the sending side reaches end-of-input it half-closes the connection, so
the other end sees EOF, finishes writing, and both sides exit cleanly. `Ctrl-C`
stops either side at any time.

| Flag | Default | Meaning |
|---|---|---|
| `-l` | off | listen for one incoming connection instead of dialing |
| `-timeout` | `10s` | how long to wait when dialing (connect mode only) |

## How it works

```
         ┌─────────── tcp-pipe ───────────┐
 stdin ──┼──► in  ──► conn (write) ────────┼──► peer
stdout ◄─┼── out ◄── conn (read)  ◄────────┼─── peer
         └────────────────────────────────┘
```

- `main.go` parses flags, handles `Ctrl-C`/`SIGTERM`, and either dials or
  listens for one connection.
- `internal/pipe` is the core: [`Join`](internal/pipe/pipe.go) runs the two
  copy directions concurrently and returns when the peer closes. It takes
  plain `io` interfaces, so its tests use `net.Pipe` and need no real network.

## Develop

```bash
make check     # gofmt, go vet, race-enabled tests
make test      # tests only
```

## Decisions

Why it is built this way: [docs/adr](docs/adr/README.md).
