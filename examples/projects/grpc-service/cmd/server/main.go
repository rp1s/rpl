package main

import (
	"flag"
	"log"
	"net"

	generatedgrpc "example.com/rpl/grpc-service/generated/user/grpc"
	"example.com/rpl/grpc-service/internal/users"
	"google.golang.org/grpc"
)

func main() {
	address := flag.String("listen", ":50051", "TCP address for the gRPC server")
	flag.Parse()

	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	generatedgrpc.RegisterUserGRPC(server, users.NewService())
	log.Printf("gRPC user service listens on %s", *address)
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
