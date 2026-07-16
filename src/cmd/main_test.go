package main

import (
	"errors"
	"rpl/internal/version"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"strings"
	"testing"
)

func TestValidateFingerprintMismatchUsesFriendlyMessage(t *testing.T) {
	originalFingerprint := version.Fingerprint
	version.Fingerprint = "expected-device"
	defer func() { version.Fingerprint = originalFingerprint }()

	originalLang := localize.Language
	originalColor := localize.UseColor
	localize.SetLanguage(localize.LangEN)
	localize.UseColor = false
	defer func() {
		localize.SetLanguage(originalLang)
		localize.UseColor = originalColor
	}()

	err := validateFingerprint("abc123", nil)
	if err == nil {
		t.Fatal("expected mismatch error")
	}

	rendered := rplerr.Format(nil, err)
	if !strings.Contains(rendered, "this RPL build is intended for a different device") {
		t.Fatalf("expected friendly mismatch message, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Rebuild RPL on this device") {
		t.Fatalf("expected helpful hint, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "abc123") {
		t.Fatalf("expected raw fingerprint to stay out of the main output, got:\n%s", rendered)
	}
}

func TestValidateFingerprintLookupErrorUsesFriendlyMessage(t *testing.T) {
	originalFingerprint := version.Fingerprint
	version.Fingerprint = "expected-device"
	defer func() { version.Fingerprint = originalFingerprint }()

	originalLang := localize.Language
	originalColor := localize.UseColor
	localize.SetLanguage(localize.LangEN)
	localize.UseColor = false
	defer func() {
		localize.SetLanguage(originalLang)
		localize.UseColor = originalColor
	}()

	err := validateFingerprint("", errors.New("registry read failed"))
	if err == nil {
		t.Fatal("expected lookup error")
	}

	rendered := rplerr.Format(nil, err)
	if !strings.Contains(rendered, "failed to determine the device fingerprint") {
		t.Fatalf("expected friendly lookup message, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "registry read failed") {
		t.Fatalf("expected root cause in rendered output, got:\n%s", rendered)
	}
}

func TestValidateFingerprintAllowsPortableReleaseBuild(t *testing.T) {
	originalFingerprint := version.Fingerprint
	version.Fingerprint = ""
	defer func() { version.Fingerprint = originalFingerprint }()

	if err := validateFingerprint("another-device", errors.New("unavailable")); err != nil {
		t.Fatalf("portable release build must not require a device fingerprint: %v", err)
	}
}
