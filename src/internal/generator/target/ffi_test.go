package target

import "testing"

func TestFFITargetIsArtifactOnlyAndKeepsModelDirectory(t *testing.T) {
	renderer, ok := Lookup("ffi")
	if !ok {
		t.Fatal("ffi target is not registered")
	}
	if EmitsHostModel(renderer) {
		t.Fatal("ffi target unexpectedly emits a host model")
	}
	layout := ResolveModelLayout(renderer, "CalculatorService")
	if layout.ModelDirName != "calculator_service" {
		t.Fatalf("model directory = %q, want calculator_service", layout.ModelDirName)
	}
	if layout.MainRelative != "" {
		t.Fatalf("main relative path = %q, want no host file", layout.MainRelative)
	}
}
