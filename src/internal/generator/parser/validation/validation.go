package validation

import (
	"fmt"
	"rpl/internal/generator/parser/ast"
	Err "rpl/pkg/error"
	"rpl/pkg/error/localize"
)

func ValidationAST(file *ast.File) error {
	if file == nil {
		return nil
	}

	if err := validatePackages(file); err != nil {
		return err
	}

	if err := validateImports(file); err != nil {
		return err
	}

	if err := validateModels(file); err != nil {
		return err
	}

	if err := validateTypes(file); err != nil {
		return err
	}

	return nil
}

func validatePackages(file *ast.File) error {
	packages := file.Packages()
	if len(packages) <= 1 {
		return nil
	}

	item := packages[1]
	return Err.New(localize.Text("package-директива может быть объявлена только один раз в файле", "package directive may only be declared once per file")).
		WithLocation(item.Position.File, item.Position.Line, item.Position.Column).
		WithHint(localize.Text("Оставьте только один `package <name>` в начале файла.", "Keep only one `package <name>` directive at the top of the file."))
}

func validateImports(file *ast.File) error {
	imports := file.Imports()
	if len(imports) == 0 {
		return nil
	}

	aliases := make(map[string]struct{})
	for _, importNode := range imports {
		if importNode == nil {
			continue
		}

		for i := range importNode.Specs {
			spec := importNode.Specs[i]
			alias := spec.Alias
			if alias == "" {
				alias = spec.Path
			}

			if _, exists := aliases[alias]; exists {
				return Err.Newf(
					localize.Text("дублирующийся алиас импорта %q", "duplicate import alias %q"),
					alias,
				).
					WithLocation(spec.Position.File, spec.Position.Line, spec.Position.Column).
					WithHint(localize.Text("Укажите другой алиас или оставьте только один импорт с этим именем.", "Use a different alias or keep only one import with that name."))
			}

			aliases[alias] = struct{}{}
		}
	}

	return nil
}

func validateModels(file *ast.File) error {
	models := file.Models()
	if len(models) == 0 {
		return nil
	}

	names := make(map[string]struct{})
	for _, model := range models {
		if model == nil {
			continue
		}

		if _, exists := names[model.Name]; exists {
			return Err.Newf(
				localize.Text("дублирующееся имя модели %q", "duplicate model name %q"),
				model.Name,
			).
				WithLocation(model.Position.File, model.Position.Line, model.Position.Column).
				WithDetail(fmt.Sprintf(localize.Text("Модель %q уже была объявлена выше в этом наборе файлов.", "Model %q was already declared earlier in this file set."), model.Name)).
				WithHint(localize.Text("Переименуйте одну из моделей или вынесите общую модель в отдельный импортируемый файл.", "Rename one of the models or move the shared model into a separate imported file."))
		}

		names[model.Name] = struct{}{}
	}

	return nil
}

func validateTypes(file *ast.File) error {
	types := file.Types()
	if len(types) == 0 {
		return nil
	}

	models := make(map[string]struct{})
	for _, model := range file.Models() {
		if model == nil {
			continue
		}
		models[model.Name] = struct{}{}
	}

	names := make(map[string]struct{})
	for _, item := range types {
		if item == nil {
			continue
		}

		if _, exists := names[item.Name]; exists {
			return Err.Newf(
				localize.Text("дублирующееся имя типа %q", "duplicate type name %q"),
				item.Name,
			).
				WithLocation(item.Position.File, item.Position.Line, item.Position.Column).
				WithHint(localize.Text("Переименуйте один из type aliases.", "Rename one of the type aliases."))
		}

		if _, exists := models[item.Name]; exists {
			return Err.Newf(
				localize.Text("имя типа %q конфликтует с моделью", "type name %q conflicts with an existing model"),
				item.Name,
			).
				WithLocation(item.Position.File, item.Position.Line, item.Position.Column).
				WithHint(localize.Text("У type alias и model должны быть разные имена.", "A type alias and a model must use different names."))
		}

		names[item.Name] = struct{}{}
	}

	return nil
}
