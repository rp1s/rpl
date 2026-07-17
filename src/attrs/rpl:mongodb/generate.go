package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strconv"
	"strings"
)

type mongoFieldMeta struct {
	FieldName  string
	Type       sdk.TypeRef
	BSONName   string
	Index      bool
	IndexGroup string
	IndexOrder int
	Unique     bool
	Sparse     bool
	Search     bool
	Sort       bool
	ObjectID   bool
	OmitEmpty  bool
	Default    string
	UpdatedAt  bool
	IsIDLike   bool
}

func generateMongoDB(req sdk.GenerateRequest) (sdk.GenerateResponse, error) {
	modelValues := req.Model.ResolvedValues("mongodb")
	dbName := strings.TrimSpace(modelValues["db"].String())
	if dbName == "" {
		dbName = "mongodb"
	}

	collectionName := strings.TrimSpace(modelValues["collection"].String())
	if collectionName == "" {
		collectionName = sdk.SnakeCase(req.Model.Name)
	}

	fields := collectMongoFields(req)
	if len(fields) == 0 {
		return sdk.GenerateResponse{}, nil
	}

	prefix := sdk.LowerCamel(req.Model.Name) + "Mongo"
	modelType := "modelpkg." + req.Model.Name
	docType := prefix + "Document"

	schemaBuilder := sdk.NewCodeBuilder()
	bsonBuilder := sdk.NewCodeBuilder()
	queriesBuilder := sdk.NewCodeBuilder()

	addMongoSchemaImports(schemaBuilder)
	addMongoBSONImports(bsonBuilder, req, fields)
	addMongoQueriesImports(queriesBuilder, req)

	schemaBuilder.AddOrderedBlock("mongo.const.db", fmt.Sprintf("const %sDatabaseName = %q", prefix, dbName), 0)
	schemaBuilder.AddOrderedBlock("mongo.const.collection", fmt.Sprintf("const %sCollectionName = %q", prefix, collectionName), 10)
	schemaBuilder.AddOrderedBlock("mongo.field.names", generateMongoFieldNamesVar(prefix, fields), 20)
	schemaBuilder.AddOrderedBlock("mongo.search.fields", generateMongoSearchFieldsVar(prefix, fields), 30)
	schemaBuilder.AddOrderedBlock("mongo.sort.fields", generateMongoSortFieldsVar(prefix, fields), 40)
	schemaBuilder.AddOrderedBlock("mongo.collection", generateMongoCollectionHelpers(prefix), 50)
	schemaBuilder.AddOrderedBlock("mongo.indexes", generateMongoIndexesHelpers(prefix, collectionName, fields), 60)

	bsonBuilder.AddOrderedBlock("mongo.doc.type", generateMongoDocumentType(docType, req.File, fields), 0)
	bsonBuilder.AddOrderedBlock("mongo.wrap", generateMongoWrapFunction(prefix, docType, modelType, fields), 10)
	bsonBuilder.AddOrderedBlock("mongo.unwrap", generateMongoUnwrapFunction(prefix, docType, modelType, fields), 20)
	bsonBuilder.AddOrderedBlock("mongo.document", generateMongoDocumentHelpers(prefix, docType, modelType, fields), 30)
	bsonBuilder.AddOrderedBlock("mongo.normalize", generateMongoNormalizeHelpers(prefix, fields), 40)
	bsonBuilder.AddOrderedBlock("mongo.search.sort", generateMongoSearchAndSortHelpers(prefix, fields), 50)

	queriesBuilder.AddOrderedBlock("mongo.insert", generateMongoInsertHelpers(prefix, modelType), 0)
	queriesBuilder.AddOrderedBlock("mongo.find", generateMongoFindHelpers(prefix, modelType, fields), 10)
	queriesBuilder.AddOrderedBlock("mongo.count", generateMongoCountHelpers(prefix), 20)
	queriesBuilder.AddOrderedBlock("mongo.replace", generateMongoReplaceHelpers(prefix, modelType, fields), 30)
	queriesBuilder.AddOrderedBlock("mongo.update", generateMongoUpdateHelpers(prefix, modelType, fields), 40)
	queriesBuilder.AddOrderedBlock("mongo.delete", generateMongoDeleteHelpers(prefix, fields), 50)
	queriesBuilder.AddOrderedBlock("mongo.extra", generateMongoExtraHelpers(prefix, modelType), 60)

	response := sdk.GenerateResponse{}
	body, err := sdk.RenderGoFile("mongodb", schemaBuilder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, fmt.Errorf("render MongoDB schema for %s: %w", req.Model.Name, err)
	}
	response.Files = append(response.Files, sdk.GeneratedFile{
		Path:    "mongodb/schema.gen.go",
		Content: string(body),
	})

	body, err = sdk.RenderGoFile("mongodb", bsonBuilder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, fmt.Errorf("render MongoDB BSON adapters for %s: %w", req.Model.Name, err)
	}
	response.Files = append(response.Files, sdk.GeneratedFile{
		Path:    "mongodb/bson.gen.go",
		Content: string(body),
	})

	body, err = sdk.RenderGoFile("mongodb", queriesBuilder.Response())
	if err != nil {
		return sdk.GenerateResponse{}, fmt.Errorf("render MongoDB queries for %s: %w", req.Model.Name, err)
	}
	response.Files = append(response.Files, sdk.GeneratedFile{
		Path:    "mongodb/queries.gen.go",
		Content: string(body),
	})

	return response, nil
}

