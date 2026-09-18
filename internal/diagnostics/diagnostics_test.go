package diagnostics

import (
	"strings"
	"testing"
	"time"
)

func TestRedactRemovesURLSecretsAndQueries(t *testing.T) {
	value := Redact("https://user:pass@example.test/media.mp4?token=secret#fragment")
	if value != "https://example.test/media.mp4" {
		t.Fatalf("Redact() = %q", value)
	}
}

func TestRedactRemovesHeaderSecrets(t *testing.T) {
	value := Redact("request failed Authorization: Bearer abc Cookie: session=def")
	if strings.Contains(value, "Bearer abc") || strings.Contains(value, "session=def") {
		t.Fatalf("Redact() leaked secret: %q", value)
	}
}

func TestDiagnosticContextAllowlist(t *testing.T) {
	diagnostic := New(CapturePayloadInvalid, StageCapture, false, "Invalid capture", "diag-1", time.Unix(1, 0)).WithContext(map[string]string{
		"browser":  "Chrome",
		"hostname": "example.test",
		"url":      "https://example.test/private?token=secret",
	})
	if len(diagnostic.SafeContext) != 2 || diagnostic.SafeContext["browser"] != "Chrome" {
		t.Fatalf("unexpected safe context: %#v", diagnostic.SafeContext)
	}
}
