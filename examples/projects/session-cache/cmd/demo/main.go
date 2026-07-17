package main

import (
	"fmt"
	"time"

	model "example.com/rpl/session-cache/generated/session"
	cache "example.com/rpl/session-cache/internal/cache"
)

func main() {
	entry, err := cache.Encode(model.Session{
		Id: "session-demo", UserId: 1, Token: "0123456789abcdef",
		IssuedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("SET %s fields=%d ttl=%d\n", entry.Key, len(entry.Values), entry.TTLSeconds)
}
