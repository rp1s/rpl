package ast

// Packages returns every package directive declared in the file.
func (file *File) Packages() []*PackageAST {
	if file == nil {
		return nil
	}

	items := make([]*PackageAST, 0)
	for _, node := range file.ASTs {
		item, ok := node.(*PackageAST)
		if ok {
			items = append(items, item)
		}
	}

	return items
}

// Package returns the first package directive, if any.
func (file *File) Package() (*PackageAST, bool) {
	items := file.Packages()
	if len(items) == 0 {
		return nil, false
	}

	return items[0], true
}

// PackageName returns the declared package name.
func (file *File) PackageName() string {
	item, ok := file.Package()
	if !ok || item == nil {
		return ""
	}

	return item.Name
}

// Targets возвращает все target-директивы файла.
func (file *File) Targets() []*TargetAST {
	if file == nil {
		return nil
	}

	targets := make([]*TargetAST, 0)
	for _, node := range file.ASTs {
		target, ok := node.(*TargetAST)
		if ok {
			targets = append(targets, target)
		}
	}

	return targets
}

// Target возвращает первую target-директиву файла.
func (file *File) Target() (*TargetAST, bool) {
	targets := file.Targets()
	if len(targets) == 0 {
		return nil, false
	}

	return targets[0], true
}

// FieldExtensions возвращает все top-level field-блоки файла.
func (file *File) FieldExtensions() []*FieldExtensionAST {
	if file == nil {
		return nil
	}

	extensions := make([]*FieldExtensionAST, 0)
	for _, node := range file.ASTs {
		extension, ok := node.(*FieldExtensionAST)
		if ok {
			extensions = append(extensions, extension)
		}
	}

	return extensions
}

// FieldMethodExtensions возвращает все top-level func Model.Field { ... } блоки.
func (file *File) FieldMethodExtensions() []*FieldMethodsExtensionAST {
	if file == nil {
		return nil
	}

	extensions := make([]*FieldMethodsExtensionAST, 0)
	for _, node := range file.ASTs {
		extension, ok := node.(*FieldMethodsExtensionAST)
		if ok {
			extensions = append(extensions, extension)
		}
	}

	return extensions
}

// ModelMethodExtensions возвращает все top-level func Model { ... } блоки.
func (file *File) ModelMethodExtensions() []*ModelMethodsExtensionAST {
	if file == nil {
		return nil
	}

	extensions := make([]*ModelMethodsExtensionAST, 0)
	for _, node := range file.ASTs {
		extension, ok := node.(*ModelMethodsExtensionAST)
		if ok {
			extensions = append(extensions, extension)
		}
	}

	return extensions
}

// NamedArg возвращает именованный аргумент атрибута по имени.
func (attr *Attr) NamedArg(name string) (Expr, bool) {
	if attr == nil || name == "" {
		return nil, false
	}

	for i := range attr.NamedArgs {
		if attr.NamedArgs[i].Name == name {
			return attr.NamedArgs[i].Value, true
		}
	}

	return nil, false
}

// NamedArg возвращает именованный аргумент target-директивы по имени.
func (target *TargetAST) NamedArg(name string) (Expr, bool) {
	if target == nil || name == "" {
		return nil, false
	}

	for i := range target.NamedArgs {
		if target.NamedArgs[i].Name == name {
			return target.NamedArgs[i].Value, true
		}
	}

	return nil, false
}

// TargetLang возвращает значение target(lang: ...), если оно задано.
func (file *File) TargetLang() string {
	target, ok := file.Target()
	if !ok {
		return ""
	}

	value, ok := target.NamedArg("lang")
	if !ok {
		return ""
	}

	return ExprString(value)
}
