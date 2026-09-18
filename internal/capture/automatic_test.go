package capture

import (
	"testing"

	"github.com/opendownload/opendownload/internal/nativeprotocol"
)

func TestAutomaticCaptureReplacesPriorCapability(t *testing.T) {
	manager := NewManager()
	first, err := manager.StartAutomatic("chrome", 41)
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.StartAutomatic("edge", 42)
	if err != nil {
		t.Fatal(err)
	}
	if first.SessionID == second.SessionID || first.Secret == second.Secret {
		t.Fatal("automatic capability was reused")
	}

	stream := nativeprotocol.Stream{URL: "https://example.test/first.mp4", Type: "direct"}
	if err := manager.AcceptAutomaticStream(first.SessionID, first.Secret, 41, stream); err == nil {
		t.Fatal("replaced capability was accepted")
	}
	if err := manager.AcceptAutomaticStream(second.SessionID, second.Secret, 42, nativeprotocol.Stream{URL: "https://example.test/second.mp4", Type: "direct"}); err != nil {
		t.Fatal(err)
	}
	if got := manager.List(); len(got) != 1 || got[0].Host != "example.test" {
		t.Fatalf("unexpected streams: %#v", got)
	}
	state := manager.Session()
	if !state.Active || state.Mode != ModeAutomatic || state.Browser != "edge" || state.TabID != 42 {
		t.Fatalf("unexpected session: %#v", state)
	}
}