func collectMongoFields(req sdk.GenerateRequest) []mongoFieldMeta {
	fields := make([]mongoFieldMeta, 0)
	for _, field := range req.Model.ActiveFields("mongodb") {
		if field.Type.IsExternal(req.File) {
			continue
		}

		values := field.ResolvedValues("mongodb")
		bsonName := strings.TrimSpace(values["name"].String())
		if bsonName == "" {
			bsonName = sdk.SnakeCase(field.Name)
		}
		if values["objectId"].BoolValue() && strings.EqualFold(field.Name, "id") {
			bsonName = "_id"
		}

		fields = append(fields, mongoFieldMeta{
			FieldName:  field.Name,
			Type:       field.Type,
			BSONName:   bsonName,
			Index:      values["index"].BoolValue(),
			IndexGroup: strings.TrimSpace(values["indexGroup"].String()),
			IndexOrder: mongoIndexOrder(values["indexOrder"]),
			Unique:     values["unique"].BoolValue(),
			Sparse:     values["sparse"].BoolValue(),
			Search:     values["search"].BoolValue(),
			Sort:       values["sort"].BoolValue(),
			ObjectID:   values["objectId"].BoolValue(),
			OmitEmpty:  values["omitempty"].BoolValue(),
			Default:    strings.TrimSpace(values["default"].String()),
			UpdatedAt:  values["updatedAt"].BoolValue(),
			IsIDLike:   strings.EqualFold(field.Name, "id") || strings.HasSuffix(strings.ToLower(field.Name), "id"),
		})
	}

	if !hasMongoSearchFields(fields) {
		for index := range fields {
			if fields[index].Type.IsString() && !fields[index].ObjectID {
				fields[index].Search = true
			}
		}
	}
	if !hasMongoSortFields(fields) {
		for index := range fields {
			if !fields[index].Type.IsList {
				fields[index].Sort = true
			}
		}
	}

	return fields
}

func mongoIndexOrder(value sdk.Value) int {
	order, err := value.Int64()
	if err == nil && order == -1 {
		return -1
	}
	return 1
}

func addMongoSchemaImports(builder *sdk.CodeBuilder) {
	if builder == nil {
		return
	}
	builder.AddImport("context")
	builder.AddImport("go.mongodb.org/mongo-driver/bson")
	builder.AddImport("go.mongodb.org/mongo-driver/mongo")
	builder.AddImport("go.mongodb.org/mongo-driver/mongo/options")
}

func addMongoBSONImports(builder *sdk.CodeBuilder, req sdk.GenerateRequest, fields []mongoFieldMeta) {
	if builder == nil {
		return
	}
	builder.AddImport(mongoModelImportPath(req.File), "modelpkg")
	builder.AddImport("context")
	builder.AddImport("fmt")
	builder.AddImport("sort")
	builder.AddImport("strings")
	builder.AddImport("go.mongodb.org/mongo-driver/bson")
	builder.AddImport("go.mongodb.org/mongo-driver/mongo")

	if mongoNeedsPrimitive(fields) {
		builder.AddImport("go.mongodb.org/mongo-driver/bson/primitive")
	}
	if mongoNeedsTime(fields) {
		builder.AddImport("time")
	}
}

func addMongoQueriesImports(builder *sdk.CodeBuilder, req sdk.GenerateRequest) {
	if builder == nil {
		return
	}
	builder.AddImport(mongoModelImportPath(req.File), "modelpkg")
	builder.AddImport("context")
	builder.AddImport("go.mongodb.org/mongo-driver/bson")
	builder.AddImport("go.mongodb.org/mongo-driver/mongo")
	builder.AddImport("go.mongodb.org/mongo-driver/mongo/options")
}

func mongoModelImportPath(file sdk.FileContext) string {
	if strings.TrimSpace(file.GoPackagePath) != "" {
		return strings.TrimSpace(file.GoPackagePath)
	}
	return ".."
}

func hasMongoSearchFields(fields []mongoFieldMeta) bool {
	for _, field := range fields {
		if field.Search {
			return true
		}
	}
	return false
}

func hasMongoSortFields(fields []mongoFieldMeta) bool {
	for _, field := range fields {
		if field.Sort {
			return true
		}
	}
	return false
}

func mongoNeedsPrimitive(fields []mongoFieldMeta) bool {
	for _, field := range fields {
		if field.ObjectID {
			return true
		}
	}
	return false
}

func mongoNeedsTime(fields []mongoFieldMeta) bool {
	for _, field := range fields {
		if field.UpdatedAt || strings.EqualFold(field.Default, "now") {
			return true
		}
	}
	return false
}

func generateMongoFieldNamesVar(prefix string, fields []mongoFieldMeta) string {
	items := make([]string, 0, len(fields))
	for _, field := range fields {
		items = append(items, strconv.Quote(field.BSONName))
	}
	return fmt.Sprintf("var %sFieldNames = []string{%s}", prefix, strings.Join(items, ", "))
}

func generateMongoSearchFieldsVar(prefix string, fields []mongoFieldMeta) string {
	items := make([]string, 0)
	for _, field := range fields {
		if field.Search {
			items = append(items, strconv.Quote(field.BSONName))
		}
	}
	return fmt.Sprintf("var %sSearchFields = []string{%s}", prefix, strings.Join(items, ", "))
}

func generateMongoSortFieldsVar(prefix string, fields []mongoFieldMeta) string {
	items := make([]string, 0)
	for _, field := range fields {
		if field.Sort {
			items = append(items, strconv.Quote(field.BSONName))
		}
	}
	return fmt.Sprintf("var %sSortFields = []string{%s}", prefix, strings.Join(items, ", "))
}

