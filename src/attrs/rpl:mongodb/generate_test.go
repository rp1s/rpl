package main

import (
	"go/format"
	"strings"
	"testing"
)

func TestGenerateMongoIndexesHelpersProducesValidGo(t *testing.T) {
	fields := []mongoFieldMeta{
		{BSONName: "_id", Unique: true},
		{BSONName: "email", Unique: true, Search: true},
		{BSONName: "name", Index: true, Search: true},
	}

	generated := generateMongoIndexesHelpers("userMongo", "users", fields)
	source := "package mongodb\n\n" + generated
	if _, err := format.Source([]byte(source)); err != nil {
		t.Fatalf("generated MongoDB indexes are invalid Go: %v\n%s", err, source)
	}
	if !strings.Contains(generated, `SetName("users_search_text")},`) {
		t.Fatalf("last index entry has no trailing comma:\n%s", generated)
	}
}
