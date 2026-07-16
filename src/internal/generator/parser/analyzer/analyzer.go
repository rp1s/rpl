package analyzer

import (
	"errors"
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	"path"
	"strings"
	"unicode"

	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer/token"
	targetpkg "rpl/internal/generator/target"
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
)

type scope string

const (
	scopeTarget scope = "target"
	scopeModel  scope = "model"
	scopeField  scope = "field"
	scopeMethod scope = "method"
)

var (
	generalAttrNamespaces = map[string]struct{}{
		"comment": {},
		"group":   {},
		"ignore":  {},
	}
)

type analysis struct {
	file            *ast.File
	models          map[string]*ast.ModelAST
	types           map[string]*ast.TypeAliasAST
	groupCandidates map[string]struct{}
	imports         map[string]ast.ImportSpec
	declaredRuntime map[string]ast.RuntimeSpec
	problems        []error
}

func AnalyzeRaw(file *ast.File) error {
	if file == nil {
		return nil
	}

	state := newAnalysis(file)
	state.validateTarget()
	state.validateDuplicateFields()
	state.validateFieldExtensions()
	state.validateModelMethodExtensions()
	state.validateFieldMethodExtensions()
	state.validateAttrTree()
	state.validateTypeAliasesRaw()
	state.validateTypeReferencesRaw()
	return state.err()
}

func Analyze(file *ast.File) error {
	if file == nil {
		return nil
	}

	state := newAnalysis(file)
	state.validateUnusedImports()
	state.validateGeneratedNameCollisions()
	return state.err()
}

func newAnalysis(file *ast.File) *analysis {
	return &analysis{
		file:            file,
		models:          modelIndex(file),
		types:           typeIndex(file),
		groupCandidates: groupCandidateIndex(file),
		imports:         importAliasIndex(file),
		declaredRuntime: runtimeIndex(file),
		problems:        make([]error, 0),
	}
}

func (state *analysis) err() error {
	if len(state.problems) == 0 {
		return nil
	}

	return errors.Join(state.problems...)
}

func (state *analysis) add(err error) {
	if err != nil {
		state.problems = append(state.problems, err)
	}
}

func (state *analysis) validateTarget() {
	targets := state.file.Targets()
	var baseline *ast.TargetAST
	for _, target := range targets {
		if target == nil {
			continue
		}

		state.validateSingleTarget(target)
		if baseline == nil {
			baseline = target
			continue
		}
		if targetsEquivalent(baseline, target) {
			continue
		}

		state.add(newProblem(
			target.Position,
			localize.Text("target-директивы в одном package должны совпадать", "target directives inside one package must match"),
		).WithHint(localize.Text(
			"Оставьте один общий `target(lang: ...)` или используйте одинаковое значение во всех файлах package.",
			"Keep one shared `target(lang: ...)` or use the same value in every file of the package.",
		)))
	}
}

func (state *analysis) validateSingleTarget(target *ast.TargetAST) {
	if target == nil {
		return
	}

	if len(target.Args) > 0 {
		state.add(newProblem(
			target.Position,
			localize.Text("target использует только именованные аргументы", "target only supports named arguments"),
		).WithHint(localize.Text(
			"Используйте форму `target(lang: golang)`.",
			"Use the `target(lang: golang)` form.",
		)))
	}

	allowedArgs := map[string]struct{}{"lang": {}}
	for _, item := range target.NamedArgs {
		if _, ok := allowedArgs[item.Name]; ok {
			continue
		}

		state.add(newProblem(
			item.Position,
			fmt.Sprintf(localize.Text("неизвестный аргумент target %q", "unknown target argument %q"), item.Name),
		).WithHint(localize.Text(
			"Сейчас target понимает только `lang`.",
			"Right now target only understands `lang`.",
		)))
	}

	lang := strings.TrimSpace(targetLangValue(target))
	if lang == "" {
		return
	}
	if _, ok := targetpkg.Lookup(targetpkg.NormalizeLanguage(lang)); ok {
		return
	}

	state.add(newProblem(
		target.Position,
		fmt.Sprintf(localize.Text("неподдерживаемый target language %q", "unsupported target language %q"), lang),
	).WithHint(localize.Text(
		"Сейчас поддерживается `golang`. Пример: `target(lang: golang)`.",
		"Right now `golang` is supported. Example: `target(lang: golang)`.",
	)))
}

func targetLangValue(target *ast.TargetAST) string {
	if target == nil {
		return ""
	}

	value, ok := target.NamedArg("lang")
	if !ok {
		return ""
	}

	return ast.ExprString(value)
}

func targetsEquivalent(left *ast.TargetAST, right *ast.TargetAST) bool {
	if left == nil || right == nil {
		return left == right
	}

	return targetpkg.NormalizeLanguage(strings.TrimSpace(targetLangValue(left))) ==
		targetpkg.NormalizeLanguage(strings.TrimSpace(targetLangValue(right)))
}