func generateMongoCollectionHelpers(prefix string) string {
	return strings.Join([]string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sCollection(db *mongo.Database) *mongo.Collection {\n\treturn db.Collection(%sCollectionName)\n}", prefix, prefix),
			"%sCollection returns the typed MongoDB collection handle.",
			"%sCollection returns the typed MongoDB collection handle.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sDatabase() string {\n\treturn %sDatabaseName\n}", prefix, prefix),
			"%sDatabase returns the configured MongoDB database name.",
			"%sDatabase returns the configured MongoDB database name.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sCollectionLabel() string {\n\treturn %sCollectionName\n}", prefix, prefix),
			"%sCollectionLabel returns the configured MongoDB collection name.",
			"%sCollectionLabel returns the configured MongoDB collection name.",
			prefix,
		),
	}, "\n\n")
}

func generateMongoIndexesHelpers(prefix string, collectionName string, fields []mongoFieldMeta) string {
	indexLines := make([]string, 0)
	type compoundIndex struct {
		name   string
		keys   []string
		unique bool
		sparse bool
	}
	compound := make([]compoundIndex, 0)
	compoundIndexes := make(map[string]int)
	for _, field := range fields {
		if field.IndexGroup != "" {
			index, ok := compoundIndexes[field.IndexGroup]
			if !ok {
				index = len(compound)
				compoundIndexes[field.IndexGroup] = index
				compound = append(compound, compoundIndex{name: field.IndexGroup})
			}
			compound[index].keys = append(compound[index].keys, fmt.Sprintf("{Key: %q, Value: %d}", field.BSONName, field.IndexOrder))
			compound[index].unique = compound[index].unique || field.Unique
			compound[index].sparse = compound[index].sparse || field.Sparse
			continue
		}
		if !field.Index && !field.Unique {
			continue
		}

		optionsExpr := fmt.Sprintf("options.Index().SetName(%q)", collectionName+"_"+field.BSONName+"_idx")
		if field.Unique {
			optionsExpr += ".SetUnique(true)"
		}
		if field.Sparse {
			optionsExpr += ".SetSparse(true)"
		}
		indexLines = append(indexLines, fmt.Sprintf("{Keys: bson.D{{Key: %q, Value: 1}}, Options: %s}", field.BSONName, optionsExpr))
	}
	for _, index := range compound {
		optionsExpr := fmt.Sprintf("options.Index().SetName(%q)", collectionName+"_"+index.name+"_idx")
		if index.unique {
			optionsExpr += ".SetUnique(true)"
		}
		if index.sparse {
			optionsExpr += ".SetSparse(true)"
		}
		indexLines = append(indexLines, fmt.Sprintf("{Keys: bson.D{%s}, Options: %s}", strings.Join(index.keys, ", "), optionsExpr))
	}

	searchFields := mongoSearchFields(fields)
	if len(searchFields) > 0 {
		items := make([]string, 0, len(searchFields))
		for _, field := range searchFields {
			items = append(items, fmt.Sprintf("{Key: %q, Value: \"text\"}", field.BSONName))
		}
		indexLines = append(indexLines, fmt.Sprintf("{Keys: bson.D{%s}, Options: options.Index().SetName(%q)}", strings.Join(items, ", "), collectionName+"_search_text"))
	}
	indexBody := strings.Join(indexLines, ",\n\t\t")
	if indexBody != "" {
		indexBody += ","
	}

	return strings.Join([]string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sIndexes() []mongo.IndexModel {\n\treturn []mongo.IndexModel{\n\t\t%s\n\t}\n}", prefix, indexBody),
			"%sIndexes returns all generated MongoDB indexes for the model.",
			"%sIndexes returns all generated MongoDB indexes for the model.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sEnsureIndexes(ctx context.Context, db *mongo.Database) error {\n\tindexes := %sIndexes()\n\tif len(indexes) == 0 {\n\t\treturn nil\n\t}\n\t_, err := %sCollection(db).Indexes().CreateMany(ctx, indexes)\n\treturn err\n}", prefix, prefix, prefix),
			"%sEnsureIndexes creates all generated MongoDB indexes.",
			"%sEnsureIndexes creates all generated MongoDB indexes.",
			prefix,
		),
	}, "\n\n")
}

func generateMongoDocumentType(docType string, file sdk.FileContext, fields []mongoFieldMeta) string {
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		tag := field.BSONName
		if field.OmitEmpty || field.ObjectID {
			tag += ",omitempty"
		}
		lines = append(lines, fmt.Sprintf("\t%s %s `bson:%q`", field.FieldName, mongoWrapperType(field, file), tag))
	}
	return fmt.Sprintf("type %s struct {\n%s\n}", docType, strings.Join(lines, "\n"))
}

func generateMongoWrapFunction(prefix string, docType string, modelType string, fields []mongoFieldMeta) string {
	lines := []string{
		fmt.Sprintf("func %sWrap(model %s, includeDefaults bool, touchUpdatedAt bool) (%s, error) {", prefix, modelType, docType),
		fmt.Sprintf("\tdoc := %s{}", docType),
	}
	for _, field := range fields {
		lines = append(lines, indentBlock(mongoWrapFieldCode(field), 1)...)
	}
	lines = append(lines, "\treturn doc, nil", "}")
	return strings.Join(lines, "\n")
}

func generateMongoUnwrapFunction(prefix string, docType string, modelType string, fields []mongoFieldMeta) string {
	lines := []string{
		fmt.Sprintf("func %sUnwrap(doc %s) %s {", prefix, docType, modelType),
		fmt.Sprintf("\tmodel := %s{}", modelType),
	}
	for _, field := range fields {
		lines = append(lines, indentBlock(mongoUnwrapFieldCode(field), 1)...)
	}
	lines = append(lines, "\treturn model", "}")
	return strings.Join(lines, "\n")
}

