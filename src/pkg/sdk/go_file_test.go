package sdk

import (
	"strings"
	"testing"
)

func TestRenderGoFileReturnsFormattingErrors(t *testing.T) {
	builder := NewCodeBuilder()
	builder.AddBlock("broken", "var values = []string{\n\t\"missing trailing comma\"\n}")

	_, err := RenderGoFile("broken", builder.Response())
	if err == nil {
		t.Fatal("RenderGoFile() returned nil error for invalid Go source")
	}
	if !strings.Contains(err.Error(), `format generated Go package "broken"`) {
		t.Fatalf("RenderGoFile() error lacks context: %v", err)
	}
}

func TestRenderGoFileFormatsValidSource(t *testing.T) {
	builder := NewCodeBuilder()
	builder.AddBlock("value", `var Value=[]string{"ok"}`)

	body, err := RenderGoFile("valid", builder.Response())
	if err != nil {
		t.Fatalf("RenderGoFile() failed: %v", err)
	}
	if !strings.Contains(string(body), `var Value = []string{"ok"}`) {
		t.Fatalf("RenderGoFile() did not format source:\n%s", body)
	}
}