func (state *analysis) validateDuplicateFields() {
	for _, model := range state.file.Models() {
		if model == nil {
			continue
		}

		seen := make(map[string]tokenOwner)
		for _, field := range model.Fields {
			if field == nil {
				continue
			}

			if previous, ok := seen[field.Name]; ok {
				state.add(newProblem(
					field.Position,
					fmt.Sprintf(localize.Text("дублирующееся поле %q в модели %q", "duplicate field %q in model %q"), field.Name, model.Name),
				).WithDetail(fmt.Sprintf(localize.Text(
					"Первое объявление уже существует в %s.",
					"The first declaration already exists at %s.",
				), positionLabel(previous.position))).WithHint(localize.Text(
					"Переименуйте одно из полей или объедините конфигурацию в один блок поля.",
					"Rename one of the fields or merge the configuration into a single field block.",
				)))
				continue
			}

			seen[field.Name] = tokenOwner{name: field.Name, position: field.Position}
		}
	}
}

func (state *analysis) validateFieldExtensions() {
	for _, extension := range state.file.FieldExtensions() {
		if extension == nil {
			continue
		}

		modelName := strings.TrimSpace(extension.Model.String())
		fieldName := strings.TrimSpace(extension.Field.String())

		model, ok := state.models[modelName]
		if !ok {
			state.add(newProblem(
				extension.Position,
				fmt.Sprintf(localize.Text("модель %q для field-блока не найдена", "model %q for field block was not found"), modelName),
			).WithHint(localize.Text(
				"Сначала объявите модель, а потом добавляйте `field Model.Field { ... }` блоки.",
				"Declare the model first, then add `field Model.Field { ... }` blocks.",
			)))
			continue
		}

		if _, ok := model.FindField(fieldName); ok {
			continue
		}

		state.add(newProblem(
			extension.Position,
			fmt.Sprintf(localize.Text("поле %q модели %q для field-блока не найдено", "field %q of model %q for field block was not found"), fieldName, modelName),
		).WithHint(localize.Text(
			"Проверьте имя поля и убедитесь, что оно объявлено внутри модели.",
			"Check the field name and make sure it is declared inside the model.",
		)))
	}
}

func (state *analysis) validateFieldMethodExtensions() {
	for _, extension := range state.file.FieldMethodExtensions() {
		if extension == nil {
			continue
		}

		modelName := strings.TrimSpace(extension.Model.String())
		fieldName := strings.TrimSpace(extension.Field.String())

		model, ok := state.models[modelName]
		if !ok {
			state.add(newProblem(
				extension.Position,
				fmt.Sprintf(localize.Text("модель %q для func-блока не найдена", "model %q for func block was not found"), modelName),
			).WithHint(localize.Text(
				"Сначала объявите модель, а потом добавляйте `func Model.Field { ... }` блоки.",
				"Declare the model first, then add `func Model.Field { ... }` blocks.",
			)))
			continue
		}

		field, ok := model.FindField(fieldName)
		if !ok || field == nil {
			state.add(newProblem(
				extension.Position,
				fmt.Sprintf(localize.Text("поле %q модели %q для func-блока не найдено", "field %q of model %q for func block was not found"), fieldName, modelName),
			).WithHint(localize.Text(
				"Проверьте имя поля и убедитесь, что оно объявлено внутри модели.",
				"Check the field name and make sure it is declared inside the model.",
			)))
			continue
		}

		seen := make(map[string]struct{})
		for _, method := range field.Methods {
			seen[strings.TrimSpace(method.Name)] = struct{}{}
		}
		for _, method := range extension.Methods {
			name := strings.TrimSpace(method.Name)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				state.add(newProblem(
					method.Position,
					fmt.Sprintf(localize.Text("метод %q для поля %q уже объявлен", "method %q for field %q is already declared"), name, fieldName),
				).WithHint(localize.Text(
					"Переименуйте метод или оставьте его только в одном месте: внутри поля или в `func Model.Field { ... }`.",
					"Rename the method or keep it in only one place: inside the field or inside `func Model.Field { ... }`.",
				)))
			}
			seen[name] = struct{}{}
		}
	}
}

func (state *analysis) validateModelMethodExtensions() {
	for _, extension := range state.file.ModelMethodExtensions() {
		if extension == nil {
			continue
		}

		modelName := strings.TrimSpace(extension.Model.String())
		model, ok := state.models[modelName]
		if !ok {
			state.add(newProblem(
				extension.Position,
				fmt.Sprintf(localize.Text("модель %q для func-блока не найдена", "model %q for func block was not found"), modelName),
			).WithHint(localize.Text(
				"Сначала объявите модель, а потом добавляйте `func Model { ... }` блоки.",
				"Declare the model first, then add `func Model { ... }` blocks.",
			)))
			continue
		}

		seen := make(map[string]struct{})
		for _, method := range model.Methods {
			seen[strings.TrimSpace(method.Name)] = struct{}{}
		}
		for _, method := range extension.Methods {
			name := strings.TrimSpace(method.Name)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				state.add(newProblem(
					method.Position,
					fmt.Sprintf(localize.Text("метод %q для модели %q уже объявлен", "method %q for model %q is already declared"), name, modelName),
				).WithHint(localize.Text(
					"Переименуйте метод или оставьте его только в одном месте: внутри модели или в `func Model { ... }`.",
					"Rename the method or keep it in only one place: inside the model or inside `func Model { ... }`.",
				)))
			}
			seen[name] = struct{}{}
		}
	}
}