func generateMongoDocumentHelpers(prefix string, docType string, modelType string, fields []mongoFieldMeta) string {
	deleteLines := make([]string, 0)
	for _, field := range fields {
		if field.ObjectID {
			deleteLines = append(deleteLines, fmt.Sprintf("delete(document, %q)", field.BSONName))
		}
	}

	parts := []string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sDocumentMap(model %s) (bson.M, error) {\n\twrapped, err := %sWrap(model, true, true)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tbody, err := bson.Marshal(wrapped)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tvar document bson.M\n\tif err := bson.Unmarshal(body, &document); err != nil {\n\t\treturn nil, err\n\t}\n\treturn document, nil\n}", prefix, modelType, prefix),
			"%sDocumentMap converts a model into a MongoDB document.",
			"%sDocumentMap converts a model into a MongoDB document.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sUpdateDocument(model %s) (bson.M, error) {\n\twrapped, err := %sWrap(model, false, true)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tbody, err := bson.Marshal(wrapped)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tvar document bson.M\n\tif err := bson.Unmarshal(body, &document); err != nil {\n\t\treturn nil, err\n\t}\n\t%s\n\treturn document, nil\n}", prefix, modelType, prefix, strings.Join(deleteLines, "\n\t")),
			"%sUpdateDocument builds a `$set`-ready document for updates.",
			"%sUpdateDocument builds a `$set`-ready document for updates.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sDocuments(models []%s) ([]any, error) {\n\titems := make([]any, 0, len(models))\n\tfor _, model := range models {\n\t\tdocument, err := %sDocumentMap(model)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\titems = append(items, document)\n\t}\n\treturn items, nil\n}", prefix, modelType, prefix),
			"%sDocuments converts a model slice into MongoDB insert payloads.",
			"%sDocuments converts a model slice into MongoDB insert payloads.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sFromDocument(document bson.M) (%s, error) {\n\tbody, err := bson.Marshal(document)\n\tif err != nil {\n\t\treturn %s{}, err\n\t}\n\tvar wrapped %s\n\tif err := bson.Unmarshal(body, &wrapped); err != nil {\n\t\treturn %s{}, err\n\t}\n\treturn %sUnwrap(wrapped), nil\n}", prefix, modelType, modelType, docType, modelType, prefix),
			"%sFromDocument decodes a raw MongoDB document into the model.",
			"%sFromDocument decodes a raw MongoDB document into the model.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sDecodeOne(result *mongo.SingleResult) (%s, error) {\n\tvar wrapped %s\n\tif err := result.Decode(&wrapped); err != nil {\n\t\treturn %s{}, err\n\t}\n\treturn %sUnwrap(wrapped), nil\n}", prefix, modelType, docType, modelType, prefix),
			"%sDecodeOne decodes one MongoDB result into the model.",
			"%sDecodeOne decodes one MongoDB result into the model.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sDecodeMany(ctx context.Context, cursor *mongo.Cursor) ([]%s, error) {\n\tdefer cursor.Close(ctx)\n\titems := make([]%s, 0)\n\tfor cursor.Next(ctx) {\n\t\tvar wrapped %s\n\t\tif err := cursor.Decode(&wrapped); err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\titems = append(items, %sUnwrap(wrapped))\n\t}\n\tif err := cursor.Err(); err != nil {\n\t\treturn nil, err\n\t}\n\treturn items, nil\n}", prefix, modelType, modelType, docType, prefix),
			"%sDecodeMany decodes a MongoDB cursor into model items.",
			"%sDecodeMany decodes a MongoDB cursor into model items.",
			prefix,
		),
	}

	if idField := mongoIDField(fields); idField != nil {
		parts = append(parts, sdk.WithDocComment(
			fmt.Sprintf("func %sIDFilter(id any) (bson.M, error) {\n\tvalue, err := %sNormalizeFilterValue(%q, id)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn bson.M{%q: value}, nil\n}", prefix, prefix, idField.BSONName, idField.BSONName),
			"%sIDFilter builds a MongoDB filter for the primary identifier field.",
			"%sIDFilter builds a MongoDB filter for the primary identifier field.",
			prefix,
		))
	}

	return strings.Join(parts, "\n\n")
}

