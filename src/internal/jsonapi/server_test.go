package jsonapi

import (
	"rpl/internal/config"
	"testing"
)

func TestNewServerUsesDefaultConfig(t *testing.T) {
	if server := New(config.Default()); server == nil {
		t.Fatal("expected server")
	}
}