func (state *analysis) validateAttrTree() {
	for _, model := range state.file.Models() {
		if model == nil {
			continue
		}

		for i := range model.Attrs {
			state.validateAttr(model, nil, nil, model.Attrs[i], scopeModel)
		}
		for i := range model.Methods {
			method := &model.Methods[i]
			for j := range method.Attrs {
				state.validateAttr(model, nil, method, method.Attrs[j], scopeMethod)
			}
		}
		for _, field := range model.Fields {
			if field == nil {
				continue
			}

			for i := range field.Attrs {
				state.validateAttr(model, field, nil, field.Attrs[i], scopeField)
			}
			state.validateFieldDefault(field)

			for i := range field.Methods {
				method := &field.Methods[i]
				for j := range method.Attrs {
					state.validateAttr(model, field, method, method.Attrs[j], scopeMethod)
				}
			}
		}
	}
}

func (state *analysis) validateTypeReferencesRaw() {
	for _, item := range state.file.Types() {
		if item == nil {
			continue
		}

		state.validateTypeRef(item.Position, item.Type, typeContext{
			allowBare: true,
			label:     fmt.Sprintf(localize.Text("type %q", "type %q"), item.Name),
		})
	}

	for _, model := range state.file.Models() {
		if model == nil {
			continue
		}

		for i := range model.Methods {
			method := &model.Methods[i]
			for _, param := range method.Params {
				state.validateTypeRef(param.Position, param.Type, typeContext{
					model:     model,
					method:    method,
					allowBare: true,
					label:     fmt.Sprintf(localize.Text("аргумент %q метода %q", "argument %q of method %q"), param.Name, method.Name),
					methodSig: true,
				})
			}
			for _, result := range method.Returns {
				state.validateTypeRef(method.Position, result, typeContext{
					model:     model,
					method:    method,
					allowBare: true,
					label:     fmt.Sprintf(localize.Text("результат метода %q", "return value of method %q"), method.Name),
					methodSig: true,
				})
			}
		}

		for _, field := range model.Fields {
			if field == nil {
				continue
			}

			state.validateTypeRef(field.Position, field.Type, typeContext{
				model:     model,
				field:     field,
				allowBare: true,
				label:     fmt.Sprintf(localize.Text("поле %q", "field %q"), field.Name),
			})

			for i := range field.Methods {
				method := &field.Methods[i]
				for _, param := range method.Params {
					state.validateTypeRef(param.Position, param.Type, typeContext{
						model:     model,
						field:     field,
						method:    method,
						allowBare: true,
						label:     fmt.Sprintf(localize.Text("аргумент %q метода %q", "argument %q of method %q"), param.Name, method.Name),
						methodSig: true,
					})
				}
				for _, result := range method.Returns {
					state.validateTypeRef(method.Position, result, typeContext{
						model:     model,
						field:     field,
						method:    method,
						allowBare: true,
						label:     fmt.Sprintf(localize.Text("результат метода %q", "return value of method %q"), method.Name),
						methodSig: true,
					})
				}
			}
		}
	}
}

func (state *analysis) validateUnusedImports() {
	used := make(map[string]struct{})
	for _, item := range state.file.Types() {
		if item == nil {
			continue
		}
		collectImportUse(used, item.Type)
	}
	for _, model := range state.file.Models() {
		if model == nil {
			continue
		}

		for _, method := range model.Methods {
			for _, param := range method.Params {
				collectImportUse(used, param.Type)
			}
			for _, result := range method.Returns {
				collectImportUse(used, result)
			}
		}

		for _, field := range model.Fields {
			if field == nil {
				continue
			}
			collectImportUse(used, field.Type)
			collectImportUseFromExpr(used, state.imports, field.Default)
			for _, method := range field.Methods {
				for _, param := range method.Params {
					collectImportUse(used, param.Type)
				}
				for _, result := range method.Returns {
					collectImportUse(used, result)
				}
			}
		}
	}

	for alias, spec := range state.imports {
		if _, ok := used[alias]; ok {
			continue
		}

		state.add(newProblem(
			spec.Position,
			fmt.Sprintf(localize.Text("неиспользуемый импорт %q", "unused import %q"), spec.Path),
		).WithHint(localize.Text(
			"Удалите импорт или используйте тип из него в моделях или сигнатурах методов.",
			"Remove the import or use a type from it in model fields or method signatures.",
		)))
	}
}

func (state *analysis) validateGeneratedNameCollisions() {
	renderer, ok := targetpkg.Lookup(targetpkg.NormalizeLanguage(state.file.TargetLang()))
	if !ok {
		renderer, _ = targetpkg.Lookup(targetpkg.DefaultLanguage)
	}

	fileOwners := make(map[string]tokenOwner)
	identOwners := make(map[string]tokenOwner)

	for _, model := range state.file.Models() {
		if model == nil {
			continue
		}

		if renderer != nil {
			layout := targetpkg.ResolveModelLayout(renderer, model.Name)
			state.claimFile(fileOwners, layout.MainRelative, model.Position, model.Name)
			if strings.TrimSpace(layout.FacadeRelative) != "" {
				state.claimFile(fileOwners, layout.FacadeRelative, model.Position, model.Name)
			}
		}

		state.claimIdentifier(identOwners, model.Name, model.Position, model.Name)
	}
	if types := state.file.Types(); len(types) > 0 {
		position := token.Position{}
		if types[0] != nil {
			position = types[0].Position
		}
		state.claimFile(fileOwners, "types.gen.go", position, "__types__")
		state.claimFile(fileOwners, path.Join("types", "types.gen.go"), position, "__types__")
	}
	for _, item := range state.file.Types() {
		if item == nil {
			continue
		}

		state.claimIdentifier(identOwners, item.Name, item.Position, item.Name)
	}
}

