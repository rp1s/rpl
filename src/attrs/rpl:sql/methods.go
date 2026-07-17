package main

import (
	"fmt"
	"strings"
)

func generateSQLDriverValuesMethod(modelType string, modelName string, prefix string, fields []sqlFieldMeta) string {
	lines := make([]string, 0, len(fields)*3+4)
	lines = append(lines, "values := make([]any, 0, len("+prefix+"ColumnNames))")

	if sqlNeedsTouchNow(fields) {
		lines = append(lines, "now := time.Now()")
	}

	for _, field := range fields {
		lines = append(lines, sqlDriverValueLines(prefix, field)...)
	}
	lines = append(lines, "return values, nil")

	return sqlWithDoc(
		fmt.Sprintf("func %sDriverValues(model %s) ([]any, error) {\n\t%s\n}", prefix, modelType, strings.Join(lines, "\n\t")),
		"sqlDriverValues подготавливает значения модели %s для SQL-запросов.",
		"sqlDriverValues prepares SQL driver values for model %s.",
		modelName,
	)
}

func generateSQLPlaceholderFunction(prefix string, dialect sqlDialect, fields []sqlFieldMeta) string {
	if dialect == sqlDialectSQLite {
		return sqlWithDoc(
			fmt.Sprintf("func %sPlaceholder(columnName string, index int) string {\n\t_, _ = columnName, index\n\treturn \"?\"\n}", prefix),
			"%sPlaceholder строит SQL placeholder для конкретной колонки.",
			"%sPlaceholder builds the SQL placeholder for a specific column.",
			prefix,
		)
	}

	lines := []string{"column, ok := " + prefix + "NormalizeColumnName(columnName)", "if !ok {\n\t\treturn fmt.Sprintf(\"$%d\", index)\n\t}", "switch column {"}
	for _, field := range fields {
		if fieldNeedsCast(field) {
			lines = append(lines, fmt.Sprintf("case %q:\n\t\treturn fmt.Sprintf(\"$%%d::%s\", index)", sqlQuotedIdentifier(field.Column), field.SQLType))
		}
	}
	lines = append(lines, "default:\n\t\treturn fmt.Sprintf(\"$%d\", index)\n\t}")

	return sqlWithDoc(
		fmt.Sprintf("func %sPlaceholder(columnName string, index int) string {\n\t%s\n}", prefix, strings.Join(lines, "\n\t")),
		"%sPlaceholder строит SQL placeholder для конкретной колонки.",
		"%sPlaceholder builds the SQL placeholder for a specific column.",
		prefix,
	)
}

func generateSQLNormalizeColumnFunction(prefix string, fields []sqlFieldMeta) string {
	lines := []string{"switch strings.TrimSpace(name) {"}
	for _, field := range fields {
		lines = append(lines, fmt.Sprintf("case %q, %q:\n\t\treturn %q, true", field.Field, field.Column, sqlQuotedIdentifier(field.Column)))
	}
	lines = append(lines, "default:\n\t\treturn \"\", false\n\t}")

	return sqlWithDoc(
		fmt.Sprintf("func %sNormalizeColumnName(name string) (string, bool) {\n\t%s\n}", prefix, strings.Join(lines, "\n\t")),
		"%sNormalizeColumnName нормализует имя поля модели в имя SQL-колонки.",
		"%sNormalizeColumnName normalizes a model field name into an SQL column name.",
		prefix,
	)
}

