package cache

import (
	"reflect"
	"testing"
	"time"

	model "example.com/rpl/session-cache/generated/session"
)

func TestSessionRoundTrip(t *testing.T) {
	now := time.Unix(1_700_000_000, 123).UTC()
	want := model.Session{
		Id:        "session-1234",
		UserId:    42,
		Token:     "0123456789abcdef",
		Roles:     []string{"reader", "writer"},
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
		DebugNote: "not persisted",
	}

	entry, err := Encode(want)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if entry.Key != "sessions:session-1234" {
		t.Fatalf("key = %q", entry.Key)
	}
	if entry.TTLSeconds != 3600 {
		t.Fatalf("ttl = %d", entry.TTLSeconds)
	}
	if _, exists := entry.Values["debug_note"]; exists {
		t.Fatal("ignored debug_note leaked into Redis hash")
	}

	got, err := Decode(entry.Values)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want.DebugNote = ""
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}