func (state *analysis) validateFieldDefault(field *ast.FieldAST) {
	if field == nil || field.Default == nil {
		return
	}
	resolvedType := state.resolveTypeRef(field.Type)
	if modelFieldRef, ok := findModelFieldReferenceInDefault(field.Default, state.models); ok {
		state.add(newProblem(
			field.Position,
			fmt.Sprintf(localize.Text("default поля %q не может ссылаться на поле модели %q", "default for field %q cannot reference model field %q"), field.Name, modelFieldRef),
		).WithHint(fmt.Sprintf(localize.Text(
			"Если вы хотели взять тип поля, пишите `%s %s`, а не `%s = %s`.",
			"If you meant the field type, write `%s %s` instead of `%s = %s`.",
		), field.Name, modelFieldRef, field.Name, modelFieldRef)).WithDetail(localize.Text(
			"В default разрешены Go-выражения, но без обращений к DSL-моделям через `Model.Field`.",
			"Go expressions are allowed in defaults, but they must not reference DSL models through `Model.Field`.",
		)))
		return
	}

	if isNilExpr(field.Default) {
		if resolvedType.Optional {
			return
		}

		state.add(newProblem(
			field.Position,
			fmt.Sprintf(localize.Text("поле %q не может иметь default `nil`, потому что оно не optional", "field %q cannot use default `nil` because it is not optional"), field.Name),
		))
		return
	}

	switch value := field.Default.(type) {
	case ast.StringExpr:
		if resolvedType.IsString() {
			return
		}
		state.add(newProblem(
			field.Position,
			fmt.Sprintf(localize.Text("string default несовместим с типом поля %q", "string default is incompatible with field type %q"), field.Type.Name.String()),
		))
	case ast.BoolExpr:
		if resolvedType.IsBool() {
			return
		}
		state.add(newProblem(
			field.Position,
			fmt.Sprintf(localize.Text("bool default несовместим с типом поля %q", "bool default is incompatible with field type %q"), field.Type.Name.String()),
		))
	case ast.NumberExpr:
		if resolvedType.IsNumeric() {
			_ = value
			return
		}
		state.add(newProblem(
			field.Position,
			fmt.Sprintf(localize.Text("numeric default несовместим с типом поля %q", "numeric default is incompatible with field type %q"), field.Type.Name.String()),
		))
	case ast.NameExpr:
		if err := validateGoDefaultExprSyntax(value.Name.String()); err != nil {
			state.add(newProblem(
				field.Position,
				fmt.Sprintf(localize.Text("некорректное Go-выражение в default поля %q", "invalid Go expression in default for field %q"), field.Name),
			).WithDetail(err.Error()).WithHint(localize.Text(
				"После `=` можно писать любое валидное Go-выражение.",
				"After `=` you can write any valid Go expression.",
			)))
		}
	case ast.GoExpr:
		if err := validateGoDefaultExprSyntax(value.Text); err != nil {
			state.add(newProblem(
				field.Position,
				fmt.Sprintf(localize.Text("некорректное Go-выражение в default поля %q", "invalid Go expression in default for field %q"), field.Name),
			).WithDetail(err.Error()).WithHint(localize.Text(
				"После `=` можно писать любое валидное Go-выражение.",
				"After `=` you can write any valid Go expression.",
			)))
		}
	default:
		state.add(newProblem(
			field.Position,
			fmt.Sprintf(localize.Text("default-значение поля %q пока не поддерживается для типа %q", "default value on field %q is not supported yet for type %q"), field.Name, field.Type.Name.String()),
		).WithHint(localize.Text(
			"Пока используйте литералы string, bool, number или `nil` для optional-полей.",
			"For now use string, bool, number literals or `nil` for optional fields.",
		)))
	}
}