func generateSQLNormalizeFilterValueFunction(prefix string, fields []sqlFieldMeta) string {
	lines := []string{
		"column, ok := " + prefix + "NormalizeColumnName(columnName)",
		"if !ok {\n\t\treturn nil, fmt.Errorf(\"unknown sql filter column %q\", columnName)\n\t}",
		"if value == nil {\n\t\treturn nil, nil\n\t}",
		"switch column {",
	}

	for _, field := range fields {
		column := sqlQuotedIdentifier(field.Column)
		switch field.StorageKind {
		case "array_string":
			lines = append(lines, fmt.Sprintf("case %q:\n\t\ttyped, ok := value.([]string)\n\t\tif !ok {\n\t\t\treturn nil, fmt.Errorf(\"sql filter %%q expects []string\", %q)\n\t\t}\n\t\treturn %sEncodeStringArray(typed), nil", column, field.Column, prefix))
		case "array_int":
			lines = append(lines, fmt.Sprintf("case %q:\n\t\tswitch typed := value.(type) {\n\t\tcase []int:\n\t\t\treturn %sEncodeIntArray(typed), nil\n\t\tcase []int64:\n\t\t\titems := make([]int, 0, len(typed))\n\t\t\tfor _, item := range typed {\n\t\t\t\titems = append(items, int(item))\n\t\t\t}\n\t\t\treturn %sEncodeIntArray(items), nil\n\t\tdefault:\n\t\t\treturn nil, fmt.Errorf(\"sql filter %%q expects []int\", %q)\n\t\t}", column, prefix, prefix, field.Column))
		case "array_uint":
			lines = append(lines, fmt.Sprintf("case %q:\n\t\ttyped, ok := value.([]uint64)\n\t\tif !ok {\n\t\t\treturn nil, fmt.Errorf(\"sql filter %%q expects []uint64\", %q)\n\t\t}\n\t\treturn %sEncodeUintArray(typed), nil", column, field.Column, prefix))
		case "array_float":
			lines = append(lines, fmt.Sprintf("case %q:\n\t\ttyped, ok := value.([]float64)\n\t\tif !ok {\n\t\t\treturn nil, fmt.Errorf(\"sql filter %%q expects []float64\", %q)\n\t\t}\n\t\treturn %sEncodeFloatArray(typed), nil", column, field.Column, prefix))
		case "array_bool":
			lines = append(lines, fmt.Sprintf("case %q:\n\t\ttyped, ok := value.([]bool)\n\t\tif !ok {\n\t\t\treturn nil, fmt.Errorf(\"sql filter %%q expects []bool\", %q)\n\t\t}\n\t\treturn %sEncodeBoolArray(typed), nil", column, field.Column, prefix))
		case "json":
			lines = append(lines, fmt.Sprintf("case %q:\n\t\treturn %sMarshalJSON(value)", column, prefix))
		}
	}

	lines = append(lines, "default:\n\t\treturn value, nil\n\t}")
	return sqlWithDoc(
		fmt.Sprintf("func %sNormalizeFilterValue(columnName string, value any) (any, error) {\n\t%s\n}", prefix, strings.Join(lines, "\n\t")),
		"%sNormalizeFilterValue приводит значение фильтра к SQL-совместимому виду.",
		"%sNormalizeFilterValue converts a filter value into an SQL-compatible form.",
		prefix,
	)
}

func generateSQLBuildWhereFunction(prefix string) string {
	return sqlWithDoc(fmt.Sprintf(`func %sBuildWhere(filters map[string]any, startIndex int) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}

	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))

	for _, key := range keys {
		column, ok := %sNormalizeColumnName(key)
		if !ok {
			return "", nil, fmt.Errorf("unknown sql filter column %%q", key)
		}

		value, err := %sNormalizeFilterValue(key, filters[key])
		if err != nil {
			return "", nil, err
		}

		if value == nil {
			parts = append(parts, column+" IS NULL")
			continue
		}

		args = append(args, value)
		parts = append(parts, fmt.Sprintf("%%s = %%s", column, %sPlaceholder(column, startIndex+len(args)-1)))
	}

	return " WHERE " + strings.Join(parts, " AND "), args, nil
}`, prefix, prefix, prefix, prefix),
		"%sBuildWhere собирает SQL WHERE-часть и аргументы из карты фильтров.",
		"%sBuildWhere builds the SQL WHERE clause and args from the filter map.",
		prefix,
	)
}

func generateSQLPaginationFunction(prefix string, dialect sqlDialect) string {
	sqliteWithoutLimit := ""
	if dialect == sqlDialectSQLite {
		sqliteWithoutLimit = `
	if offset > 0 && limit <= 0 {
		query += " LIMIT -1"
	}`
	}

	return sqlWithDoc(fmt.Sprintf(`func %sAppendPagination(query string, args []any, limit int, offset int) (string, []any) {
	if limit > 0 {
		args = append(args, limit)
		query += " LIMIT " + %sPlaceholder("", len(args))
	}%s
	if offset > 0 {
		args = append(args, offset)
		query += " OFFSET " + %sPlaceholder("", len(args))
	}
	return query, args
}`, prefix, prefix, sqliteWithoutLimit, prefix),
		"%sAppendPagination безопасно добавляет LIMIT/OFFSET для выбранного SQL-диалекта.",
		"%sAppendPagination safely appends LIMIT/OFFSET for the selected SQL dialect.",
		prefix,
	)
}