func generateMongoNormalizeHelpers(prefix string, fields []mongoFieldMeta) string {
	nameCases := make([]string, 0, len(fields))
	valueCases := make([]string, 0, len(fields))
	for _, field := range fields {
		nameCases = append(nameCases, fmt.Sprintf("case %q, %q:\n\t\treturn %q, true", field.FieldName, field.BSONName, field.BSONName))
		valueCases = append(valueCases, mongoNormalizeValueCase(field))
	}

	return strings.Join([]string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sNormalizeFieldName(name string) (string, bool) {\n\ttrimmed := strings.TrimSpace(name)\n\tif trimmed == \"\" {\n\t\treturn \"\", false\n\t}\n\tif strings.HasPrefix(trimmed, \"$\") {\n\t\treturn trimmed, true\n\t}\n\tswitch trimmed {\n\t%s\n\tdefault:\n\t\treturn \"\", false\n\t}\n}", prefix, strings.Join(nameCases, "\n\t")),
			"%sNormalizeFieldName maps model field names to BSON field names.",
			"%sNormalizeFieldName maps model field names to BSON field names.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sNormalizeFilterValue(fieldName string, value any) (any, error) {\n\tnormalized, ok := %sNormalizeFieldName(fieldName)\n\tif !ok {\n\t\treturn nil, fmt.Errorf(\"unknown mongodb field %%q\", fieldName)\n\t}\n\tswitch normalized {\n\t%s\n\tdefault:\n\t\treturn value, nil\n\t}\n}", prefix, prefix, strings.Join(valueCases, "\n\t")),
			"%sNormalizeFilterValue normalizes values for BSON-aware filters.",
			"%sNormalizeFilterValue normalizes values for BSON-aware filters.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sFilter(filters map[string]any) (bson.M, error) {\n\tif len(filters) == 0 {\n\t\treturn bson.M{}, nil\n\t}\n\tkeys := make([]string, 0, len(filters))\n\tfor key := range filters {\n\t\tkeys = append(keys, key)\n\t}\n\tsort.Strings(keys)\n\tresult := bson.M{}\n\tfor _, key := range keys {\n\t\tif strings.HasPrefix(key, \"$\") {\n\t\t\tresult[key] = filters[key]\n\t\t\tcontinue\n\t\t}\n\t\tnormalized, ok := %sNormalizeFieldName(key)\n\t\tif !ok {\n\t\t\treturn nil, fmt.Errorf(\"unknown mongodb filter field %%q\", key)\n\t\t}\n\t\tvalue, err := %sNormalizeFilterValue(normalized, filters[key])\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tresult[normalized] = value\n\t}\n\treturn result, nil\n}", prefix, prefix, prefix),
			"%sFilter builds a normalized MongoDB filter document.",
			"%sFilter builds a normalized MongoDB filter document.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sUpdateFieldsDocument(updates map[string]any) (bson.M, error) {\n\tif len(updates) == 0 {\n\t\treturn bson.M{}, nil\n\t}\n\tresult := bson.M{}\n\tfor key, value := range updates {\n\t\tif strings.HasPrefix(strings.TrimSpace(key), \"$\") {\n\t\t\tresult[key] = value\n\t\t\tcontinue\n\t\t}\n\t\tnormalized, ok := %sNormalizeFieldName(key)\n\t\tif !ok {\n\t\t\treturn nil, fmt.Errorf(\"unknown mongodb update field %%q\", key)\n\t\t}\n\t\tnormalizedValue, err := %sNormalizeFilterValue(normalized, value)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tresult[normalized] = normalizedValue\n\t}\n\treturn result, nil\n}", prefix, prefix, prefix),
			"%sUpdateFieldsDocument normalizes a flat update payload.",
			"%sUpdateFieldsDocument normalizes a flat update payload.",
			prefix,
		),
	}, "\n\n")
}

func generateMongoSearchAndSortHelpers(prefix string, fields []mongoFieldMeta) string {
	searchFields := mongoSearchFields(fields)
	searchItems := make([]string, 0, len(searchFields))
	for _, field := range searchFields {
		searchItems = append(searchItems, fmt.Sprintf("{%q: bson.M{\"$regex\": trimmed, \"$options\": \"i\"}}", field.BSONName))
	}

	defaultSort := ""
	if sortable := mongoSortFields(fields); len(sortable) > 0 {
		defaultSort = sortable[0].BSONName
	}

	return strings.Join([]string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sSearchFilter(term string) bson.M {\n\ttrimmed := strings.TrimSpace(term)\n\tif trimmed == \"\" || len(%sSearchFields) == 0 {\n\t\treturn bson.M{}\n\t}\n\treturn bson.M{\"$or\": []bson.M{%s}}\n}", prefix, prefix, strings.Join(searchItems, ", ")),
			"%sSearchFilter builds a case-insensitive `$or` regex filter.",
			"%sSearchFilter builds a case-insensitive `$or` regex filter.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sSort(orderBy string, descending bool) bson.D {\n\tname := strings.TrimSpace(orderBy)\n\tif normalized, ok := %sNormalizeFieldName(name); ok {\n\t\tname = normalized\n\t}\n\tif name == \"\" {\n\t\tname = %q\n\t}\n\torder := 1\n\tif descending {\n\t\torder = -1\n\t}\n\treturn bson.D{{Key: name, Value: order}}\n}", prefix, prefix, defaultSort),
			"%sSort builds a MongoDB sort document.",
			"%sSort builds a MongoDB sort document.",
			prefix,
		),
	}, "\n\n")
}

func generateMongoInsertHelpers(prefix string, modelType string) string {
	return strings.Join([]string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sInsertOne(ctx context.Context, db *mongo.Database, model %s) error {\n\tdocument, err := %sDocumentMap(model)\n\tif err != nil {\n\t\treturn err\n\t}\n\t_, err = %sCollection(db).InsertOne(ctx, document)\n\treturn err\n}", prefix, modelType, prefix, prefix),
			"%sInsertOne inserts one model into MongoDB.",
			"%sInsertOne inserts one model into MongoDB.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sInsertMany(ctx context.Context, db *mongo.Database, models []%s) error {\n\tdocuments, err := %sDocuments(models)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif len(documents) == 0 {\n\t\treturn nil\n\t}\n\t_, err = %sCollection(db).InsertMany(ctx, documents)\n\treturn err\n}", prefix, modelType, prefix, prefix),
			"%sInsertMany inserts multiple models into MongoDB.",
			"%sInsertMany inserts multiple models into MongoDB.",
			prefix,
		),
	}, "\n\n")
}

