package fingerprint

import "testing"

func TestFingerprintFromSourcesIsDeterministic(t *testing.T) {
	first, err := fingerprintFromSources([]Source{
		{Name: "mac", Value: "bb:bb:bb:bb:bb:bb"},
		{Name: "platform_uuid", Value: "machine-a"},
		{Name: "mac", Value: "aa:aa:aa:aa:aa:aa"},
	})
	if err != nil {
		t.Fatalf("fingerprint from first source set: %v", err)
	}

	second, err := fingerprintFromSources([]Source{
		{Name: "mac", Value: "aa:aa:aa:aa:aa:aa"},
		{Name: "platform_uuid", Value: "machine-a"},
		{Name: "mac", Value: "bb:bb:bb:bb:bb:bb"},
		{Name: "platform_uuid", Value: "machine-a"},
		{Name: "", Value: "ignored"},
	})
	if err != nil {
		t.Fatalf("fingerprint from second source set: %v", err)
	}

	if first != second {
		t.Fatalf("expected deterministic fingerprint, got %q and %q", first, second)
	}
}

func TestSelectFingerprintSourcesPrefersPrimary(t *testing.T) {
	selected := selectFingerprintSources(
		[]Source{{Name: "platform_uuid", Value: "machine-a"}},
		[]Source{{Name: "mac", Value: "aa:aa:aa:aa:aa:aa"}},
	)

	if len(selected) != 1 || selected[0].Name != "platform_uuid" {
		t.Fatalf("expected primary source to win, got %#v", selected)
	}
}

func TestFingerprintFromSourcesRejectsEmptyInput(t *testing.T) {
	if _, err := fingerprintFromSources(nil); err == nil {
		t.Fatal("expected empty source list to fail")
	}
}
