package main

import (
	"strings"
	"testing"
)

func TestSQLitePaginationSupportsOffsetWithoutLimit(t *testing.T) {
	generated := generateSQLPaginationFunction("playerSQL", sqlDialectSQLite)
	if !strings.Contains(generated, `query += " LIMIT -1"`) {
		t.Fatalf("SQLite pagination does not add LIMIT -1: %s", generated)
	}
	if !strings.Contains(generated, `query += " OFFSET "`) {
		t.Fatalf("SQLite pagination does not append OFFSET: %s", generated)
	}
}

func TestPostgresPaginationDoesNotAddSQLiteLimit(t *testing.T) {
	generated := generateSQLPaginationFunction("playerSQL", sqlDialectPostgres)
	if strings.Contains(generated, "LIMIT -1") {
		t.Fatalf("Postgres pagination contains SQLite-only LIMIT -1: %s", generated)
	}
}

func TestUpdatedAtIsTouchedForEveryUpdate(t *testing.T) {
	fields := []sqlFieldMeta{
		{Field: "CreatedAt", Column: "created_at"},
		{Field: "UpdatedAt", Column: "updated_at", UpdatedAt: true},
	}

	touch := generateSQLTouchUpdatedAtValuesFunction("userSQL", fields)
	if !strings.Contains(touch, `[]int{1}`) || !strings.Contains(touch, "time.Now()") {
		t.Fatalf("updatedAt touch helper is incomplete: %s", touch)
	}

	update := generateSQLUpdateAssignmentsMethod("modelpkg.User", "User", "userSQL")
	if !strings.Contains(update, "userSQLTouchUpdatedAt(values)") {
		t.Fatalf("Update does not touch updatedAt values: %s", update)
	}

	upsert := generateSQLUpsertMethod("modelpkg.User", "User", "userSQL", "Executor")
	if !strings.Contains(upsert, "userSQLTouchUpdatedAt(values)") {
		t.Fatalf("Upsert does not touch updatedAt values: %s", upsert)
	}
}

func TestBuildWherePassesOriginalFilterKeyToNormalizer(t *testing.T) {
	generated := generateSQLBuildWhereFunction("userSQL")
	if !strings.Contains(generated, `userSQLNormalizeFilterValue(key, filters[key])`) {
		t.Fatalf("BuildWhere should pass the original filter key to NormalizeFilterValue:\n%s", generated)
	}
}

func TestStoreWrapsFunctionalAPI(t *testing.T) {
	generated := generateSQLStoreType("modelpkg.User", "User", "Executor", true)
	for _, want := range []string{
		"type Store struct",
		"func NewStore",
		"func (store *Store) Create",
		"func (store *Store) Upsert",
		"func (store *Store) WithExecutor",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("store does not contain %q:\n%s", want, generated)
		}
	}
}
