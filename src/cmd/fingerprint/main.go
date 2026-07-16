package main

import (
	"fmt"
	"rpl/pkg/fingerprint"
)

func main() {
	key, err := fingerprint.Fingerprint()
	if err != nil {
		panic(1)
	}
	fmt.Println(key)
}
