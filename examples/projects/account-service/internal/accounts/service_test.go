package accounts

import (
	"context"
	stdsql "database/sql"
	"strings"
	"testing"
	"time"

	model "example.com/rpl/account-service/generated/account"
)

type recordingDB struct {
	queries []string
	args    [][]any
}

func (db *recordingDB) ExecContext(_ context.Context, query string, args ...any) (stdsql.Result, error) {
	db.queries = append(db.queries, query)
	db.args = append(db.args, args)
	return nil, nil
}

func (db *recordingDB) QueryContext(context.Context, string, ...any) (*stdsql.Rows, error) {
	return nil, nil
}

func (db *recordingDB) QueryRowContext(context.Context, string, ...any) *stdsql.Row {
	return &stdsql.Row{}
}

func TestRegisterValidatesAndWritesAccount(t *testing.T) {
	db := &recordingDB{}
	service := New(db)
	ctx := context.Background()

	if err := service.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	account := model.Account{
		Id:          7,
		Email:       "team@example.com",
		DisplayName: "RPL Team",
		Status:      "active",
		CreatedAt:   time.Unix(1_700_000_000, 0).UTC(),
		UpdatedAt:   time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := service.Register(ctx, account); err != nil {
		t.Fatalf("register: %v", err)
	}

	if len(db.queries) < 2 {
		t.Fatalf("queries = %d, want schema and insert calls", len(db.queries))
	}
	if got := db.queries[len(db.queries)-1]; !strings.Contains(got, "INSERT INTO") {
		t.Fatalf("last query = %q, want INSERT", got)
	}
}

func TestRegisterRejectsInvalidAccount(t *testing.T) {
	db := &recordingDB{}
	err := New(db).Register(context.Background(), model.Account{Email: "broken", DisplayName: "x", Status: "unknown"})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(db.queries) != 0 {
		t.Fatalf("invalid account executed %d queries", len(db.queries))
	}
}
