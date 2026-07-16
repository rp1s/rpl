package sdk

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func (model Model) FindField(name string) (*Field, bool) {
	for i := range model.Fields {
		if model.Fields[i].Name == name {
			return &model.Fields[i], true
		}
	}

	return nil, false
}

func (model Model) HasRuntimeAttrs() bool {
	if len(model.RuntimeAttrs) > 0 {
		return true
	}

	for _, field := range model.Fields {
		if len(field.RuntimeAttrs) > 0 {
			return true
		}
		for _, method := range field.Methods {
			if len(method.RuntimeAttrs) > 0 {
				return true
			}
		}
	}
	for _, method := range model.Methods {
		if len(method.RuntimeAttrs) > 0 {
			return true
		}
	}

	return false
}

func (model Model) Attr(identifier string) (Attr, bool) {
	return findAttr(model.Attrs, identifier)
}

func (model Model) RuntimeAttr(identifier string) (Attr, bool) {
	return findAttr(model.RuntimeAttrs, identifier)
}

func (model Model) Comment() string {
	attr, ok := model.Attr("comment")
	if !ok || len(attr.Args) == 0 {
		return ""
	}

	return attr.Args[0].String()
}

func (model Model) Group() string {
	attr, ok := model.Attr("group")
	if !ok || len(attr.Args) == 0 {
		return ""
	}

	return attr.Args[0].String()
}

func (model Model) ActiveFields(runtimeName string) []Field {
	fields := make([]Field, 0, len(model.Fields))
	for _, field := range model.Fields {
		if field.IgnoredBy(runtimeName) {
			continue
		}

		fields = append(fields, field)
	}

	return fields
}

func (model Model) ActiveMethods(runtimeName string) []Method {
	methods := make([]Method, 0, len(model.Methods))
	for _, method := range model.Methods {
		if runtimeName != "" && !method.HasRuntimeAffinity(runtimeName) {
			continue
		}

		methods = append(methods, method)
	}

	return methods
}

func (field Field) Attr(identifier string) (Attr, bool) {
	return findAttr(field.Attrs, identifier)
}

func (field Field) Comment() string {
	attr, ok := field.Attr("comment")
	if !ok || len(attr.Args) == 0 {
		return ""
	}

	return attr.Args[0].String()
}

func (field Field) Group() string {
	attr, ok := field.Attr("group")
	if !ok || len(attr.Args) == 0 {
		return ""
	}

	return attr.Args[0].String()
}

func (field Field) RuntimeAttr(identifier string) (Attr, bool) {
	return findAttr(field.RuntimeAttrs, identifier)
}

func (field Field) GoType() string {
	return field.Type.GoString()
}

func (field Field) Mode(runtimeName string) string {
	for _, attr := range append([]Attr(nil), field.RuntimeAttrs...) {
		if insideModeForAttr(attr, runtimeName) {
			return "inside"
		}
	}

	attr, ok := field.RuntimeAttr(runtimeName)
	if !ok {
		return ""
	}

	if mode := strings.TrimSpace(attr.NamedString("mode")); mode != "" {
		return mode
	}
	if insideModeForAttr(attr, runtimeName) {
		return "inside"
	}

	return ""
}

func (field Field) IgnoredBy(runtimeName string) bool {
	trimmedRuntime := strings.TrimSpace(runtimeName)
	if trimmedRuntime == "" {
		return false
	}

	if attr, ok := field.Attr("ignore"); ok {
		for _, arg := range attr.Args {
			if strings.EqualFold(strings.TrimSpace(arg.String()), trimmedRuntime) {
				return true
			}
		}
	}

	for _, attr := range append(append([]Attr(nil), field.RuntimeAttrs...), field.Attrs...) {
		if attr.Matches(trimmedRuntime + ".ignore") {
			return true
		}

		value, ok := attr.Named("ignore")
		if !ok {
			continue
		}

		if value.Kind == "bool" {
			if value.BoolValue() {
				return true
			}
			continue
		}

		for _, item := range splitRuntimeList(value.String()) {
			if strings.EqualFold(item, trimmedRuntime) {
				return true
			}
		}
	}

	return false
}

func (field Field) IgnoredTargets() []string {
	items := make([]string, 0)
	seen := make(map[string]struct{})

	if attr, ok := field.Attr("ignore"); ok {
		for _, arg := range attr.Args {
			name := strings.TrimSpace(arg.String())
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			items = append(items, name)
		}
	}

	for _, attr := range append(append([]Attr(nil), field.RuntimeAttrs...), field.Attrs...) {
		if value, ok := attr.Named("ignore"); ok {
			for _, item := range splitRuntimeList(value.String()) {
				if _, exists := seen[item]; exists {
					continue
				}
				seen[item] = struct{}{}
				items = append(items, item)
			}
		}
	}

	sort.Strings(items)
	return items
}

func (field Field) HasRuntimeAttr(identifier string) bool {
	_, ok := field.RuntimeAttr(identifier)
	return ok
}

func (field Field) ActiveMethods(runtimeName string) []Method {
	methods := make([]Method, 0, len(field.Methods))
	for _, method := range field.Methods {
		if runtimeName != "" && !method.HasRuntimeAffinity(runtimeName) {
			continue
		}

		methods = append(methods, method)
	}

	return methods
}

func (method Method) RuntimeAttr(identifier string) (Attr, bool) {
	return findAttr(method.RuntimeAttrs, identifier)
}

func (method Method) HasRuntimeAffinity(runtimeName string) bool {
	if strings.TrimSpace(runtimeName) == "" {
		return false
	}
	if len(method.RuntimeAttrs) > 0 {
		return true
	}

	for _, attr := range method.Attrs {
		if attr.Matches(runtimeName) || strings.HasPrefix(attr.Identifier, runtimeName+".") || strings.HasPrefix(attr.Package, runtimeName+".") {
			return true
		}
	}

	return false
}

