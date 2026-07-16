package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	usermodel "m/src/user"
	usergrpc "m/src/user/grpc"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:50051", "gRPC server address")
	id := flag.Int("id", 42, "user id")
	name := flag.String("name", "Alice", "user name")
	phone := flag.String("phone", "+79990000000", "user phone")
	deleteAfter := flag.Bool("delete", false, "delete the saved user after demo calls")
	timeout := flag.Duration("timeout", 5*time.Second, "request timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	conn, err := grpc.NewClient(
		*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("grpc.NewClient(%q): %v", *addr, err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("conn.Close(): %v", closeErr)
		}
	}()

	var service usergrpc.UserService = usergrpc.NewUserGRPCClient(conn)
	user := usermodel.User{
		Id:    *id,
		Name:  *name,
		Phone: *phone,
	}

	savedUser, err := service.Put(ctx, user)
	if err != nil {
		log.Fatalf("service.Put(): %v", err)
	}

	fetchedUser, err := service.GetByID(ctx, savedUser.Id)
	if err != nil {
		log.Fatalf("service.GetByID(): %v", err)
	}

	users, err := service.List(ctx)
	if err != nil {
		log.Fatalf("service.List(): %v", err)
	}

	idString, err := service.String(ctx)
	if err != nil {
		log.Fatalf("service.String(): %v", err)
	}

	fmt.Printf("Put => id=%d name=%q phone=%q\n", savedUser.Id, savedUser.Name, savedUser.Phone)
	fmt.Printf("GetByID(%d) => name=%q phone=%q\n", fetchedUser.Id, fetchedUser.Name, fetchedUser.Phone)
	fmt.Printf("List => %d user(s)\n", len(users))
	for _, item := range users {
		fmt.Printf("  id=%d name=%q phone=%q\n", item.Id, item.Name, item.Phone)
	}
	fmt.Printf("String() => %q\n", idString)

	if *deleteAfter {
		if err := service.Delete(ctx, savedUser.Id); err != nil {
			log.Fatalf("service.Delete(): %v", err)
		}

		usersAfterDelete, err := service.List(ctx)
		if err != nil {
			log.Fatalf("service.List() after delete: %v", err)
		}

		fmt.Printf("Delete(%d) => ok\n", savedUser.Id)
		fmt.Printf("List after delete => %d user(s)\n", len(usersAfterDelete))
	}
}
