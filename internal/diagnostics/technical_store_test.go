package diagnostics

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTechnicalStoreDisabledByDefault(t *testing.T) {
	store := NewTechnicalStore()
	if store.Enabled() {
		t.Fatal("TechnicalStore enabled by default")
	}
	if id := store.Record(TechnicalDiagnostic{DiagnosticID: "diag-1", RawError: "secret"}); id != "" {
		t.Fatalf("Record() while disabled = %q", id)
	}
	if diagnostic := store.Get("diag-1"); diagnostic != nil {
		t.Fatalf("Get() while disabled = %#v", diagnostic)
	}
}

func TestTechnicalStoreTruncatesHeadersAndClearsOnDisable(t *testing.T) {
	store := NewTechnicalStore()
	store.SetEnabled(true)
	largeHeader := strings.Repeat("a", DefaultTechnicalHeaderValueBytes+1)
	id := store.Record(TechnicalDiagnostic{
		DiagnosticID:         "diag-1",
		OccurredAt:           time.Unix(1, 0),
		RequestID:            "browser-request-1",
		RequestMethod:        "GET",
		RequestType:          "media",
		RequestTimestamp:     1234.5,
		RequestFrameID:       7,
		RequestParentFrameID: 2,
		RequestURL:           "https://example.test/private?token=secret",
		RequestDocumentURL:   "https://example.test/watch?session=secret",
		RequestOriginURL:     "https://origin.example.test/?token=secret",
		RequestInitiator:     "https://initiator.example.test/?token=secret",
		ResponseStatusLine:   "HTTP/2 403 Forbidden",
		ResponseFromCache:    true,
		ResponseIP:           "192.0.2.1",
		RequestHeaders: map[string]string{
			"Authorization": largeHeader,
		},
	})
	if id != "diag-1" {
		t.Fatalf("Record() ID = %q", id)
	}
	diagnostic := store.Get(id)
	if diagnostic == nil {
		t.Fatal("Get() = nil")
	}
	if !diagnostic.Truncated {
		t.Fatal("diagnostic was not marked truncated")
	}
	if got := len(diagnostic.RequestHeaders["Authorization"]); got > DefaultTechnicalHeaderValueBytes {
		t.Fatalf("header bytes = %d, want at most %d", got, DefaultTechnicalHeaderValueBytes)
	}
	if diagnostic.RequestID != "browser-request-1" || diagnostic.RequestMethod != "GET" || diagnostic.RequestType != "media" || diagnostic.RequestTimestamp != 1234.5 || diagnostic.RequestFrameID != 7 || diagnostic.RequestParentFrameID != 2 || diagnostic.RequestDocumentURL == "" || diagnostic.RequestOriginURL == "" || diagnostic.RequestInitiator == "" || diagnostic.ResponseStatusLine != "HTTP/2 403 Forbidden" || !diagnostic.ResponseFromCache || diagnostic.ResponseIP != "192.0.2.1" {
		t.Fatalf("browser metadata was not retained: %#v", diagnostic)
	}
	diagnostic.RequestHeaders["Authorization"] = "mutated"
	if got := store.Get(id).RequestHeaders["Authorization"]; got == "mutated" {
		t.Fatal("Get() exposed internal header map")
	}

	store.SetEnabled(false)
	if store.Count() != 0 || store.Bytes() != 0 {
		t.Fatalf("store retained data after disable: count=%d bytes=%d", store.Count(), store.Bytes())
	}
	if diagnostic := store.Get(id); diagnostic != nil {
		t.Fatalf("Get() after disable = %#v", diagnostic)
	}
}

func TestTechnicalStoreEvictsOldestRecordAndHonorsByteCap(t *testing.T) {
	t.Run("record cap", func(t *testing.T) {
		store := NewTechnicalStoreWithOptions(TechnicalStoreOptions{MaxRecords: 2, MaxBytes: 10 << 10})
		store.SetEnabled(true)
		for _, id := range []string{"first", "second", "third"} {
			if got := store.Record(TechnicalDiagnostic{DiagnosticID: id, RawError: "failure"}); got != id {
				t.Fatalf("Record(%q) = %q", id, got)
			}
		}
		if store.Get("first") != nil {
			t.Fatal("oldest record was not evicted")
		}
		if store.Get("second") == nil || store.Get("third") == nil || store.Count() != 2 {
			t.Fatalf("unexpected retained records: count=%d", store.Count())
		}
	})

	t.Run("byte cap", func(t *testing.T) {
		const maxBytes = 1300
		store := NewTechnicalStoreWithOptions(TechnicalStoreOptions{MaxRecords: 20, MaxBytes: maxBytes})
		store.SetEnabled(true)
		id := store.Record(TechnicalDiagnostic{DiagnosticID: "large", RawError: strings.Repeat("x", 128<<10)})
		if id != "large" {
			t.Fatalf("Record() = %q", id)
		}
		diagnostic := store.Get(id)
		if diagnostic == nil || !diagnostic.Truncated {
			t.Fatalf("large diagnostic = %#v", diagnostic)
		}
		if store.Bytes() > maxBytes {
			t.Fatalf("store bytes = %d, want at most %d", store.Bytes(), maxBytes)
		}
	})
}

func TestTechnicalStoreConcurrentAccess(t *testing.T) {
	store := NewTechnicalStore()
	store.SetEnabled(true)
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for index := 0; index < 64; index++ {
				id := store.Record(TechnicalDiagnostic{DiagnosticID: strings.Join([]string{"diag", string(rune('a' + worker)), string(rune('a' + index%26))}, "-"), RawError: "failure"})
				if id != "" {
					_ = store.Get(id)
				}
				_ = store.Count()
				_ = store.Bytes()
			}
		}(worker)
	}
	group.Wait()
	if store.Count() > DefaultTechnicalStoreMaxRecords || store.Bytes() > DefaultTechnicalStoreMaxBytes {
		t.Fatalf("store exceeded limits: count=%d bytes=%d", store.Count(), store.Bytes())
	}
}