func (method Method) Mode(runtimeName string) string {
	for _, attr := range append([]Attr(nil), method.RuntimeAttrs...) {
		if insideModeForAttr(attr, runtimeName) {
			return "inside"
		}
	}

	attr, ok := method.RuntimeAttr(runtimeName)
	if !ok {
		return ""
	}

	if mode := strings.TrimSpace(attr.NamedString("mode")); mode != "" {
		return mode
	}
	if insideModeForAttr(attr, runtimeName) {
		return "inside"
	}

	return ""
}

func (attr Attr) Matches(identifier string) bool {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return false
	}

	if attr.Identifier == identifier || attr.Package == identifier || attr.Name == identifier {
		return true
	}

	qualified := strings.Trim(strings.Join([]string{strings.TrimSpace(attr.Package), strings.TrimSpace(attr.Name)}, "."), ".")
	return qualified == identifier
}

func (attr Attr) Namespace() string {
	identifier := strings.TrimSpace(attr.Identifier)
	if identifier == "" {
		return ""
	}

	parts := strings.Split(identifier, ".")
	return parts[0]
}

func (attr Attr) SubName() string {
	identifier := strings.TrimSpace(attr.Identifier)
	if identifier == "" {
		return ""
	}

	parts := strings.Split(identifier, ".")
	if len(parts) < 2 {
		return ""
	}

	return parts[1]
}

func (attr Attr) Named(name string) (Value, bool) {
	for i := range attr.NamedArgs {
		if attr.NamedArgs[i].Name == name {
			return attr.NamedArgs[i].Value, true
		}
	}

	return Value{}, false
}

func (attr Attr) NamedString(name string) string {
	value, ok := attr.Named(name)
	if !ok {
		return ""
	}

	return value.String()
}

func (attr Attr) NamedBool(name string) bool {
	value, ok := attr.Named(name)
	if !ok {
		return false
	}

	return value.BoolValue()
}

func (attr Attr) PositionalStrings() []string {
	if len(attr.Args) == 0 {
		return nil
	}

	items := make([]string, 0, len(attr.Args))
	for _, arg := range attr.Args {
		items = append(items, arg.String())
	}

	return items
}

func (value Value) String() string {
	switch value.Kind {
	case "bool":
		if value.Bool {
			return "true"
		}
		return "false"
	default:
		return value.Text
	}
}

func (value Value) BoolValue() bool {
	if value.Kind == "bool" {
		return value.Bool
	}

	return strings.EqualFold(strings.TrimSpace(value.Text), "true")
}

func (value Value) IsBool() bool {
	return value.Kind == "bool"
}

func (value Value) IsNumber() bool {
	return value.Kind == "number"
}

func (value Value) IsStringLike() bool {
	switch value.Kind {
	case "string", "name":
		return true
	default:
		return false
	}
}

func (value Value) Int64() (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value.Text), 10, 64)
}

func (value Value) Float64() (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value.Text), 64)
}

func (typeRef TypeRef) GoString() string {
	name := strings.TrimSpace(typeRef.Name)
	if name == "" {
		name = "any"
	}

	if typeRef.IsList {
		return "[]" + name
	}

	if typeRef.Optional {
		return "*" + name
	}

	return name
}

func (typeRef TypeRef) BaseName() string {
	name := strings.TrimSpace(typeRef.Name)
	if name == "" {
		return ""
	}

	parts := strings.Split(name, ".")
	return parts[len(parts)-1]
}

func (typeRef TypeRef) IsString() bool {
	return typeRef.BaseName() == "string"
}

func (typeRef TypeRef) IsBool() bool {
	return typeRef.BaseName() == "bool"
}

func (typeRef TypeRef) IsInteger() bool {
	switch typeRef.BaseName() {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "byte":
		return true
	default:
		return false
	}
}

func (typeRef TypeRef) IsFloat() bool {
	switch typeRef.BaseName() {
	case "float32", "float64":
		return true
	default:
		return false
	}
}

func (typeRef TypeRef) IsNumeric() bool {
	return typeRef.IsInteger() || typeRef.IsFloat()
}

func (typeRef TypeRef) IsTime() bool {
	return strings.TrimSpace(typeRef.Name) == "time.Time"
}

func (typeRef TypeRef) IsError() bool {
	return typeRef.BaseName() == "error"
}

func (typeRef TypeRef) IsByte() bool {
	switch typeRef.BaseName() {
	case "byte", "uint8":
		return true
	default:
		return false
	}
}

func (typeRef TypeRef) IsBytes() bool {
	return typeRef.IsList && typeRef.IsByte()
}

func (typeRef TypeRef) IsScalar() bool {
	return !typeRef.IsList && (typeRef.IsString() || typeRef.IsBool() || typeRef.IsNumeric() || typeRef.IsTime())
}

func CollectRuntimeValues(attrs []Attr, runtimeName string) map[string]Value {
	resolved, ok := ResolveAttrs(attrs, runtimeName)
	if !ok {
		return map[string]Value{}
	}

	return resolved.ValueMap()
}

func insideModeForAttr(attr Attr, runtimeName string) bool {
	if runtimeName == "" {
		return false
	}
	if attr.NamedBool("inside") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(attr.NamedString("mode")), "inside") {
		return true
	}
	if attr.Matches(runtimeName+".Inside") || attr.Matches(runtimeName+".inside") {
		return true
	}

	return false
}

func findAttr(items []Attr, identifier string) (Attr, bool) {
	for _, item := range items {
		if item.Matches(identifier) {
			return item, true
		}
	}

	return Attr{}, false
}

func splitRuntimeList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	})

	items := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			items = append(items, trimmed)
		}
	}

	return items
}
