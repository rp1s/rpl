package main

import (
	"fmt"

	model "example.com/rpl/account-service/generated/account"
	"example.com/rpl/account-service/generated/account/validation"
)

func main() {
	account := model.Account{Email: "hello@example.com", DisplayName: "RPL User", Status: "active"}
	if err := validation.Validate(account); err != nil {
		panic(err)
	}
	fmt.Printf("account schema is ready: %s\n", account.Email)
}
