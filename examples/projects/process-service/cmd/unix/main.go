package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	transport "example.com/rpl/process-service/generated/user/transport"
	"example.com/rpl/process-service/internal/users"
)

func main() {
	socket := flag.String("socket", "/tmp/rpl-users.sock", "Unix domain socket path")
	flag.Parse()

	server, err := transport.ListenUserUnix(*socket, users.NewService())
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()
	log.Printf("Unix transport listens on %s", *socket)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
}
