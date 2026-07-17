package main

import (
	"fmt"
	"strings"
)

func generateSQLExecutorType(typeName string) string {
	return fmt.Sprintf(`type %s interface {
	ExecContext(ctx context.Context, query string, args ...any) (stdsql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*stdsql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *stdsql.Row
}`, typeName)
}

func generateSQLTableNameConst(prefix string, tableName string) string {
	return fmt.Sprintf("const TableName = %q\n\nconst %sTableName = %q", tableName, prefix, sqlQuotedIdentifier(tableName))
}

func generateSQLColumnType(fields []sqlFieldMeta) string {
	constants := make([]string, 0, len(fields))
	for _, field := range fields {
		constants = append(constants, fmt.Sprintf("Column%s Column = %q", field.Field, field.Column))
	}

	return fmt.Sprintf(`type Column string

const (
	%s
)

func (column Column) String() string {
	return string(column)
}

func Where(column Column, value any) map[string]any {
	return map[string]any{string(column): value}
}

func And(filters ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, filter := range filters {
		for column, value := range filter {
			result[column] = value
		}
	}
	return result
}`, strings.Join(constants, "\n\t"))
}

func generateSQLColumnNamesVar(prefix string, fields []sqlFieldMeta) string {
	items := make([]string, 0, len(fields))
	for _, field := range fields {
		items = append(items, fmt.Sprintf("%q", sqlQuotedIdentifier(field.Column)))
	}

	return fmt.Sprintf("var %sColumnNames = []string{%s}", prefix, strings.Join(items, ", "))
}

func generateSQLSearchColumnsVar(prefix string, fields []sqlFieldMeta) string {
	items := make([]string, 0)
	for _, field := range fields {
		if field.Searchable {
			items = append(items, fmt.Sprintf("%q", sqlQuotedIdentifier(field.Column)))
		}
	}

	return fmt.Sprintf("var %sSearchColumns = []string{%s}", prefix, strings.Join(items, ", "))
}

func generateSQLSelectStatementConst(prefix string, tableName string, fields []sqlFieldMeta) string {
	columns := make([]string, 0, len(fields))
	for _, field := range fields {
		columns = append(columns, sqlQuotedIdentifier(field.Column))
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(columns, ", "), sqlQuotedIdentifier(tableName))
	return fmt.Sprintf("const %sSelectStatement = `%s`", prefix, query)
}

func generateSQLOrderByConst(prefix string, fields []sqlFieldMeta, configured ...string) string {
	orderBy := ""
	if len(configured) > 0 {
		requested := strings.TrimSpace(configured[0])
		for _, field := range fields {
			if strings.EqualFold(requested, field.Field) || strings.EqualFold(requested, field.Column) {
				orderBy = sqlQuotedIdentifier(field.Column)
				break
			}
		}
	}
	for _, field := range fields {
		if orderBy != "" {
			break
		}
		if field.PrimaryKey {
			orderBy = sqlQuotedIdentifier(field.Column)
			break
		}
	}
	if orderBy == "" {
		for _, field := range fields {
			if field.Unique {
				orderBy = sqlQuotedIdentifier(field.Column)
				break
			}
		}
	}
	if orderBy == "" && len(fields) > 0 {
		orderBy = sqlQuotedIdentifier(fields[0].Column)
	}

	return fmt.Sprintf("const %sOrderByColumn = `%s`", prefix, orderBy)
}

