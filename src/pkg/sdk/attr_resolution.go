package sdk

import "strings"

const (
	AttrOriginScopeUnknown        = "unknown"
	AttrOriginScopeModel          = "model"
	AttrOriginScopeField          = "field"
	AttrOriginScopeFieldExtension = "field_extension"
	AttrOriginScopeMethod         = "method"
)

type AttrOrigin struct {
	Scope  string `json:"scope,omitempty"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

type DiagnosticLoc interface {
	DiagnosticOrigin() AttrOrigin
}

type ResolvedValue struct {
	Name    string
	Value   Value
	Origins []AttrOrigin
}

type AttrConflict struct {
	Name           string
	Previous       Value
	PreviousOrigin AttrOrigin
	Current        Value
	CurrentOrigin  AttrOrigin
}

type ResolvedAttr struct {
	Identifier string
	Namespace  string
	Attrs      []Attr
	Values     map[string]ResolvedValue
	Conflicts  []AttrConflict
}

func (origin AttrOrigin) IsZero() bool {
	return strings.TrimSpace(origin.Scope) == "" && strings.TrimSpace(origin.Path) == "" && origin.Line == 0 && origin.Column == 0
}

func (attr Attr) DiagnosticOrigin() AttrOrigin {
	return attr.Origin
}

func (item NamedValue) DiagnosticOrigin() AttrOrigin {
	return item.Origin
}

func (value Value) TypeName() string {
	switch strings.TrimSpace(value.Kind) {
	case "bool":
		return "bool"
	case "number":
		return "number"
	case "string":
		return "string"
	case "name":
		return "name"
	case "":
		return "unknown"
	default:
		return strings.TrimSpace(value.Kind)
	}
}

func (value Value) Equal(other Value) bool {
	return strings.TrimSpace(value.Kind) == strings.TrimSpace(other.Kind) && strings.TrimSpace(value.Text) == strings.TrimSpace(other.Text) && value.Bool == other.Bool
}

func (resolved ResolvedAttr) Value(name string) (Value, bool) {
	item, ok := resolved.Values[strings.TrimSpace(name)]
	if !ok {
		return Value{}, false
	}

	return item.Value, true
}

func (resolved ResolvedAttr) ValueMap() map[string]Value {
	items := make(map[string]Value, len(resolved.Values))
	for name, item := range resolved.Values {
		items[name] = item.Value
	}

	return items
}

func (resolved ResolvedAttr) ValueOrigins(name string) []AttrOrigin {
	item, ok := resolved.Values[strings.TrimSpace(name)]
	if !ok {
		return nil
	}

	return append([]AttrOrigin(nil), item.Origins...)
}

func (resolved ResolvedAttr) Origins() []AttrOrigin {
	origins := make([]AttrOrigin, 0)
	for _, attr := range resolved.Attrs {
		origins = appendOriginUnique(origins, attr.Origin)
		for _, item := range attr.NamedArgs {
			origins = appendOriginUnique(origins, item.Origin)
		}
	}

	return origins
}

func (model Model) ResolvedAttr(identifier string) (ResolvedAttr, bool) {
	return resolvePreferredAttrs(model.RuntimeAttrs, model.Attrs, identifier)
}

func (model Model) ResolvedValues(identifier string) map[string]Value {
	resolved, ok := model.ResolvedAttr(identifier)
	if !ok {
		return map[string]Value{}
	}

	return resolved.ValueMap()
}

func (model Model) AttrOrigins(identifier string) []AttrOrigin {
	resolved, ok := model.ResolvedAttr(identifier)
	if !ok {
		return nil
	}

	return resolved.Origins()
}

func (model Model) Conflicts(identifier string) []AttrConflict {
	resolved, ok := model.ResolvedAttr(identifier)
	if !ok {
		return nil
	}

	return append([]AttrConflict(nil), resolved.Conflicts...)
}

func (field Field) ResolvedAttr(identifier string) (ResolvedAttr, bool) {
	return resolvePreferredAttrs(field.RuntimeAttrs, field.Attrs, identifier)
}

func (field Field) ResolvedValues(identifier string) map[string]Value {
	resolved, ok := field.ResolvedAttr(identifier)
	if !ok {
		return map[string]Value{}
	}

	return resolved.ValueMap()
}

func (field Field) AttrOrigins(identifier string) []AttrOrigin {
	resolved, ok := field.ResolvedAttr(identifier)
	if !ok {
		return nil
	}

	return resolved.Origins()
}

func (field Field) Conflicts(identifier string) []AttrConflict {
	resolved, ok := field.ResolvedAttr(identifier)
	if !ok {
		return nil
	}

	return append([]AttrConflict(nil), resolved.Conflicts...)
}

func (method Method) ResolvedAttr(identifier string) (ResolvedAttr, bool) {
	return resolvePreferredAttrs(method.RuntimeAttrs, method.Attrs, identifier)
}

func (method Method) ResolvedValues(identifier string) map[string]Value {
	resolved, ok := method.ResolvedAttr(identifier)
	if !ok {
		return map[string]Value{}
	}

	return resolved.ValueMap()
}

func (method Method) AttrOrigins(identifier string) []AttrOrigin {
	resolved, ok := method.ResolvedAttr(identifier)
	if !ok {
		return nil
	}

	return resolved.Origins()
}

func (method Method) Conflicts(identifier string) []AttrConflict {
	resolved, ok := method.ResolvedAttr(identifier)
	if !ok {
		return nil
	}

	return append([]AttrConflict(nil), resolved.Conflicts...)
}

func ResolveAttrs(attrs []Attr, identifier string) (ResolvedAttr, bool) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return ResolvedAttr{}, false
	}

	resolved := ResolvedAttr{
		Identifier: identifier,
		Namespace:  identifier,
		Attrs:      make([]Attr, 0),
		Values:     make(map[string]ResolvedValue),
		Conflicts:  make([]AttrConflict, 0),
	}

	found := false
	for _, attr := range attrs {
		if !attr.Matches(identifier) && attr.Namespace() != identifier {
			continue
		}

		found = true
		resolved.Attrs = append(resolved.Attrs, attr)
		for _, item := range expandAttrValues(attr) {
			current, exists := resolved.Values[item.Name]
			if !exists {
				resolved.Values[item.Name] = item
				continue
			}

			if !current.Value.Equal(item.Value) {
				previousOrigin := AttrOrigin{}
				if len(current.Origins) > 0 {
					previousOrigin = current.Origins[len(current.Origins)-1]
				}
				currentOrigin := AttrOrigin{}
				if len(item.Origins) > 0 {
					currentOrigin = item.Origins[len(item.Origins)-1]
				}
				resolved.Conflicts = append(resolved.Conflicts, AttrConflict{
					Name:           item.Name,
					Previous:       current.Value,
					PreviousOrigin: previousOrigin,
					Current:        item.Value,
					CurrentOrigin:  currentOrigin,
				})
			}

			current.Value = item.Value
			for _, origin := range item.Origins {
				current.Origins = appendOriginUnique(current.Origins, origin)
			}
			resolved.Values[item.Name] = current
		}
	}

	return resolved, found
}

func resolvePreferredAttrs(runtimeAttrs []Attr, attrs []Attr, identifier string) (ResolvedAttr, bool) {
	if resolved, ok := ResolveAttrs(runtimeAttrs, identifier); ok {
		return resolved, true
	}

	return ResolveAttrs(attrs, identifier)
}

func expandAttrValues(attr Attr) []ResolvedValue {
	values := make([]ResolvedValue, 0, len(attr.NamedArgs)+1)
	subName := strings.TrimSpace(attr.SubName())
	if strings.EqualFold(subName, "inside") {
		subName = "inside"
	}
	if subName != "" {
		value := Value{Kind: "bool", Bool: true}
		if len(attr.Args) > 0 {
			value = attr.Args[0]
		}
		values = append(values, ResolvedValue{
			Name:    subName,
			Value:   value,
			Origins: []AttrOrigin{attr.Origin},
		})
	}

	for _, item := range attr.NamedArgs {
		origin := item.Origin
		if origin.IsZero() {
			origin = attr.Origin
		}
		values = append(values, ResolvedValue{
			Name:    strings.TrimSpace(item.Name),
			Value:   item.Value,
			Origins: []AttrOrigin{origin},
		})
	}

	return values
}

func appendOriginUnique(origins []AttrOrigin, origin AttrOrigin) []AttrOrigin {
	if origin.IsZero() {
		return origins
	}

	for _, item := range origins {
		if item == origin {
			return origins
		}
	}

	return append(origins, origin)
}