func generateSQLUpdateAssignmentsMethod(modelType string, modelName string, prefix string) string {
	return sqlWithDoc(fmt.Sprintf(`func %sBuildUpdateAssignments(model %s, startIndex int) (string, []any, error) {
	values, err := %sDriverValues(model)
	if err != nil {
		return "", nil, err
	}
	%sTouchUpdatedAt(values)
	if len(%sUpdateColumnIndexes) == 0 {
		return "", nil, fmt.Errorf("model %s has no mutable SQL columns")
	}

	columns := %sColumnNames
	parts := make([]string, 0, len(%sUpdateColumnIndexes))
	args := make([]any, 0, len(%sUpdateColumnIndexes))

	for _, index := range %sUpdateColumnIndexes {
		column := columns[index]
		parts = append(parts, fmt.Sprintf("%%s = %%s", column, %sPlaceholder(column, startIndex+len(args))))
		args = append(args, values[index])
	}

	return strings.Join(parts, ", "), args, nil
}`, prefix, modelType, prefix, prefix, prefix, modelName, prefix, prefix, prefix, prefix, prefix),
		"sqlBuildUpdateAssignments подготавливает SET-часть UPDATE для модели %s.",
		"sqlBuildUpdateAssignments prepares the SQL UPDATE assignments for model %s.",
		modelName,
	)
}

func generateSQLTouchUpdatedAtValuesFunction(prefix string, fields []sqlFieldMeta) string {
	indexes := make([]string, 0)
	for index, field := range fields {
		if field.UpdatedAt {
			indexes = append(indexes, fmt.Sprintf("%d", index))
		}
	}
	if len(indexes) == 0 {
		return fmt.Sprintf("func %sTouchUpdatedAt(values []any) {\n\t_ = values\n}", prefix)
	}

	return fmt.Sprintf(`func %sTouchUpdatedAt(values []any) {
	now := time.Now()
	for _, index := range []int{%s} {
		if index >= 0 && index < len(values) {
			values[index] = now
		}
	}
}`, prefix, strings.Join(indexes, ", "))
}

func generateSQLScanMethod(modelType string, modelName string, prefix string, fields []sqlFieldMeta) string {
	lines := make([]string, 0, len(fields)*4+4)
	for _, field := range fields {
		lines = append(lines, sqlScanVarDecl(field)...)
	}

	destinations := make([]string, 0, len(fields))
	for _, field := range fields {
		destinations = append(destinations, sqlScanVarRef(field))
	}
	lines = append(lines, "if err := scanner.Scan("+strings.Join(destinations, ", ")+"); err != nil {\n\t\treturn err\n\t}")

	for _, field := range fields {
		lines = append(lines, sqlAssignScanLines(prefix, field)...)
	}
	lines = append(lines, "return nil")

	return sqlWithDoc(
		fmt.Sprintf("func %sScan(scanner interface{ Scan(dest ...any) error }, model *%s) error {\n\t%s\n}", prefix, modelType, strings.Join(lines, "\n\t")),
		"sqlScan заполняет модель %s данными из SQL scanner.",
		"sqlScan fills model %s from an SQL scanner.",
		modelName,
	)
}

func generateSQLQueryManyFunction(modelType string, modelName string, prefix string, executorType string) string {
	return sqlWithDoc(fmt.Sprintf(`func %sQueryMany(ctx context.Context, db %s, query string, args ...any) ([]%s, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]%s, 0)
	for rows.Next() {
		var item %s
		if err := %sScan(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}`, prefix, executorType, modelType, modelType, modelType, prefix),
		"%sQueryMany выполняет SQL-запрос и собирает список моделей %s.",
		"%sQueryMany executes the SQL query and collects a list of %s models.",
		prefix,
		modelName,
	)
}

func generateSQLInitMethod(modelName string, prefix string, executorType string) string {
	return sqlWithDoc(fmt.Sprintf(`func Init(ctx context.Context, db %s) error {
	for _, statement := range %sCreateStatements {
		if strings.TrimSpace(statement) == "" {
			continue
		}

		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}`, executorType, prefix),
		"SQLInit создает таблицу и вспомогательные индексы для модели %s.",
		"SQLInit creates the table and supporting indexes for model %s.",
		modelName,
	)
}