func generateMongoFindHelpers(prefix string, modelType string, fields []mongoFieldMeta) string {
	parts := []string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sFindOne(ctx context.Context, db *mongo.Database, filters map[string]any) (%s, error) {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn %s{}, err\n\t}\n\tresult := %sCollection(db).FindOne(ctx, filter)\n\treturn %sDecodeOne(result)\n}", prefix, modelType, prefix, modelType, prefix, prefix),
			"%sFindOne finds one MongoDB document by filter.",
			"%sFindOne finds one MongoDB document by filter.",
			prefix,
		),
	}

	if idField := mongoIDField(fields); idField != nil {
		parts = append(parts, sdk.WithDocComment(
			fmt.Sprintf("func %sFindByID(ctx context.Context, db *mongo.Database, id any) (%s, error) {\n\tfilter, err := %sIDFilter(id)\n\tif err != nil {\n\t\treturn %s{}, err\n\t}\n\tresult := %sCollection(db).FindOne(ctx, filter)\n\treturn %sDecodeOne(result)\n}", prefix, modelType, prefix, modelType, prefix, prefix),
			"%sFindByID finds a MongoDB document by the primary identifier field.",
			"%sFindByID finds a MongoDB document by the primary identifier field.",
			prefix,
		))
	}

	parts = append(parts,
		sdk.WithDocComment(
			fmt.Sprintf("func %sFindMany(ctx context.Context, db *mongo.Database, filters map[string]any, limit int64, skip int64, orderBy string, descending bool) ([]%s, error) {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tfindOptions := options.Find()\n\tif limit > 0 {\n\t\tfindOptions.SetLimit(limit)\n\t}\n\tif skip > 0 {\n\t\tfindOptions.SetSkip(skip)\n\t}\n\tfindOptions.SetSort(%sSort(orderBy, descending))\n\tcursor, err := %sCollection(db).Find(ctx, filter, findOptions)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn %sDecodeMany(ctx, cursor)\n}", prefix, modelType, prefix, prefix, prefix, prefix),
			"%sFindMany finds multiple MongoDB documents with paging and sorting.",
			"%sFindMany finds multiple MongoDB documents with paging and sorting.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sList(ctx context.Context, db *mongo.Database, limit int64, skip int64, orderBy string, descending bool) ([]%s, error) {\n\treturn %sFindMany(ctx, db, bson.M{}, limit, skip, orderBy, descending)\n}", prefix, modelType, prefix),
			"%sList lists MongoDB documents with paging and sorting.",
			"%sList lists MongoDB documents with paging and sorting.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sSearch(ctx context.Context, db *mongo.Database, term string, limit int64, skip int64, orderBy string, descending bool) ([]%s, error) {\n\treturn %sFindMany(ctx, db, %sSearchFilter(term), limit, skip, orderBy, descending)\n}", prefix, modelType, prefix, prefix),
			"%sSearch performs a generated text-style search across searchable fields.",
			"%sSearch performs a generated text-style search across searchable fields.",
			prefix,
		),
	)

	return strings.Join(parts, "\n\n")
}

func generateMongoCountHelpers(prefix string) string {
	return strings.Join([]string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sCount(ctx context.Context, db *mongo.Database, filters map[string]any) (int64, error) {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn 0, err\n\t}\n\treturn %sCollection(db).CountDocuments(ctx, filter)\n}", prefix, prefix, prefix),
			"%sCount counts MongoDB documents matching the filter.",
			"%sCount counts MongoDB documents matching the filter.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sExists(ctx context.Context, db *mongo.Database, filters map[string]any) (bool, error) {\n\tcount, err := %sCount(ctx, db, filters)\n\tif err != nil {\n\t\treturn false, err\n\t}\n\treturn count > 0, nil\n}", prefix, prefix),
			"%sExists checks whether at least one MongoDB document matches the filter.",
			"%sExists checks whether at least one MongoDB document matches the filter.",
			prefix,
		),
	}, "\n\n")
}

func generateMongoReplaceHelpers(prefix string, modelType string, fields []mongoFieldMeta) string {
	parts := []string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sReplaceOne(ctx context.Context, db *mongo.Database, model %s, filters map[string]any, upsert bool) error {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn err\n\t}\n\tdocument, err := %sDocumentMap(model)\n\tif err != nil {\n\t\treturn err\n\t}\n\t_, err = %sCollection(db).ReplaceOne(ctx, filter, document, options.Replace().SetUpsert(upsert))\n\treturn err\n}", prefix, modelType, prefix, prefix, prefix),
			"%sReplaceOne replaces one MongoDB document by filter.",
			"%sReplaceOne replaces one MongoDB document by filter.",
			prefix,
		),
	}

	if mongoIDField(fields) != nil {
		parts = append(parts, sdk.WithDocComment(
			fmt.Sprintf("func %sReplaceByID(ctx context.Context, db *mongo.Database, id any, model %s, upsert bool) error {\n\tfilter, err := %sIDFilter(id)\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn %sReplaceOne(ctx, db, model, filter, upsert)\n}", prefix, modelType, prefix, prefix),
			"%sReplaceByID replaces one MongoDB document by primary identifier.",
			"%sReplaceByID replaces one MongoDB document by primary identifier.",
			prefix,
		))
	}

	return strings.Join(parts, "\n\n")
}

