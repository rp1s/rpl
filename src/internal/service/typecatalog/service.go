package typecatalog

import (
	"strings"

	targetpkg "rpl/internal/generator/target"
	"rpl/pkg/error/localize"
	catalogpkg "rpl/pkg/sdk/catalog"
)

// Service returns editor-friendly type and snippet catalogs for target
// languages. It does not validate target support for code generation yet; the
// catalog is intentionally broader so editors can guide modelling work before a
// renderer is fully implemented.
type Service struct{}

func New() *Service {
	return &Service{}
}

func (service *Service) Catalog(lang string) catalogpkg.Catalog {
	_ = NormalizeLanguage(lang)
	return golangCatalog()
}

func NormalizeLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "", "go", "golang":
		return targetpkg.DefaultLanguage
	default:
		return lang
	}
}

func golangCatalog() catalogpkg.Catalog {
	return catalogpkg.Catalog{
		Lang:  targetpkg.DefaultLanguage,
		Label: "Go",
		Help: localize.Text(
			"Базовые типы и шаблоны для Go-таргета RPL.",
			"Core type and structure suggestions for the Go target.",
		),
		Types: []catalogpkg.TypeSpec{
			{Name: "string", Category: "scalar", Help: localize.Text("Обычная строка Go.", "Standard Go string.")},
			{Name: "bool", Category: "scalar", Help: localize.Text("Булево значение.", "Boolean value.")},
			{Name: "byte", Category: "scalar", Help: localize.Text("Один байт. Часто нужен для []byte.", "Single byte value. Often used with []byte.")},
			{Name: "int", Category: "scalar", Help: localize.Text("Платформенный целочисленный тип Go.", "Platform-sized Go integer.")},
			{Name: "int32", Category: "scalar", Help: localize.Text("Фиксированный 32-bit signed integer.", "Fixed 32-bit signed integer.")},
			{Name: "int64", Category: "scalar", Help: localize.Text("Фиксированный 64-bit signed integer.", "Fixed 64-bit signed integer.")},
			{Name: "uint32", Category: "scalar", Help: localize.Text("Фиксированный 32-bit unsigned integer.", "Fixed 32-bit unsigned integer.")},
			{Name: "uint64", Category: "scalar", Help: localize.Text("Фиксированный 64-bit unsigned integer.", "Fixed 64-bit unsigned integer.")},
			{Name: "float32", Category: "scalar", Help: localize.Text("32-bit floating point number.", "32-bit floating point number.")},
			{Name: "float64", Category: "scalar", Help: localize.Text("64-bit floating point number.", "64-bit floating point number.")},
			{Name: "[]byte", Category: "binary", Help: localize.Text("Сырые бинарные данные.", "Raw binary payload.")},
			{Name: "time.Time", Category: "time", Help: localize.Text("Временная отметка для Go, SQL и validate attrs.", "Timestamp type used by Go, SQL, and validate attrs.")},
			{Name: "error", Category: "method", Help: localize.Text("Стандартный Go error для return-сигнатур.", "Standard Go error return type.")},
		},
		Structures: []catalogpkg.StructureSpec{
			{
				Label:    "Type Alias",
				Category: "type",
				Insert:   "type ${1:UserID} ${2:int64}",
				Help: localize.Text(
					"Переиспользуемый alias для доменных идентификаторов и value-objects.",
					"Reusable alias for domain identifiers and value objects.",
				),
			},
			{
				Label:    "Time Model",
				Category: "model",
				Insert:   "model ${1:Event} {\n\tId int64\n\tCreatedAt time.Time\n\tUpdatedAt time.Time?\n}",
				Help: localize.Text(
					"Базовый шаблон модели с временными полями.",
					"Basic model template with timestamp fields.",
				),
			},
			{
				Label:    "Model Method",
				Category: "method",
				Insert:   "func ${1:String} return (${2:string})",
				Help: localize.Text(
					"Сигнатура метода модели без transport-привязки.",
					"Model method signature without transport coupling.",
				),
			},
		},
	}.Normalized()
}