func generateSQLCreateMethod(modelType string, modelName string, prefix string, executorType string) string {
	return sqlWithDoc(fmt.Sprintf(`func Create(ctx context.Context, db %s, model %s) error {
	values, err := %sDriverValues(model)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, %sInsertStatement, values...)
	return err
}`, executorType, modelType, prefix, prefix),
		"SQLCreate вставляет текущую модель %s в базу данных.",
		"SQLCreate inserts the current %s model into the database.",
		modelName,
	)
}

func generateSQLGetMethod(modelType string, modelName string, prefix string, executorType string) string {
	return sqlWithDoc(fmt.Sprintf(`func Get(ctx context.Context, db %s, filters map[string]any) (%s, error) {
	if len(filters) == 0 {
		return %s{}, fmt.Errorf("SQLGet requires at least one filter")
	}

	where, args, err := %sBuildWhere(filters, 1)
	if err != nil {
		return %s{}, err
	}

	query := %sSelectStatement + where + " LIMIT 1"
	row := db.QueryRowContext(ctx, query, args...)
	var item %s
	if err := %sScan(row, &item); err != nil {
		return %s{}, err
	}

	return item, nil
}`, executorType, modelType, modelType, prefix, modelType, prefix, modelType, prefix, modelType),
		"SQLGet находит одну модель %s по фильтрам.",
		"SQLGet finds one %s model by filters.",
		modelName,
	)
}

func generateSQLUpdateMethod(modelType string, modelName string, prefix string, executorType string) string {
	return sqlWithDoc(fmt.Sprintf(`func Update(ctx context.Context, db %s, model %s, filters map[string]any) error {
	if len(filters) == 0 {
		return fmt.Errorf("SQLUpdate requires at least one filter")
	}

	assignments, args, err := %sBuildUpdateAssignments(model, 1)
	if err != nil {
		return err
	}

	where, whereArgs, err := %sBuildWhere(filters, len(args)+1)
	if err != nil {
		return err
	}

	query := "UPDATE " + %sTableName + " SET " + assignments + where
	_, err = db.ExecContext(ctx, query, append(args, whereArgs...)...)
	return err
}`, executorType, modelType, prefix, prefix, prefix),
		"SQLUpdate обновляет запись модели %s по переданным фильтрам.",
		"SQLUpdate updates the %s record using the provided filters.",
		modelName,
	)
}

func generateSQLUpsertMethod(modelType string, modelName string, prefix string, executorType string) string {
	return sqlWithDoc(fmt.Sprintf(`func Upsert(ctx context.Context, db %s, model %s) error {
	statement := %sUpsertStatement
	if strings.TrimSpace(statement) == "" {
		return fmt.Errorf("SQLUpsertStatement is empty because the model has no unique columns")
	}

	values, err := %sDriverValues(model)
	if err != nil {
		return err
	}
	%sTouchUpdatedAt(values)

	_, err = db.ExecContext(ctx, statement, values...)
	return err
}`, executorType, modelType, prefix, prefix, prefix),
		"SQLUpsert создает или обновляет модель %s по уникальным колонкам.",
		"SQLUpsert creates or updates model %s using its unique columns.",
		modelName,
	)
}

func generateSQLDeleteMethod(modelName string, prefix string, executorType string) string {
	return sqlWithDoc(fmt.Sprintf(`func Delete(ctx context.Context, db %s, filters map[string]any) error {
	if len(filters) == 0 {
		return fmt.Errorf("SQLDelete requires at least one filter")
	}

	where, args, err := %sBuildWhere(filters, 1)
	if err != nil {
		return err
	}

	query := "DELETE FROM " + %sTableName + where
	_, err = db.ExecContext(ctx, query, args...)
	return err
}`, executorType, prefix, prefix),
		"SQLDelete удаляет записи модели %s по фильтрам.",
		"SQLDelete deletes %s records by filters.",
		modelName,
	)
}

func generateSQLListMethod(modelType string, modelName string, prefix string, executorType string) string {
	return sqlWithDoc(fmt.Sprintf(`func List(ctx context.Context, db %s, limit int, offset int) ([]%s, error) {
	query := %sSelectStatement
	if orderBy := %sOrderByColumn; orderBy != "" {
		query += " ORDER BY " + orderBy
	}

	query, args := %sAppendPagination(query, nil, limit, offset)

	return %sQueryMany(ctx, db, query, args...)
}`, executorType, modelType, prefix, prefix, prefix, prefix),
		"SQLList возвращает список моделей %s с пагинацией.",
		"SQLList returns a paginated list of %s models.",
		modelName,
	)
}

