package calculator_test

import (
	"context"
	"encoding/json"
	ffigo "example.com/rpl/ffi-service/generated/calculator_service/ffi/go"
	"fmt"
	"testing"
)

type fakeNative struct{}

func (fakeNative) Call(_ context.Context, method string, payload []byte) ([]byte, error) {
	if method != "add" {
		return nil, fmt.Errorf("unexpected method %q", method)
	}
	var request ffigo.AddRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, err
	}
	return json.Marshal(request.Left + request.Right)
}

func TestGeneratedGoFFIClient(t *testing.T) {
	client := ffigo.NewClient(fakeNative{})
	value, err := client.Add(context.Background(), ffigo.AddRequest{Left: 20, Right: 22})
	if err != nil {
		t.Fatal(err)
	}
	if value != 42 {
		t.Fatalf("Add() = %d, want 42", value)
	}
}