func (state *analysis) validateTypeRef(position token.Position, typeRef ast.TypeRef, ctx typeContext) {
	name := strings.TrimSpace(typeRef.Name.String())
	if name == "" {
		return
	}

	switch classifyType(typeRef, state.imports, state.models, state.types) {
	case typeBuiltin, typeModel, typeExternal:
		return
	case typeTime:
		return
	case typeCyclicAlias:
		state.add(newProblem(
			position,
			fmt.Sprintf(localize.Text("%s использует циклический type alias %q", "%s uses a cyclic type alias %q"), ctx.label, name),
		).WithHint(localize.Text(
			"Разорвите цикл между type aliases или замените один из них на конкретный тип.",
			"Break the cycle between type aliases or replace one of them with a concrete type.",
		)))
	case typeUnknownModel:
		if _, ok := state.groupCandidates[name]; ok {
			return
		}
		state.add(newProblem(
			position,
			fmt.Sprintf(localize.Text("%s ссылается на несуществующую модель %q", "%s references a missing model %q"), ctx.label, name),
		).WithHint(localize.Text(
			"Добавьте модель с таким именем, импортируйте её из `.rpl` или исправьте тип.",
			"Add a model with that name, import it from `.rpl`, or fix the type.",
		)))
	case typeUnknownModelField:
		state.add(newProblem(
			position,
			fmt.Sprintf(localize.Text("%s ссылается на несуществующее поле модели %q", "%s references a missing model field %q"), ctx.label, name),
		).WithHint(localize.Text(
			"Используйте форму `Model.Field` только для реально существующего поля модели.",
			"Use the `Model.Field` form only for a real field that exists on the model.",
		)))
	case typeUnresolvedExternal:
		message := localize.Text("%s использует неразрешимый внешний Go-тип %q", "%s uses an unresolved external Go type %q")
		if !ctx.methodSig {
			message = localize.Text("%s использует неразрешимый импортированный тип %q", "%s uses an unresolved imported type %q")
		}
		state.add(newProblem(
			position,
			fmt.Sprintf(message, ctx.label, name),
		).WithHint(localize.Text(
			"Проверьте alias в `import (...)` и имя типа. Для внешних типов используйте форму вроде `package.Var`.",
			"Check the alias in `import (...)` and the type name. For external types use a form like `package.Var`.",
		)))
	}
}

func (state *analysis) validateTypeAliasesRaw() {
	for _, item := range state.file.Types() {
		if item == nil {
			continue
		}

		kind := classifyType(item.Type, state.imports, state.models, state.types)
		if kind == typeModel {
			state.add(newProblem(
				item.Position,
				fmt.Sprintf(localize.Text("type %q не может быть alias на model %q", "type %q cannot alias model %q"), item.Name, item.Type.Name.String()),
			).WithHint(localize.Text(
				"Используйте type aliases для scalar-типов, импортированных Go-типов или ссылок вида `Model.Field`, которые разворачиваются в конкретный тип поля.",
				"Use type aliases for scalar types, imported Go types, or `Model.Field` references that resolve to a concrete field type.",
			)))
		}
	}
}

func (state *analysis) validateAttr(model *ast.ModelAST, field *ast.FieldAST, method *ast.FieldMethodAST, attr ast.Attr, where scope) {
	_ = model
	_ = field
	_ = method
	namespace := attrNamespace(attr)
	if namespace == "" {
		state.add(newProblem(attr.Position, localize.Text("атрибут без имени недопустим", "attribute without a name is not allowed")))
		return
	}

	if _, ok := generalAttrNamespaces[namespace]; ok {
		state.validateGeneralAttr(attr, where)
		return
	}

	if _, declared := state.declaredRuntime[namespace]; declared {
		return
	}

	state.add(newProblem(
		attr.Position,
		fmt.Sprintf(localize.Text("namespace attr %q не объявлен в attrs (...)", "attr namespace %q is not declared in attrs (...)"), namespace),
	).WithHint(localize.Text(
		"Объявите attr в `attrs (...)` или исправьте namespace атрибута.",
		"Declare the attr in `attrs (...)` or fix the attribute namespace.",
	)))
}

func (state *analysis) validateGeneralAttr(attr ast.Attr, where scope) {
	switch attrNamespace(attr) {
	case "group":
		if where != scopeField {
			state.add(scopeProblem(attr.Position, where, attr, scopeField))
		}
		validateSingleStringAttr(state, attr, map[string]struct{}{"group": {}, "name": {}})
	case "comment":
		if where != scopeModel && where != scopeField {
			state.add(scopeProblem(attr.Position, where, attr, scopeModel, scopeField))
		}
		validateSingleStringAttr(state, attr, map[string]struct{}{"text": {}, "comment": {}})
	case "ignore":
		if where != scopeField {
			state.add(scopeProblem(attr.Position, where, attr, scopeField))
		}
		if len(attr.NamedArgs) > 0 {
			state.add(newProblem(
				attr.Position,
				localize.Text("@ignore(...) принимает только позиционные runtime-имена", "@ignore(...) only accepts positional runtime names"),
			))
		}
		for _, expr := range attr.Args {
			if !isStringLikeExpr(expr) {
				state.add(newProblem(attr.Position, localize.Text("@ignore(...) принимает только строки", "@ignore(...) only accepts strings")))
			}
		}
		if len(attr.Args) == 0 {
			state.add(newProblem(attr.Position, localize.Text("@ignore(...) требует хотя бы один runtime", "@ignore(...) requires at least one runtime name")))
		}
	}
}

func modelIndex(file *ast.File) map[string]*ast.ModelAST {
	index := make(map[string]*ast.ModelAST)
	for _, model := range file.Models() {
		if model == nil {
			continue
		}
		index[strings.TrimSpace(model.Name)] = model
	}
	return index
}

func typeIndex(file *ast.File) map[string]*ast.TypeAliasAST {
	index := make(map[string]*ast.TypeAliasAST)
	for _, item := range file.Types() {
		if item == nil {
			continue
		}
		index[strings.TrimSpace(item.Name)] = item
	}
	return index
}

