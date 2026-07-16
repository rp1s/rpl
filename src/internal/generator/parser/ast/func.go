package ast

import (
	"sort"
	"strings"
)

// Walk обходит все AST-узлы верхнего уровня в файле.
// В fn по очереди передаётся каждый корневой узел из file.ASTs.
// Если fn возвращает false, обход останавливается.
// Ничего не возвращает.
func (file *File) Walk(fn func(AST) bool) {
	if file == nil || fn == nil {
		return
	}

	for _, node := range file.ASTs {
		if !fn(node) {
			return
		}
	}
}

// FindModel ищет модель по имени.
// Принимает имя модели, например "User".
// Возвращает указатель на ModelAST и true, если модель найдена.
// Если модель не найдена, возвращает nil и false.
func (file *File) FindModel(name string) (*ModelAST, bool) {
	if file == nil {
		return nil, false
	}

	for i := range file.ASTs {
		model, ok := file.ASTs[i].(*ModelAST)
		if !ok {
			continue
		}
		if model.Name == name {
			return model, true
		}
	}

	return nil, false
}

// FindType ищет top-level type alias по имени.
func (file *File) FindType(name string) (*TypeAliasAST, bool) {
	if file == nil {
		return nil, false
	}

	for i := range file.ASTs {
		item, ok := file.ASTs[i].(*TypeAliasAST)
		if !ok {
			continue
		}
		if item.Name == name {
			return item, true
		}
	}

	return nil, false
}

// Models возвращает все модели, объявленные в файле.
// Ничего не принимает.
// Возвращает срез указателей на все ModelAST из текущего файла.
func (file *File) Models() []*ModelAST {
	if file == nil {
		return nil
	}

	models := make([]*ModelAST, 0)
	for _, node := range file.ASTs {
		model, ok := node.(*ModelAST)
		if ok {
			models = append(models, model)
		}
	}

	return models
}

// Types возвращает все top-level type aliases файла.
func (file *File) Types() []*TypeAliasAST {
	if file == nil {
		return nil
	}

	items := make([]*TypeAliasAST, 0)
	for _, node := range file.ASTs {
		item, ok := node.(*TypeAliasAST)
		if ok {
			items = append(items, item)
		}
	}

	return items
}

// Imports возвращает все блоки import, объявленные в файле.
// Ничего не принимает.
// Возвращает срез указателей на все ImportAST из текущего файла.
func (file *File) Imports() []*ImportAST {
	if file == nil {
		return nil
	}

	imports := make([]*ImportAST, 0)
	for _, node := range file.ASTs {
		importNode, ok := node.(*ImportAST)
		if ok {
			imports = append(imports, importNode)
		}
	}

	return imports
}

// Runtimes возвращает все блоки runtimes, объявленные в файле.
// Ничего не принимает.
// Возвращает срез указателей на все RuntimesAST из текущего файла.
func (file *File) Runtimes() []*RuntimesAST {
	if file == nil {
		return nil
	}

	runtimes := make([]*RuntimesAST, 0)
	for _, node := range file.ASTs {
		runtimeNode, ok := node.(*RuntimesAST)
		if ok {
			runtimes = append(runtimes, runtimeNode)
		}
	}

	return runtimes
}

// RuntimeMap возвращает все рантаймы файла в виде map по идентификатору.
// Идентификатором считается "author:name" либо просто "name", если автор не указан.
// Если один и тот же рантайм объявлен несколько раз, последнее значение перезапишет предыдущее.
func (file *File) RuntimeMap() map[string]RuntimeSpec {
	if file == nil {
		return nil
	}

	runtimes := make(map[string]RuntimeSpec)
	for _, runtimeBlock := range file.Runtimes() {
		if runtimeBlock == nil {
			continue
		}

		for i := range runtimeBlock.Specs {
			spec := runtimeBlock.Specs[i]
			key := spec.Identifier()
			if key == "" {
				continue
			}

			runtimes[key] = spec
		}
	}

	return runtimes
}

// ModelsWithAttr возвращает все модели файла, у которых есть указанный атрибут
// либо на самой модели, либо на любом её поле.
// Например можно искать "group", "grpc", "validate" или "max".
func (file *File) ModelsWithAttr(name string) []*ModelAST {
	if file == nil {
		return nil
	}

	models := make([]*ModelAST, 0)
	for _, model := range file.Models() {
		if model != nil && model.HasAnyAttr(name) {
			models = append(models, model)
		}
	}

	return models
}

