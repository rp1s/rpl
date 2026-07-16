package main

import (
	"strings"
	"testing"
)

func TestCreateStatementsUseExecutableNewlinesAndQuotedIdentifiers(t *testing.T) {
	fields := []sqlFieldMeta{
		{Field: "Id", Column: "id", SQLType: "INTEGER", PrimaryKey: true},
		{Field: "Order", Column: "order", SQLType: "TEXT", Indexed: true},
	}

	generated := generateSQLCreateStatementsVar("itemSQL", "group", fields)
	if strings.Contains(generated, `\\n`) {
		t.Fatalf("DDL contains escaped backslash-n instead of a newline: %s", generated)
	}
	for _, want := range []string{
		`CREATE TABLE IF NOT EXISTS \"group\"`,
		`\"id\" INTEGER NOT NULL PRIMARY KEY`,
		`CREATE INDEX IF NOT EXISTS \"idx_group_order\" ON \"group\" (\"order\")`,
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated DDL does not contain %q:\n%s", want, generated)
		}
	}
}

func TestUpsertUsesPrimaryKeyAsConflictTarget(t *testing.T) {
	fields := []sqlFieldMeta{
		{Dialect: sqlDialectPostgres, Field: "TenantId", Column: "tenant_id", SQLType: "BIGINT", PrimaryKey: true},
		{Dialect: sqlDialectPostgres, Field: "Id", Column: "id", SQLType: "BIGINT", PrimaryKey: true},
		{Dialect: sqlDialectPostgres, Field: "Email", Column: "email", SQLType: "TEXT", Unique: true},
	}

	generated := generateSQLUpsertStatementConst("userSQL", "users", fields)
	if !strings.Contains(generated, `ON CONFLICT (\"tenant_id\", \"id\")`) {
		t.Fatalf("upsert does not use the composite primary key: %s", generated)
	}
	if !strings.Contains(generated, `\"email\" = EXCLUDED.\"email\"`) {
		t.Fatalf("non-key unique column should remain mutable: %s", generated)
	}
}

func TestUpsertWithMultipleUniqueColumnsUsesOneRealConstraint(t *testing.T) {
	fields := []sqlFieldMeta{
		{Dialect: sqlDialectSQLite, Field: "Id", Column: "id", SQLType: "INTEGER", Unique: true},
		{Dialect: sqlDialectSQLite, Field: "Email", Column: "email", SQLType: "TEXT", Unique: true},
		{Dialect: sqlDialectSQLite, Field: "Name", Column: "name", SQLType: "TEXT"},
	}

	generated := generateSQLUpsertStatementConst("userSQL", "users", fields)
	if !strings.Contains(generated, `ON CONFLICT (\"id\")`) {
		t.Fatalf("upsert does not target the first concrete unique constraint: %s", generated)
	}
	if strings.Contains(generated, `ON CONFLICT (\"id\", \"email\")`) {
		t.Fatalf("upsert targets a composite constraint that does not exist: %s", generated)
	}
}

func TestUpdateColumnsExcludePrimaryKeys(t *testing.T) {
	fields := []sqlFieldMeta{
		{Field: "Id", Column: "id", PrimaryKey: true},
		{Field: "Name", Column: "name"},
		{Field: "UpdatedAt", Column: "updated_at", UpdatedAt: true},
	}

	generated := generateSQLUpdateColumnIndexesVar("userSQL", fields)
	if !strings.Contains(generated, `[]int{1, 2}`) {
		t.Fatalf("unexpected update indexes: %s", generated)
	}
}

func TestSQLQuotedIdentifierEscapesQuotes(t *testing.T) {
	if got, want := sqlQuotedIdentifier(`a"b`), `"a""b"`; got != want {
		t.Fatalf("sqlQuotedIdentifier() = %q, want %q", got, want)
	}
}