func generateMongoUpdateHelpers(prefix string, modelType string, fields []mongoFieldMeta) string {
	parts := []string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sUpdateFields(ctx context.Context, db *mongo.Database, filters map[string]any, updates map[string]any, upsert bool) error {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn err\n\t}\n\tupdateFields, err := %sUpdateFieldsDocument(updates)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif len(updateFields) == 0 {\n\t\treturn nil\n\t}\n\t_, err = %sCollection(db).UpdateOne(ctx, filter, bson.M{\"$set\": updateFields}, options.Update().SetUpsert(upsert))\n\treturn err\n}", prefix, prefix, prefix, prefix),
			"%sUpdateFields updates one MongoDB document with a flat `$set` payload.",
			"%sUpdateFields updates one MongoDB document with a flat `$set` payload.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sUpdateManyFields(ctx context.Context, db *mongo.Database, filters map[string]any, updates map[string]any, upsert bool) error {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn err\n\t}\n\tupdateFields, err := %sUpdateFieldsDocument(updates)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif len(updateFields) == 0 {\n\t\treturn nil\n\t}\n\t_, err = %sCollection(db).UpdateMany(ctx, filter, bson.M{\"$set\": updateFields}, options.Update().SetUpsert(upsert))\n\treturn err\n}", prefix, prefix, prefix, prefix),
			"%sUpdateManyFields updates many MongoDB documents with a flat `$set` payload.",
			"%sUpdateManyFields updates many MongoDB documents with a flat `$set` payload.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sUpdateModel(ctx context.Context, db *mongo.Database, filters map[string]any, model %s, upsert bool) error {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn err\n\t}\n\tupdateFields, err := %sUpdateDocument(model)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif len(updateFields) == 0 {\n\t\treturn nil\n\t}\n\t_, err = %sCollection(db).UpdateOne(ctx, filter, bson.M{\"$set\": updateFields}, options.Update().SetUpsert(upsert))\n\treturn err\n}", prefix, modelType, prefix, prefix, prefix),
			"%sUpdateModel updates one MongoDB document from the model payload.",
			"%sUpdateModel updates one MongoDB document from the model payload.",
			prefix,
		),
	}

	if mongoIDField(fields) != nil {
		parts = append(parts, sdk.WithDocComment(
			fmt.Sprintf("func %sUpdateByID(ctx context.Context, db *mongo.Database, id any, model %s, upsert bool) error {\n\tfilter, err := %sIDFilter(id)\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn %sUpdateModel(ctx, db, filter, model, upsert)\n}", prefix, modelType, prefix, prefix),
			"%sUpdateByID updates one MongoDB document by primary identifier.",
			"%sUpdateByID updates one MongoDB document by primary identifier.",
			prefix,
		))
	}

	return strings.Join(parts, "\n\n")
}

func generateMongoDeleteHelpers(prefix string, fields []mongoFieldMeta) string {
	parts := []string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sDeleteOne(ctx context.Context, db *mongo.Database, filters map[string]any) error {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn err\n\t}\n\t_, err = %sCollection(db).DeleteOne(ctx, filter)\n\treturn err\n}", prefix, prefix, prefix),
			"%sDeleteOne deletes one MongoDB document by filter.",
			"%sDeleteOne deletes one MongoDB document by filter.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sDeleteMany(ctx context.Context, db *mongo.Database, filters map[string]any) error {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn err\n\t}\n\t_, err = %sCollection(db).DeleteMany(ctx, filter)\n\treturn err\n}", prefix, prefix, prefix),
			"%sDeleteMany deletes many MongoDB documents by filter.",
			"%sDeleteMany deletes many MongoDB documents by filter.",
			prefix,
		),
	}

	if mongoIDField(fields) != nil {
		parts = append(parts, sdk.WithDocComment(
			fmt.Sprintf("func %sDeleteByID(ctx context.Context, db *mongo.Database, id any) error {\n\tfilter, err := %sIDFilter(id)\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn %sDeleteOne(ctx, db, filter)\n}", prefix, prefix, prefix),
			"%sDeleteByID deletes one MongoDB document by primary identifier.",
			"%sDeleteByID deletes one MongoDB document by primary identifier.",
			prefix,
		))
	}

	return strings.Join(parts, "\n\n")
}

func generateMongoExtraHelpers(prefix string, modelType string) string {
	return strings.Join([]string{
		sdk.WithDocComment(
			fmt.Sprintf("func %sDistinct(ctx context.Context, db *mongo.Database, fieldName string, filters map[string]any) ([]any, error) {\n\tfilter, err := %sFilter(filters)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tnormalized, ok := %sNormalizeFieldName(fieldName)\n\tif !ok {\n\t\treturn nil, nil\n\t}\n\treturn %sCollection(db).Distinct(ctx, normalized, filter)\n}", prefix, prefix, prefix, prefix),
			"%sDistinct returns distinct values for a generated MongoDB field.",
			"%sDistinct returns distinct values for a generated MongoDB field.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sAggregate(ctx context.Context, db *mongo.Database, pipeline mongo.Pipeline) ([]%s, error) {\n\tcursor, err := %sCollection(db).Aggregate(ctx, pipeline)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn %sDecodeMany(ctx, cursor)\n}", prefix, modelType, prefix, prefix),
			"%sAggregate runs an aggregation pipeline and decodes typed results.",
			"%sAggregate runs an aggregation pipeline and decodes typed results.",
			prefix,
		),
		sdk.WithDocComment(
			fmt.Sprintf("func %sWatch(ctx context.Context, db *mongo.Database, pipeline mongo.Pipeline) (*mongo.ChangeStream, error) {\n\treturn %sCollection(db).Watch(ctx, pipeline)\n}", prefix, prefix),
			"%sWatch opens a MongoDB change stream for the collection.",
			"%sWatch opens a MongoDB change stream for the collection.",
			prefix,
		),
	}, "\n\n")
}

func mongoWrapperType(field mongoFieldMeta, file sdk.FileContext) string {
	if field.ObjectID {
		if field.Type.Optional {
			return "*primitive.ObjectID"
		}
		return "primitive.ObjectID"
	}
	return mongoQualifiedType(field.Type, file)
}

func mongoQualifiedType(typeRef sdk.TypeRef, file sdk.FileContext) string {
	base := strings.TrimSpace(typeRef.Name)
	if base == "" {
		base = "any"
	}

	if !strings.Contains(base, ".") {
		if _, ok := typeRef.RefModel(file); ok {
			base = "modelpkg." + base
		}
	}

	if typeRef.IsList {
		return "[]" + base
	}
	if typeRef.Optional {
		return "*" + base
	}
	return base
}

