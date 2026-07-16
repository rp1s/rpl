package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os/signal"
	"syscall"

	usermodel "m/src/user"
	usergrpc "m/src/user/grpc"

	"google.golang.org/grpc"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:50051", "gRPC listen address")
	flag.Parse()

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}

	server := grpc.NewServer()
	seedUsers := []usermodel.User{
		{
			Id:    1,
			Name:  "initial",
			Phone: "+70000000000",
		},
	}

	var service usergrpc.UserService = newInMemoryUserService(seedUsers...)
	usergrpc.RegisterUserGRPC(server, service)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		server.GracefulStop()
	}()

	log.Printf("user gRPC server listening on %s with %d seed user(s)", *addr, len(seedUsers))

	if err := server.Serve(listener); err != nil && ctx.Err() == nil {
		log.Fatalf("grpc serve: %v", err)
	}
}