// ModelsByAttrs автоматически группирует модели по всем их атрибутам:
// и атрибутам самой модели, и атрибутам её полей.
// Возвращает коллекцию групп вида:
// [{Attr: "grpc", Models: [...]}, {Attr: "max", Models: [...]}].
func (file *File) ModelsByAttrs() []ModelsGroup {
	if file == nil {
		return nil
	}

	grouped := make(map[string][]*ModelAST)
	for _, model := range file.Models() {
		if model == nil {
			continue
		}

		for _, name := range model.AllAttrNames() {
			grouped[name] = append(grouped[name], model)
		}
	}

	if len(grouped) == 0 {
		return nil
	}

	items := make([]ModelsGroup, 0, len(grouped))
	for attrName, models := range grouped {
		items = append(items, ModelsGroup{
			Attr:   attrName,
			Models: models,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Attr < items[j].Attr
	})

	return items
}

// GroupedModelsByAttrs возвращает группировку моделей по атрибутам в виде среза структур.
// Это алиас к ModelsByAttrs().
func (file *File) GroupedModelsByAttrs() []ModelsGroup {
	return file.ModelsByAttrs()
}

// FindField ищет поле модели по имени.
// Принимает имя поля, например "Email".
// Возвращает указатель на FieldAST и true, если поле найдено.
// Если поле не найдено, возвращает nil и false.
func (model *ModelAST) FindField(name string) (*FieldAST, bool) {
	if model == nil {
		return nil, false
	}

	for i := range model.Fields {
		if model.Fields[i] != nil && model.Fields[i].Name == name {
			return model.Fields[i], true
		}
	}

	return nil, false
}

// HasAttr проверяет, есть ли у модели атрибут с таким именем.
// Принимает имя атрибута, например "group".
// Возвращает true, если у модели есть такой атрибут.
func (model *ModelAST) HasAttr(name string) bool {
	_, ok := model.FindAttr(name)
	return ok
}

// HasAnyAttr проверяет атрибут и на самой модели, и на любом её поле.
func (model *ModelAST) HasAnyAttr(name string) bool {
	if model == nil {
		return false
	}

	if model.HasAttr(name) {
		return true
	}

	for _, method := range model.Methods {
		if methodHasAttr(method, name) {
			return true
		}
	}

	for _, field := range model.Fields {
		if field != nil && field.HasAttr(name) {
			return true
		}
	}

	return false
}

// AllAttrNames возвращает все уникальные имена атрибутов модели:
// и с самой модели, и со всех её полей.
func (model *ModelAST) AllAttrNames() []string {
	if model == nil {
		return nil
	}

	seen := make(map[string]struct{})
	names := make([]string, 0)

	appendAttrNames := func(attrs []Attr) {
		for _, attr := range attrs {
			name := attr.Identifier()
			if name == "" {
				continue
			}

			if _, ok := seen[name]; ok {
				continue
			}

			seen[name] = struct{}{}
			names = append(names, name)
		}
	}

	appendAttrNames(model.Attrs)
	for _, method := range model.Methods {
		appendAttrNames(method.Attrs)
	}
	for _, field := range model.Fields {
		if field == nil {
			continue
		}

		appendAttrNames(field.Attrs)
		for _, method := range field.Methods {
			appendAttrNames(method.Attrs)
		}
	}

	sort.Strings(names)
	return names
}

func methodHasAttr(method FieldMethodAST, name string) bool {
	for i := range method.Attrs {
		if method.Attrs[i].Matches(name) {
			return true
		}
	}
	return false
}

// FindAttr ищет атрибут модели по имени.
// Принимает имя атрибута.
// Возвращает указатель на Attr и true, если атрибут найден.
// Если атрибут не найден, возвращает nil и false.
func (model *ModelAST) FindAttr(name string) (*Attr, bool) {
	if model == nil {
		return nil, false
	}

	for i := range model.Attrs {
		if model.Attrs[i].Matches(name) {
			return &model.Attrs[i], true
		}
	}

	return nil, false
}

// FieldsWithAttr возвращает все поля модели с указанным атрибутом.
// Принимает имя атрибута, например "min" или "email".
// Возвращает срез указателей на поля, у которых найден указанный атрибут.
func (model *ModelAST) FieldsWithAttr(name string) []*FieldAST {
	if model == nil {
		return nil
	}

	fields := make([]*FieldAST, 0)
	for _, field := range model.Fields {
		if field != nil && field.HasAttr(name) {
			fields = append(fields, field)
		}
	}

	return fields
}

// FieldsWithAttrValue возвращает все поля модели, у которых есть указанный
// атрибут и значение его аргумента совпадает с переданным.
// Принимает имя атрибута, строковое значение и индекс аргумента.
// Например можно искать @default("now") через ("default", "now", 0).
// Возвращает срез указателей на поля, удовлетворяющих условию.
func (model *ModelAST) FieldsWithAttrValue(name, value string, argIndex int) []*FieldAST {
	if model == nil {
		return nil
	}

	fields := make([]*FieldAST, 0)
	for _, field := range model.Fields {
		if field == nil {
			continue
		}

		attr, ok := field.FindAttr(name)
		if !ok {
			continue
		}

		arg, ok := attr.Arg(argIndex)
		if !ok {
			continue
		}

		if ExprString(arg) == value {
			fields = append(fields, field)
		}
	}

	return fields
}

// FieldsLen возвращает количество полей в модели.
// Ничего не принимает.
// Возвращает число полей в model.Fields.
func (model *ModelAST) FieldsLen() int {
	if model == nil {
		return 0
	}

	return len(model.Fields)
}

// FieldNames возвращает имена всех полей модели.
// Ничего не принимает.
// Возвращает срез строк с именами всех полей в порядке объявления.
func (model *ModelAST) FieldNames() []string {
	if model == nil {
		return nil
	}

	names := make([]string, 0, len(model.Fields))
	for _, field := range model.Fields {
		if field == nil {
			continue
		}

		names = append(names, field.Name)
	}

	return names
}

// FieldAttrs возвращает все поля модели вместе с их атрибутами.
// Ничего не принимает.
// Возвращает срез FieldAttrs, где:
// Field - это само поле,
// Attrs - это все атрибуты этого поля.
func (model *ModelAST) FieldAttrs() []FieldAttrs {
	if model == nil {
		return nil
	}

	items := make([]FieldAttrs, 0, len(model.Fields))
	for _, field := range model.Fields {
		if field == nil {
			continue
		}

		items = append(items, FieldAttrs{
			Field: field,
			Attrs: field.Attrs,
		})
	}

	return items
}

// HasAttr проверяет, есть ли у поля атрибут с таким именем.
// Принимает имя атрибута.
// Возвращает true, если у поля найден указанный атрибут.
func (field *FieldAST) HasAttr(name string) bool {
	_, ok := field.FindAttr(name)
	return ok
}

// FindAttr ищет атрибут поля по имени.
// Принимает имя атрибута.
// Возвращает указатель на Attr и true, если атрибут найден.
// Если атрибут не найден, возвращает nil и false.
func (field *FieldAST) FindAttr(name string) (*Attr, bool) {
	if field == nil {
		return nil, false
	}

	for i := range field.Attrs {
		if field.Attrs[i].Matches(name) {
			return &field.Attrs[i], true
		}
	}

	return nil, false
}

// Arg возвращает аргумент атрибута по индексу.
// Принимает индекс аргумента, начиная с нуля.
// Возвращает Expr и true, если аргумент существует.
// Если индекс выходит за границы, возвращает nil и false.
func (attr *Attr) Arg(index int) (Expr, bool) {
	if attr == nil || index < 0 || index >= len(attr.Args) {
		return nil, false
	}

	return attr.Args[index], true
}

// ExprString возвращает строковое представление выражения.
// Поддерживает StringExpr, NumberExpr, BoolExpr, NameExpr и GoExpr.
// Если тип выражения неизвестен, возвращает пустую строку.
func ExprString(expr Expr) string {
	switch value := expr.(type) {
	case StringExpr:
		return value.Value
	case NumberExpr:
		return value.Value
	case BoolExpr:
		if value.Value {
			return "true"
		}
		return "false"
	case NameExpr:
		return value.Name.String()
	case GoExpr:
		return value.Text
	default:
		return ""
	}
}

// FindSpecByAlias ищет import-элемент по алиасу.
// Принимает алиас, например "t" для записи t "time".
// Возвращает указатель на ImportSpec и true, если элемент найден.
// Если элемент не найден, возвращает nil и false.
func (importNode *ImportAST) FindSpecByAlias(alias string) (*ImportSpec, bool) {
	if importNode == nil {
		return nil, false
	}

	for i := range importNode.Specs {
		if importNode.Specs[i].Alias == alias {
			return &importNode.Specs[i], true
		}
	}

	return nil, false
}

// FindSpecByPath ищет import-элемент по пути.
// Принимает путь импорта, например "time".
// Возвращает указатель на ImportSpec и true, если элемент найден.
// Если элемент не найден, возвращает nil и false.
func (importNode *ImportAST) FindSpecByPath(path string) (*ImportSpec, bool) {
	if importNode == nil {
		return nil, false
	}

	for i := range importNode.Specs {
		if importNode.Specs[i].Path == path {
			return &importNode.Specs[i], true
		}
	}

	return nil, false
}

// String собирает имя из частей через точку.
// Например Name{Parts: []string{"time", "Time"}} превратится в "time.Time".
// Ничего не принимает и возвращает строковое представление имени.
func (name Name) String() string {
	return strings.Join(name.Parts, ".")
}

func (spec RuntimeSpec) Identifier() string {
	switch {
	case spec.Author == "":
		return spec.Name
	case spec.Name == "":
		return spec.Author
	default:
		return spec.Author + ":" + spec.Name
	}
}

// EqualString проверяет, совпадает ли имя с переданной строкой.
// Принимает строку, например "time.Time".
// Возвращает true, если результат name.String() равен переданному значению.
func (name Name) EqualString(value string) bool {
	return name.String() == value
}

func (attr Attr) Identifier() string {
	packageName := attr.Package.String()
	name := attr.Name.String()

	switch {
	case name == "":
		return packageName
	case packageName == "":
		return name
	case packageName == name:
		return name
	default:
		return packageName + "." + name
	}
}

func (attr Attr) Matches(value string) bool {
	if value == "" {
		return false
	}

	return attr.Name.EqualString(value) || attr.Identifier() == value
}