func importAliasIndex(file *ast.File) map[string]ast.ImportSpec {
	index := make(map[string]ast.ImportSpec)
	for _, importNode := range file.Imports() {
		if importNode == nil {
			continue
		}
		for _, spec := range importNode.Specs {
			alias := importAlias(spec)
			if alias == "" {
				continue
			}
			index[alias] = spec
		}
	}
	return index
}

func runtimeIndex(file *ast.File) map[string]ast.RuntimeSpec {
	index := make(map[string]ast.RuntimeSpec)
	for _, runtimeBlock := range file.Runtimes() {
		if runtimeBlock == nil {
			continue
		}
		for _, spec := range runtimeBlock.Specs {
			if strings.TrimSpace(spec.Name) == "" {
				continue
			}
			index[spec.Name] = spec
		}
	}
	return index
}

func groupCandidateIndex(file *ast.File) map[string]struct{} {
	candidates := make(map[string]struct{})

	for _, model := range file.Models() {
		if model == nil || strings.TrimSpace(model.GeneratedFrom) != "" {
			continue
		}

		for _, field := range model.Fields {
			if field == nil {
				continue
			}

			groupAttr, ok := field.FindAttr("group")
			if !ok {
				continue
			}

			groupName := strings.TrimSpace(firstGroupAttrString(groupAttr))
			if groupName == "" {
				continue
			}

			candidates[groupCandidateName(model.Name, groupName)] = struct{}{}
		}
	}

	return candidates
}

func firstGroupAttrString(attr *ast.Attr) string {
	if attr == nil {
		return ""
	}
	if len(attr.Args) > 0 {
		return ast.ExprString(attr.Args[0])
	}
	for _, item := range attr.NamedArgs {
		if item.Name == "group" || item.Name == "name" {
			return ast.ExprString(item.Value)
		}
	}
	return ""
}

func groupCandidateName(base string, group string) string {
	return strings.TrimSpace(base) + groupSuffix(group)
}

func groupSuffix(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	upperNext := true
	for _, item := range trimmed {
		switch {
		case unicode.IsLetter(item) || unicode.IsDigit(item):
			if upperNext {
				builder.WriteRune(unicode.ToUpper(item))
				upperNext = false
			} else {
				builder.WriteRune(item)
			}
		default:
			upperNext = true
		}
	}

	if builder.Len() == 0 {
		return "Group"
	}
	return builder.String()
}

func importAlias(spec ast.ImportSpec) string {
	if strings.TrimSpace(spec.Alias) != "" {
		return strings.TrimSpace(spec.Alias)
	}
	return path.Base(strings.TrimSpace(spec.Path))
}

func collectImportUse(used map[string]struct{}, typeRef ast.TypeRef) {
	if len(typeRef.Name.Parts) < 2 {
		return
	}
	used[typeRef.Name.Parts[0]] = struct{}{}
}

func collectImportUseFromExpr(used map[string]struct{}, imports map[string]ast.ImportSpec, expr ast.Expr) {
	if expr == nil {
		return
	}

	switch value := expr.(type) {
	case ast.NameExpr:
		if len(value.Name.Parts) < 2 {
			return
		}
		alias := strings.TrimSpace(value.Name.Parts[0])
		if _, ok := imports[alias]; ok {
			used[alias] = struct{}{}
		}
	case ast.GoExpr:
		parsed, err := goparser.ParseExpr(value.Text)
		if err != nil {
			return
		}
		goast.Inspect(parsed, func(node goast.Node) bool {
			selector, ok := node.(*goast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*goast.Ident)
			if !ok {
				return true
			}
			alias := strings.TrimSpace(ident.Name)
			if _, ok := imports[alias]; ok {
				used[alias] = struct{}{}
			}
			return true
		})
	}
}

func findModelFieldReferenceInDefault(expr ast.Expr, models map[string]*ast.ModelAST) (string, bool) {
	if expr == nil || len(models) == 0 {
		return "", false
	}

	switch value := expr.(type) {
	case ast.NameExpr:
		return findExistingModelFieldReference(value.Name.Parts, models)
	case ast.GoExpr:
		parsed, err := goparser.ParseExpr(value.Text)
		if err != nil {
			return "", false
		}

		var match string
		goast.Inspect(parsed, func(node goast.Node) bool {
			selector, ok := node.(*goast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*goast.Ident)
			if !ok {
				return true
			}

			ref, ok := findExistingModelFieldReference([]string{ident.Name, selector.Sel.Name}, models)
			if ok {
				match = ref
				return false
			}
			return true
		})

		if strings.TrimSpace(match) != "" {
			return match, true
		}
	}

	return "", false
}

func findExistingModelFieldReference(parts []string, models map[string]*ast.ModelAST) (string, bool) {
	if len(parts) != 2 {
		return "", false
	}

	modelName := strings.TrimSpace(parts[0])
	fieldName := strings.TrimSpace(parts[1])
	if modelName == "" || fieldName == "" {
		return "", false
	}

	model, ok := models[modelName]
	if !ok || model == nil {
		return "", false
	}
	if _, ok := model.FindField(fieldName); !ok {
		return "", false
	}

	return modelName + "." + fieldName, true
}

func attrNamespace(attr ast.Attr) string {
	identifier := strings.TrimSpace(attr.Identifier())
	if identifier == "" {
		return ""
	}
	parts := strings.Split(identifier, ".")
	return parts[0]
}

