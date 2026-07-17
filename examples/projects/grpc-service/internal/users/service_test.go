package users

import (
	"context"
	"testing"

	generatedgrpc "example.com/rpl/grpc-service/generated/user/grpc"
)

func TestGeneratedGRPCAdapterCallsService(t *testing.T) {
	server := generatedgrpc.NewUserGRPCServer(NewService())
	created, err := server.Put(context.Background(), &generatedgrpc.UserMessage{
		Id: 7, Name: "Ada", Email: "ada@example.com",
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if created.Name != "Ada" {
		t.Fatalf("created name = %q", created.Name)
	}

	found, err := server.GetByID(context.Background(), &generatedgrpc.UserGetByIDRequest{Id: 7})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.Email != "ada@example.com" {
		t.Fatalf("found email = %q", found.Email)
	}
}

func TestGeneratedGRPCAdapterReturnsValidationError(t *testing.T) {
	server := generatedgrpc.NewUserGRPCServer(NewService())
	_, err := server.Put(context.Background(), &generatedgrpc.UserMessage{Name: "x", Email: "broken"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
