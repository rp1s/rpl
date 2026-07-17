package main

import (
	"log"
	"net/http"

	transport "example.com/rpl/process-service/generated/user/transport"
	"example.com/rpl/process-service/internal/users"
)

func main() {
	mux := http.NewServeMux()
	transport.RegisterUserHTTP(mux, users.NewService())
	log.Print("HTTP transport listens on http://127.0.0.1:8080/rpl/user/")
	if err := http.ListenAndServe("127.0.0.1:8080", mux); err != nil {
		log.Fatal(err)
	}
}