func attrSubName(attr ast.Attr) string {
	identifier := strings.TrimSpace(attr.Identifier())
	if identifier == "" {
		return ""
	}
	parts := strings.Split(identifier, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func validateSingleStringAttr(state *analysis, attr ast.Attr, allowedNamed map[string]struct{}) {
	if len(attr.NamedArgs) > 0 {
		for _, item := range attr.NamedArgs {
			if _, ok := allowedNamed[item.Name]; !ok {
				state.add(newProblem(item.Position, fmt.Sprintf(localize.Text("неизвестный аргумент attr %q", "unknown attr argument %q"), item.Name)))
				continue
			}
			if !isStringLikeExpr(item.Value) {
				state.add(newProblem(item.Position, fmt.Sprintf(localize.Text("аргумент %q должен быть строкой", "argument %q must be a string"), item.Name)))
			}
		}
	}
	for _, expr := range attr.Args {
		if !isStringLikeExpr(expr) {
			state.add(newProblem(attr.Position, localize.Text("атрибут принимает только строковые аргументы", "attribute only accepts string arguments")))
		}
	}
	if len(attr.Args) > 1 || len(attr.NamedArgs) > 1 {
		state.add(newProblem(attr.Position, localize.Text("атрибут принимает только одно значение", "attribute accepts only one value")))
	}
}

func isStringLikeExpr(expr ast.Expr) bool {
	switch expr.(type) {
	case ast.StringExpr, ast.NameExpr:
		return true
	default:
		return false
	}
}

func isNilExpr(expr ast.Expr) bool {
	switch value := expr.(type) {
	case ast.NameExpr:
		return strings.TrimSpace(value.Name.String()) == "nil"
	case ast.GoExpr:
		return strings.TrimSpace(value.Text) == "nil"
	default:
		return false
	}
}

func validateGoDefaultExprSyntax(expr string) error {
	if strings.TrimSpace(expr) == "" {
		return errors.New(localize.Text("пустое Go-выражение", "empty Go expression"))
	}
	_, err := goparser.ParseExpr(expr)
	return err
}

func scopeProblem(position token.Position, where scope, attr ast.Attr, allowed ...scope) error {
	items := make([]string, 0, len(allowed))
	for _, item := range allowed {
		items = append(items, string(item))
	}
	return newProblem(
		position,
		fmt.Sprintf(localize.Text("attr %q нельзя использовать в %s", "attr %q cannot be used on %s"), attr.Identifier(), where),
	).WithDetail(fmt.Sprintf(localize.Text(
		"Разрешённые области: %s.",
		"Allowed scopes: %s.",
	), strings.Join(items, ", ")))
}

type typeKind string

const (
	typeBuiltin            typeKind = "builtin"
	typeExternal           typeKind = "external"
	typeModel              typeKind = "model"
	typeTime               typeKind = "time"
	typeCyclicAlias        typeKind = "cyclic-alias"
	typeUnknownModel       typeKind = "unknown-model"
	typeUnknownModelField  typeKind = "unknown-model-field"
	typeUnresolvedExternal typeKind = "unresolved-external"
)

func classifyType(typeRef ast.TypeRef, imports map[string]ast.ImportSpec, models map[string]*ast.ModelAST, types map[string]*ast.TypeAliasAST) typeKind {
	if resolved, ok, missingField, cyclic := resolveTypeAliasRef(typeRef, models, types); ok {
		if cyclic {
			return typeCyclicAlias
		}
		if missingField {
			return typeUnknownModelField
		}
		return classifyResolvedType(resolved, imports, models, types)
	}

	if resolved, ok, missingField := resolveModelFieldTypeRef(typeRef, models); ok {
		if missingField {
			return typeUnknownModelField
		}
		return classifyResolvedType(resolved, imports, models, types)
	}

	return classifyResolvedType(typeRef, imports, models, types)
}

func classifyResolvedType(typeRef ast.TypeRef, imports map[string]ast.ImportSpec, models map[string]*ast.ModelAST, types map[string]*ast.TypeAliasAST) typeKind {
	name := strings.TrimSpace(typeRef.Name.String())
	if name == "" {
		return typeBuiltin
	}

	if len(typeRef.Name.Parts) == 1 {
		base := typeRef.Name.Parts[0]
		switch base {
		case "any", "string", "bool", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "byte", "float32", "float64", "error":
			return typeBuiltin
		}
		if _, ok := types[base]; ok {
			if resolved, ok, missingField, cyclic := resolveTypeAliasRef(typeRef, models, types); ok {
				if cyclic {
					return typeCyclicAlias
				}
				if missingField {
					return typeUnknownModelField
				}
				return classifyResolvedType(resolved, imports, models, types)
			}
		}
		if _, ok := models[base]; ok {
			return typeModel
		}
		return typeUnknownModel
	}

	alias := typeRef.Name.Parts[0]
	spec, ok := imports[alias]
	if !ok {
		if len(typeRef.Name.Parts) == 2 && looksLikeModelReference(alias) {
			return typeUnknownModelField
		}
		return typeUnresolvedExternal
	}
	if spec.Path == "time" && typeRef.Name.Parts[len(typeRef.Name.Parts)-1] == "Time" {
		return typeTime
	}
	return typeExternal
}

func resolveModelFieldTypeRef(typeRef ast.TypeRef, models map[string]*ast.ModelAST) (ast.TypeRef, bool, bool) {
	if len(typeRef.Name.Parts) != 2 {
		return ast.TypeRef{}, false, false
	}

	modelName := strings.TrimSpace(typeRef.Name.Parts[0])
	fieldName := strings.TrimSpace(typeRef.Name.Parts[1])
	model, ok := models[modelName]
	if !ok || model == nil {
		return ast.TypeRef{}, false, false
	}

	field, ok := model.FindField(fieldName)
	if !ok || field == nil {
		return ast.TypeRef{}, true, true
	}

	resolved := field.Type
	if next, ok, missingField := resolveModelFieldTypeRef(resolved, models); ok {
		resolved = next
		if missingField {
			return ast.TypeRef{}, true, true
		}
	}

	resolved.IsList = resolved.IsList || typeRef.IsList
	resolved.Optional = resolved.Optional || typeRef.Optional
	return resolved, true, false
}

func resolveTypeAliasRef(typeRef ast.TypeRef, models map[string]*ast.ModelAST, types map[string]*ast.TypeAliasAST) (ast.TypeRef, bool, bool, bool) {
	return resolveTypeAliasRefSeen(typeRef, models, types, make(map[string]struct{}))
}

func resolveTypeAliasRefSeen(typeRef ast.TypeRef, models map[string]*ast.ModelAST, types map[string]*ast.TypeAliasAST, seen map[string]struct{}) (ast.TypeRef, bool, bool, bool) {
	if resolved, ok, missingField := resolveModelFieldTypeRef(typeRef, models); ok {
		if missingField {
			return ast.TypeRef{}, true, true, false
		}
		typeRef = resolved
	}

	if len(typeRef.Name.Parts) != 1 {
		return ast.TypeRef{}, false, false, false
	}

	name := strings.TrimSpace(typeRef.Name.String())
	alias, ok := types[name]
	if !ok || alias == nil {
		return ast.TypeRef{}, false, false, false
	}
	if _, exists := seen[name]; exists {
		return ast.TypeRef{}, true, false, true
	}
	seen[name] = struct{}{}

	resolved := alias.Type
	if next, ok, missingField, cyclic := resolveTypeAliasRefSeen(resolved, models, types, seen); ok {
		if missingField || cyclic {
			return ast.TypeRef{}, true, missingField, cyclic
		}
		resolved = next
	}

	resolved.IsList = resolved.IsList || typeRef.IsList
	resolved.Optional = resolved.Optional || typeRef.Optional
	return resolved, true, false, false
}

func (state *analysis) resolveTypeRef(typeRef ast.TypeRef) ast.TypeRef {
	if resolved, ok, missingField, cyclic := resolveTypeAliasRef(typeRef, state.models, state.types); ok && !missingField && !cyclic {
		return resolved
	}
	if resolved, ok, missingField := resolveModelFieldTypeRef(typeRef, state.models); ok && !missingField {
		return resolved
	}
	return typeRef
}

func looksLikeModelReference(name string) bool {
	for _, char := range strings.TrimSpace(name) {
		return unicode.IsUpper(char)
	}

	return false
}

func newProblem(position token.Position, message string) *Err.Error {
	return Err.New(strings.TrimSpace(message)).
		WithLocation(position.File, position.Line, position.Column)
}

type tokenOwner struct {
	name     string
	position token.Position
}

func (state *analysis) claimFile(owners map[string]tokenOwner, fileName string, position token.Position, owner string) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return
	}
	if previous, ok := owners[fileName]; ok {
		state.add(newProblem(
			position,
			fmt.Sprintf(localize.Text("collision в generated filename %q", "collision in generated filename %q"), fileName),
		).WithDetail(fmt.Sprintf(localize.Text(
			"Имя уже занято моделью %q в %s.",
			"The name is already used by model %q at %s.",
		), previous.name, positionLabel(previous.position))))
		return
	}
	owners[fileName] = tokenOwner{name: owner, position: position}
}

func (state *analysis) claimIdentifier(owners map[string]tokenOwner, identifier string, position token.Position, owner string) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return
	}
	if previous, ok := owners[identifier]; ok {
		state.add(newProblem(
			position,
			fmt.Sprintf(localize.Text("collision top-level helper name %q в корневом фасадном пакете", "top-level helper name %q collides in the root facade package"), identifier),
		).WithDetail(fmt.Sprintf(localize.Text(
			"Имя уже занято моделью %q в %s.",
			"The name is already used by model %q at %s.",
		), previous.name, positionLabel(previous.position))))
		return
	}
	owners[identifier] = tokenOwner{name: owner, position: position}
}

func positionLabel(position token.Position) string {
	if position.Line > 0 && position.Column > 0 {
		return fmt.Sprintf("%s:%d:%d", position.File, position.Line, position.Column)
	}
	if position.Line > 0 {
		return fmt.Sprintf("%s:%d", position.File, position.Line)
	}
	return position.File
}

type typeContext struct {
	model     *ast.ModelAST
	field     *ast.FieldAST
	method    *ast.FieldMethodAST
	allowBare bool
	label     string
	methodSig bool
}
