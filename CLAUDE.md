# remote-cmd (tcp-pipe)

A tiny netcat-style tool in Go: pipes stdin/stdout over one TCP connection, both directions.
The repo is called remote-cmd for history; the binary and module code call it `tcp-pipe`.

## Commands
```sh
make build     # go build -o tcp-pipe .
make check     # gofmt check, go vet, go test -race ./...  (run before every PR)
make test      # tests only
```

## Layout
- `main.go`: flags, signals, dial or listen for one connection; all logic in `run()` so tests
  pass streams and args in.
- `internal/pipe/pipe.go`: `Join` (both copy directions, half-close on input EOF) and `copyData`.
- `main_test.go`, `internal/pipe/pipe_test.go`: tests with `net.Pipe` / loopback, no real network.
- `docs/adr/`: decision records (`.adr`), index in `docs/adr/README.md`.

## Conventions
- Read the ADR before changing a recorded decision; commit subjects carry its number.
- It moves data only: never add anything that executes what it receives (ADR 0002).
- Standard library only; Go 1.24 (`go.mod`).
- `internal/pipe` stays free of flags, signals and `os`: it takes `io` interfaces.