func mongoWrapFieldCode(field mongoFieldMeta) []string {
	source := "model." + field.FieldName
	target := "doc." + field.FieldName

	if field.ObjectID {
		if field.Type.Optional {
			return []string{
				fmt.Sprintf("if %s != nil && strings.TrimSpace(*%s) != \"\" {", source, source),
				fmt.Sprintf("\tparsed, err := primitive.ObjectIDFromHex(*%s)", source),
				"\tif err != nil {",
				fmt.Sprintf("\t\treturn doc, fmt.Errorf(\"invalid object id for field %s: %%w\", err)", field.BSONName),
				"\t}",
				fmt.Sprintf("\t%s = &parsed", target),
				"}",
			}
		}
		return []string{
			fmt.Sprintf("if strings.TrimSpace(%s) != \"\" {", source),
			fmt.Sprintf("\tparsed, err := primitive.ObjectIDFromHex(%s)", source),
			"\tif err != nil {",
			fmt.Sprintf("\t\treturn doc, fmt.Errorf(\"invalid object id for field %s: %%w\", err)", field.BSONName),
			"\t}",
			fmt.Sprintf("\t%s = parsed", target),
			"}",
		}
	}

	if field.Type.IsTime() {
		switch {
		case field.Type.Optional && field.UpdatedAt:
			return []string{
				"if touchUpdatedAt {",
				"\tnow := time.Now()",
				fmt.Sprintf("\t%s = &now", target),
				fmt.Sprintf("} else if %s != nil {", source),
				fmt.Sprintf("\t%s = %s", target, source),
				fmt.Sprintf("} else if includeDefaults && strings.EqualFold(%q, \"now\") {", field.Default),
				"\tnow := time.Now()",
				fmt.Sprintf("\t%s = &now", target),
				"}",
			}
		case field.Type.Optional:
			return []string{
				fmt.Sprintf("if %s != nil {", source),
				fmt.Sprintf("\t%s = %s", target, source),
				fmt.Sprintf("} else if includeDefaults && strings.EqualFold(%q, \"now\") {", field.Default),
				"\tnow := time.Now()",
				fmt.Sprintf("\t%s = &now", target),
				"}",
			}
		case field.UpdatedAt:
			return []string{
				"if touchUpdatedAt {",
				fmt.Sprintf("\t%s = time.Now()", target),
				fmt.Sprintf("} else if includeDefaults && strings.EqualFold(%q, \"now\") && %s.IsZero() {", field.Default, source),
				fmt.Sprintf("\t%s = time.Now()", target),
				"} else {",
				fmt.Sprintf("\t%s = %s", target, source),
				"}",
			}
		case strings.EqualFold(field.Default, "now"):
			return []string{
				fmt.Sprintf("if includeDefaults && %s.IsZero() {", source),
				fmt.Sprintf("\t%s = time.Now()", target),
				"} else {",
				fmt.Sprintf("\t%s = %s", target, source),
				"}",
			}
		}
	}

	return []string{fmt.Sprintf("%s = %s", target, source)}
}

func mongoUnwrapFieldCode(field mongoFieldMeta) []string {
	source := "doc." + field.FieldName
	target := "model." + field.FieldName

	if field.ObjectID {
		if field.Type.Optional {
			return []string{
				fmt.Sprintf("if %s != nil && !%s.IsZero() {", source, source),
				fmt.Sprintf("\tvalue := %s.Hex()", source),
				fmt.Sprintf("\t%s = &value", target),
				"}",
			}
		}
		return []string{
			fmt.Sprintf("if !%s.IsZero() {", source),
			fmt.Sprintf("\t%s = %s.Hex()", target, source),
			"}",
		}
	}

	return []string{fmt.Sprintf("%s = %s", target, source)}
}

func mongoNormalizeValueCase(field mongoFieldMeta) string {
	if !field.ObjectID {
		return fmt.Sprintf("case %q:\n\t\treturn value, nil", field.BSONName)
	}

	return fmt.Sprintf(`case %q:
		switch typed := value.(type) {
		case nil:
			return nil, nil
		case primitive.ObjectID:
			return typed, nil
		case *primitive.ObjectID:
			return typed, nil
		case string:
			if strings.TrimSpace(typed) == "" {
				return primitive.NilObjectID, nil
			}
			return primitive.ObjectIDFromHex(strings.TrimSpace(typed))
		case *string:
			if typed == nil || strings.TrimSpace(*typed) == "" {
				return nil, nil
			}
			value, err := primitive.ObjectIDFromHex(strings.TrimSpace(*typed))
			if err != nil {
				return nil, err
			}
			return value, nil
		default:
			return nil, fmt.Errorf("unsupported object id value %%T", value)
		}`, field.BSONName)
}

func mongoSearchFields(fields []mongoFieldMeta) []mongoFieldMeta {
	items := make([]mongoFieldMeta, 0)
	for _, field := range fields {
		if field.Search {
			items = append(items, field)
		}
	}
	return items
}

func mongoSortFields(fields []mongoFieldMeta) []mongoFieldMeta {
	items := make([]mongoFieldMeta, 0)
	for _, field := range fields {
		if field.Sort {
			items = append(items, field)
		}
	}
	return items
}

func mongoIDField(fields []mongoFieldMeta) *mongoFieldMeta {
	for index := range fields {
		if fields[index].ObjectID && fields[index].IsIDLike {
			return &fields[index]
		}
	}
	for index := range fields {
		if fields[index].IsIDLike {
			return &fields[index]
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &fields[0]
}

func indentBlock(lines []string, level int) []string {
	prefix := strings.Repeat("\t", level)
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			result = append(result, "")
			continue
		}
		result = append(result, prefix+line)
	}
	return result
}
