package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

type sqlDialect string

const (
	sqlDialectPostgres sqlDialect = "postgres"
	sqlDialectSQLite   sqlDialect = "sqlite"
)

type sqlFieldMeta struct {
	Dialect     sqlDialect
	Field       string
	Column      string
	Type        sdk.TypeRef
	SQLType     string
	Nullable    bool
	Default     string
	Indexed     bool
	PrimaryKey  bool
	Unique      bool
	UpdatedAt   bool
	Searchable  bool
	StorageKind string
}

func generateSQL(req sdk.GenerateRequest) (sdk.GenerateResponse, error) {
	modelImportAlias := "modelpkg"
	modelType := modelImportAlias + "." + req.Model.Name

	modelValues := req.Model.ResolvedValues("sql")
	dialect := resolveSQLDialectValue(modelValues["db"].String())
	tableName := strings.TrimSpace(modelValues["table"].String())
	if tableName == "" {
		tableName = sdk.SnakeCase(req.Model.Name)
	}

	fields := collectSQLFields(req, dialect)
	if len(fields) == 0 {
		return sdk.GenerateResponse{}, nil
	}

	prefix := sdk.LowerCamel(req.Model.Name) + "SQL"
	executorTypeName := "Executor"

	schemaBuilder := sdk.NewCodeBuilder()
	scanBuilder := sdk.NewCodeBuilder()
	queriesBuilder := sdk.NewCodeBuilder()

	addSQLSchemaImports(schemaBuilder)
	addSQLScanImports(scanBuilder, req, fields)
	addSQLQueriesImports(queriesBuilder, req)

	schemaBuilder.AddOrderedBlock("sql.executor.type", generateSQLExecutorType(executorTypeName), 0)
	schemaBuilder.AddOrderedBlock("sql.table", generateSQLTableNameConst(prefix, tableName), 10)
	schemaBuilder.AddOrderedBlock("sql.column.type", generateSQLColumnType(fields), 15)
	schemaBuilder.AddOrderedBlock("sql.column.names", generateSQLColumnNamesVar(prefix, fields), 20)
	schemaBuilder.AddOrderedBlock("sql.search.columns", generateSQLSearchColumnsVar(prefix, fields), 30)
	schemaBuilder.AddOrderedBlock("sql.select.statement", generateSQLSelectStatementConst(prefix, tableName, fields), 40)
	schemaBuilder.AddOrderedBlock("sql.order.by", generateSQLOrderByConst(prefix, fields), 50)
	schemaBuilder.AddOrderedBlock("sql.create.statements", generateSQLCreateStatementsVar(prefix, tableName, fields), 60)
	schemaBuilder.AddOrderedBlock("sql.insert.statement", generateSQLInsertStatementConst(prefix, tableName, fields), 70)
	schemaBuilder.AddOrderedBlock("sql.update.columns", generateSQLUpdateColumnIndexesVar(prefix, fields), 75)

	if upsert := generateSQLUpsertStatementConst(prefix, tableName, fields); upsert != "" {
		schemaBuilder.AddOrderedBlock("sql.upsert.statement", upsert, 80)
		queriesBuilder.AddOrderedBlock("sql.upsert", generateSQLUpsertMethod(modelType, req.Model.Name, prefix, executorTypeName), 70)
	}

	scanBuilder.AddOrderedBlock("sql.driver.values", generateSQLDriverValuesMethod(modelType, req.Model.Name, prefix, fields), 10)
	scanBuilder.AddOrderedBlock("sql.placeholder", generateSQLPlaceholderFunction(prefix, dialect, fields), 20)
	scanBuilder.AddOrderedBlock("sql.column.normalize", generateSQLNormalizeColumnFunction(prefix, fields), 30)
	scanBuilder.AddOrderedBlock("sql.filter.normalize", generateSQLNormalizeFilterValueFunction(prefix, fields), 40)
	scanBuilder.AddOrderedBlock("sql.update.assignments", generateSQLUpdateAssignmentsMethod(modelType, req.Model.Name, prefix), 50)
	scanBuilder.AddOrderedBlock("sql.scan", generateSQLScanMethod(modelType, req.Model.Name, prefix, fields), 60)
	scanBuilder.AddOrderedBlock("sql.updated_at", generateSQLTouchUpdatedAtValuesFunction(prefix, fields), 65)

	if helper := generateSQLHelpers(prefix, dialect, fields); helper != "" {
		scanBuilder.AddOrderedBlock("sql.helpers", helper, 70)
	}

	queriesBuilder.AddOrderedBlock("sql.where", generateSQLBuildWhereFunction(prefix), 10)
	queriesBuilder.AddOrderedBlock("sql.pagination", generateSQLPaginationFunction(prefix, dialect), 15)
	queriesBuilder.AddOrderedBlock("sql.query.many", generateSQLQueryManyFunction(modelType, req.Model.Name, prefix, executorTypeName), 20)
	queriesBuilder.AddOrderedBlock("sql.init", generateSQLInitMethod(req.Model.Name, prefix, executorTypeName), 30)
	queriesBuilder.AddOrderedBlock("sql.create", generateSQLCreateMethod(modelType, req.Model.Name, prefix, executorTypeName), 40)
	queriesBuilder.AddOrderedBlock("sql.get", generateSQLGetMethod(modelType, req.Model.Name, prefix, executorTypeName), 50)
	queriesBuilder.AddOrderedBlock("sql.update", generateSQLUpdateMethod(modelType, req.Model.Name, prefix, executorTypeName), 60)
	queriesBuilder.AddOrderedBlock("sql.delete", generateSQLDeleteMethod(req.Model.Name, prefix, executorTypeName), 80)
	queriesBuilder.AddOrderedBlock("sql.list", generateSQLListMethod(modelType, req.Model.Name, prefix, executorTypeName), 90)
	queriesBuilder.AddOrderedBlock("sql.search", generateSQLSearchMethod(modelType, req.Model.Name, prefix, executorTypeName), 100)
	queriesBuilder.AddOrderedBlock("sql.store", generateSQLStoreType(modelType, req.Model.Name, executorTypeName, generateSQLUpsertStatementConst(prefix, tableName, fields) != ""), 110)

	response := sdk.GenerateResponse{}
	body, err := sdk.RenderGoFile("sql", schemaBuilder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, fmt.Errorf("render SQL schema for %s: %w", req.Model.Name, err)
	}
	response.Files = append(response.Files, sdk.GeneratedFile{
		Path:    "sql/schema.gen.go",
		Content: string(body),
	})

	body, err = sdk.RenderGoFile("sql", scanBuilder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, fmt.Errorf("render SQL scanners for %s: %w", req.Model.Name, err)
	}
	response.Files = append(response.Files, sdk.GeneratedFile{
		Path:    "sql/scan.gen.go",
		Content: string(body),
	})

	body, err = sdk.RenderGoFile("sql", queriesBuilder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, fmt.Errorf("render SQL queries for %s: %w", req.Model.Name, err)
	}
	response.Files = append(response.Files, sdk.GeneratedFile{
		Path:    "sql/queries.gen.go",
		Content: string(body),
	})

	return response, nil
}