func generateSQLCreateStatementsVar(prefix string, tableName string, fields []sqlFieldMeta) string {
	columnDefs := make([]string, 0, len(fields)+1)
	indexStatements := make([]string, 0)
	primaryKeys := make([]sqlFieldMeta, 0)
	for _, field := range fields {
		if field.PrimaryKey {
			primaryKeys = append(primaryKeys, field)
		}
	}

	for _, field := range fields {
		column := sqlQuotedIdentifier(field.Column) + " " + field.SQLType
		if !field.Nullable {
			column += " NOT NULL"
		}
		if field.Default != "" {
			column += " DEFAULT " + field.Default
		}
		if field.PrimaryKey && len(primaryKeys) == 1 {
			column += " PRIMARY KEY"
		} else if field.Unique && !field.PrimaryKey {
			column += " UNIQUE"
		}

		columnDefs = append(columnDefs, column)
		if field.Indexed && !field.Unique && !field.PrimaryKey {
			indexName := "idx_" + tableName + "_" + field.Column
			indexStatements = append(indexStatements, fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s);", sqlQuotedIdentifier(indexName), sqlQuotedIdentifier(tableName), sqlQuotedIdentifier(field.Column)))
		}
	}
	if len(primaryKeys) > 1 {
		columns := make([]string, 0, len(primaryKeys))
		for _, field := range primaryKeys {
			columns = append(columns, sqlQuotedIdentifier(field.Column))
		}
		columnDefs = append(columnDefs, "PRIMARY KEY ("+strings.Join(columns, ", ")+")")
	}

	statements := []string{
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n    %s\n);", sqlQuotedIdentifier(tableName), strings.Join(columnDefs, ",\n    ")),
	}
	statements = append(statements, indexStatements...)

	items := make([]string, 0, len(statements))
	for _, statement := range statements {
		items = append(items, fmt.Sprintf("`%s`", statement))
	}

	return fmt.Sprintf("var %sCreateStatements = []string{\n\t%s,\n}", prefix, strings.Join(items, ",\n\t"))
}

func generateSQLUpdateColumnIndexesVar(prefix string, fields []sqlFieldMeta) string {
	indexes := make([]string, 0, len(fields))
	for index, field := range fields {
		if !field.PrimaryKey {
			indexes = append(indexes, fmt.Sprintf("%d", index))
		}
	}
	return fmt.Sprintf("var %sUpdateColumnIndexes = []int{%s}", prefix, strings.Join(indexes, ", "))
}

func generateSQLInsertStatementConst(prefix string, tableName string, fields []sqlFieldMeta) string {
	columns := make([]string, 0, len(fields))
	placeholders := make([]string, 0, len(fields))
	for i, field := range fields {
		columns = append(columns, sqlQuotedIdentifier(field.Column))
		placeholders = append(placeholders, sqlPlaceholder(field, i+1))
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", sqlQuotedIdentifier(tableName), strings.Join(columns, ", "), strings.Join(placeholders, ", "))
	return fmt.Sprintf("const %sInsertStatement = `%s`", prefix, query)
}

func generateSQLUpsertStatementConst(prefix string, tableName string, fields []sqlFieldMeta) string {
	columns := make([]string, 0, len(fields))
	placeholders := make([]string, 0, len(fields))
	conflictFields := sqlConflictFields(fields)
	conflictColumns := make([]string, 0, len(conflictFields))
	updateColumns := make([]string, 0)
	conflictSet := make(map[string]struct{}, len(conflictFields))
	for _, field := range conflictFields {
		conflictSet[field.Column] = struct{}{}
		conflictColumns = append(conflictColumns, sqlQuotedIdentifier(field.Column))
	}

	for i, field := range fields {
		quotedColumn := sqlQuotedIdentifier(field.Column)
		columns = append(columns, quotedColumn)
		placeholders = append(placeholders, sqlPlaceholder(field, i+1))
		if _, conflict := conflictSet[field.Column]; conflict {
			continue
		}

		updateColumns = append(updateColumns, fmt.Sprintf("%s = EXCLUDED.%s", quotedColumn, quotedColumn))
	}

	if len(conflictColumns) == 0 {
		return ""
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s)", sqlQuotedIdentifier(tableName), strings.Join(columns, ", "), strings.Join(placeholders, ", "), strings.Join(conflictColumns, ", "))
	if len(updateColumns) == 0 {
		query += " DO NOTHING;"
	} else {
		query += " DO UPDATE SET " + strings.Join(updateColumns, ", ") + ";"
	}

	return fmt.Sprintf("const %sUpsertStatement = `%s`", prefix, query)
}

func sqlConflictFields(fields []sqlFieldMeta) []sqlFieldMeta {
	primaryKeys := make([]sqlFieldMeta, 0)
	for _, field := range fields {
		if field.PrimaryKey {
			primaryKeys = append(primaryKeys, field)
		}
	}
	if len(primaryKeys) > 0 {
		return primaryKeys
	}

	for _, field := range fields {
		if field.Unique {
			return []sqlFieldMeta{field}
		}
	}
	return nil
}

func sqlQuotedIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
