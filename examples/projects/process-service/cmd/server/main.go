package main

import (
	transport "example.com/rpl/process-service/generated/user/transport"
	"example.com/rpl/process-service/internal/users"
)

func main() {
	if err := transport.RunUserTransportServer(users.NewService()); err != nil {
		panic(err)
	}
}
