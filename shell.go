package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
)

var (
	listen  = flag.Bool("s", false, "Start as server")
	host    = flag.String("h", "localhost", "Host name")
	port    = flag.Int("p", 9090, "Port")
	command = flag.Bool("l", false, "waiting for Server command")
	do      = flag.String("e", "cmd", "Command for perform")
)

func rec() {
	r := recover()
	if r != nil {
		fmt.Println(r)
	}
}

func treatEr(err error) {
	if err != nil {
		panic(err)
	}
}

func startServer() {
	addr := fmt.Sprintf("%s:%d", *host, *port)
	ln, err := net.Listen("tcp", addr)
	treatEr(err)
	for {
		conn, err := ln.Accept()
		treatEr(err)
		go processClient(conn)
	}
}

func processClient(conn net.Conn) {
	if *command {
		err := launchCMD(conn)
		if err != nil {
			conn.Close()
			panic(err)
		}
	}
	_, err := io.Copy(os.Stdout, conn)
	treatEr(err)
	conn.Close()
}

func launchCMD(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	treatEr(err)
	cmd := exec.Command(strings.TrimSpace(line))
	in, err := cmd.StdinPipe()
	treatEr(err)
	out, err := cmd.StdoutPipe()
	treatEr(err)
	go io.Copy(in, conn)
	go io.Copy(conn, out)
	return cmd.Run()
}

func startClient() {
	addr:=fmt.Sprintf("%s:%d", *host, *port)
	conn, err := net.Dial("tcp", addr)
	treatEr(err)
	if len(*do) > 0 {
		cmd := fmt.Sprintf("%s\n", *do)
		conn.Write([]byte(cmd))
	}
	go io.Copy(os.Stdout, conn)
	_, err = io.Copy(conn, os.Stdin)
	treatEr(err)
}

func main() {
	flag.Parse()
	defer rec()
	if *listen {
		startServer()
		return
	}
	startClient()
}
