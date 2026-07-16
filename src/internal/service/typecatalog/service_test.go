package typecatalog

import (
	catalogpkg "rpl/pkg/sdk/catalog"
	"testing"
)

func TestNormalizeLanguage(t *testing.T) {
	cases := map[string]string{
		"":      "golang",
		"go":    "golang",
		"Go":    "golang",
		"rust":  "rust",
		" Zig ": "zig",
	}

	for input, want := range cases {
		if got := NormalizeLanguage(input); got != want {
			t.Fatalf("NormalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCatalogDefaultsToGolang(t *testing.T) {
	service := New()
	catalog := service.Catalog("")

	if catalog.Lang != "golang" {
		t.Fatalf("catalog lang = %q, want golang", catalog.Lang)
	}
	if !catalogHasType(catalog, "time.Time") {
		t.Fatalf("golang catalog does not include time.Time")
	}
	if !catalogHasStructure(catalog, "Type Alias") {
		t.Fatalf("golang catalog does not include Type Alias structure")
	}
}

func catalogHasType(catalog catalogpkg.Catalog, name string) bool {
	for _, item := range catalog.Types {
		if item.Name == name {
			return true
		}
	}
	return false
}

func catalogHasStructure(catalog catalogpkg.Catalog, label string) bool {
	for _, item := range catalog.Structures {
		if item.Label == label {
			return true
		}
	}
	return false
}