func generateSQLSearchMethod(modelType string, modelName string, prefix string, executorType string) string {
	return sqlWithDoc(fmt.Sprintf(`func Search(ctx context.Context, db %s, term string, limit int, offset int) ([]%s, error) {
	trimmed := strings.TrimSpace(term)
	if trimmed == "" || len(%sSearchColumns) == 0 {
		return List(ctx, db, limit, offset)
	}

	pattern := "%%" + trimmed + "%%"
	parts := make([]string, 0, len(%sSearchColumns))
	args := make([]any, 0, len(%sSearchColumns)+2)
	for _, column := range %sSearchColumns {
		args = append(args, pattern)
		parts = append(parts, fmt.Sprintf("LOWER(%%s) LIKE LOWER(%%s)", column, %sPlaceholder(column, len(args))))
	}

	query := %sSelectStatement + " WHERE (" + strings.Join(parts, " OR ") + ")"
	if orderBy := %sOrderByColumn; orderBy != "" {
		query += " ORDER BY " + orderBy
	}
	query, args = %sAppendPagination(query, args, limit, offset)

	return %sQueryMany(ctx, db, query, args...)
}`, executorType, modelType, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix),
		"SQLSearch ищет модели %s по текстовому запросу.",
		"SQLSearch searches %s models by a text query.",
		modelName,
	)
}

func generateSQLStoreType(modelType string, modelName string, executorType string, hasUpsert bool) string {
	parts := []string{
		sqlWithDoc(fmt.Sprintf(`type Store struct {
	db %s
}`, executorType),
			"Store хранит SQL executor для операций модели %s.",
			"Store holds the SQL executor used for %s operations.",
			modelName,
		),
		sqlWithDoc(fmt.Sprintf(`func NewStore(db %s) *Store {
	return &Store{db: db}
}`, executorType),
			"NewStore создает SQL store для модели %s.",
			"NewStore creates an SQL store for %s.",
			modelName,
		),
		`func (store *Store) executor() (Executor, error) {
	if store == nil || store.db == nil {
		return nil, fmt.Errorf("sql store executor is nil")
	}
	return store.db, nil
}`,
		`func (store *Store) DB() Executor {
	if store == nil {
		return nil
	}
	return store.db
}`,
		`func (store *Store) WithExecutor(db Executor) *Store {
	return NewStore(db)
}`,
		`func (store *Store) Init(ctx context.Context) error {
	db, err := store.executor()
	if err != nil {
		return err
	}
	return Init(ctx, db)
}`,
		fmt.Sprintf(`func (store *Store) Create(ctx context.Context, model %s) error {
	db, err := store.executor()
	if err != nil {
		return err
	}
	return Create(ctx, db, model)
}`, modelType),
		fmt.Sprintf(`func (store *Store) Get(ctx context.Context, filters map[string]any) (%s, error) {
	db, err := store.executor()
	if err != nil {
		return %s{}, err
	}
	return Get(ctx, db, filters)
}`, modelType, modelType),
		fmt.Sprintf(`func (store *Store) Update(ctx context.Context, model %s, filters map[string]any) error {
	db, err := store.executor()
	if err != nil {
		return err
	}
	return Update(ctx, db, model, filters)
}`, modelType),
		`func (store *Store) Delete(ctx context.Context, filters map[string]any) error {
	db, err := store.executor()
	if err != nil {
		return err
	}
	return Delete(ctx, db, filters)
}`,
		fmt.Sprintf(`func (store *Store) List(ctx context.Context, limit int, offset int) ([]%s, error) {
	db, err := store.executor()
	if err != nil {
		return nil, err
	}
	return List(ctx, db, limit, offset)
}`, modelType),
		fmt.Sprintf(`func (store *Store) Search(ctx context.Context, term string, limit int, offset int) ([]%s, error) {
	db, err := store.executor()
	if err != nil {
		return nil, err
	}
	return Search(ctx, db, term, limit, offset)
}`, modelType),
	}

	if hasUpsert {
		parts = append(parts, fmt.Sprintf(`func (store *Store) Upsert(ctx context.Context, model %s) error {
	db, err := store.executor()
	if err != nil {
		return err
	}
	return Upsert(ctx, db, model)
}`, modelType))
	}

	return strings.Join(parts, "\n\n")
}
