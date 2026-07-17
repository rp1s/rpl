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

func TestGenerateMongoIndexesHelpersBuildsCompoundIndexes(t *testing.T) {
	fields := []mongoFieldMeta{
		{BSONName: "tenant_id", IndexGroup: "tenant_created", IndexOrder: 1, Unique: true},
		{BSONName: "created_at", IndexGroup: "tenant_created", IndexOrder: -1},
	}

	generated := generateMongoIndexesHelpers("eventMongo", "events", fields)
	for _, want := range []string{
		`bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}}`,
		`SetName("events_tenant_created_idx")`,
		`.SetUnique(true)`,
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("compound indexes do not contain %q:\n%s", want, generated)
		}
	}
}