func resolveSQLDialectValue(raw string) sqlDialect {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(sqlDialectPostgres), "postgresql":
		return sqlDialectPostgres
	case string(sqlDialectSQLite), "sqlite3":
		return sqlDialectSQLite
	default:
		return sqlDialectPostgres
	}
}

func sqlModelImportPath(file sdk.FileContext) string {
	if strings.TrimSpace(file.GoPackagePath) != "" {
		return strings.TrimSpace(file.GoPackagePath)
	}
	return ".."
}

func addSQLSchemaImports(builder *sdk.CodeBuilder) {
	if builder == nil {
		return
	}
	builder.AddImport("context")
	builder.AddImport("database/sql", "stdsql")
}

func addSQLScanImports(builder *sdk.CodeBuilder, req sdk.GenerateRequest, fields []sqlFieldMeta) {
	if builder == nil {
		return
	}
	builder.AddImport(sqlModelImportPath(req.File), "modelpkg")
	builder.AddImport("fmt")
	builder.AddImport("strings")
	if sqlScanNeedsStdSQL(fields) {
		builder.AddImport("database/sql", "stdsql")
	}
	if sqlNeedsJSON(fields) || sqlNeedsArrayHelpers(fields) {
		builder.AddImport("encoding/json")
	}
	if sqlNeedsStrconv(fields) {
		builder.AddImport("strconv")
	}
	if sqlNeedsTouchNow(fields) {
		builder.AddImport("time")
	}
}

func addSQLQueriesImports(builder *sdk.CodeBuilder, req sdk.GenerateRequest) {
	if builder == nil {
		return
	}
	builder.AddImport(sqlModelImportPath(req.File), "modelpkg")
	builder.AddImport("context")
	builder.AddImport("fmt")
	builder.AddImport("sort")
	builder.AddImport("strings")
}

func sqlWithDoc(code string, primary string, fallback string, args ...any) string {
	return sdk.WithDocComment(code, primary, fallback, args...)
}

func collectSQLFields(req sdk.GenerateRequest, dialect sqlDialect) []sqlFieldMeta {
	fields := make([]sqlFieldMeta, 0)
	for _, field := range req.Model.ActiveFields("sql") {
		if field.Type.IsExternal(req.File) {
			continue
		}

		meta := sqlFieldMeta{
			Dialect:    dialect,
			Field:      field.Name,
			Column:     resolvedSQLColumnName(field),
			Type:       field.Type,
			SQLType:    sqlColumnType(field, req.File, dialect),
			Nullable:   field.Type.Optional,
			Searchable: sqlFieldSearchable(field, req.File),
		}

		values := field.ResolvedValues("sql")
		if value := strings.TrimSpace(values["default"].String()); value != "" {
			meta.Default = sqlDefaultLiteral(value, field)
		}
		meta.Indexed = values["index"].BoolValue()
		meta.PrimaryKey = values["primaryKey"].BoolValue()
		meta.Unique = values["unique"].BoolValue()
		meta.UpdatedAt = values["updatedAt"].BoolValue()

		meta.StorageKind = sqlStorageKind(field, req.File)
		fields = append(fields, meta)
	}

	return fields
}

func sqlScanNeedsStdSQL(fields []sqlFieldMeta) bool {
	for _, field := range fields {
		switch {
		case field.StorageKind == "array_string", field.StorageKind == "array_int", field.StorageKind == "array_uint", field.StorageKind == "array_float", field.StorageKind == "array_bool", field.StorageKind == "json":
			continue
		case field.Type.IsTime():
			return true
		case field.Type.Optional && (field.Type.IsString() || field.Type.IsBool() || field.Type.IsInteger() || field.Type.IsFloat()):
			return true
		}
	}
	return false
}
