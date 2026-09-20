//go:build windows

package nativehost

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/opendownload/opendownload/internal/nativeprotocol"
)

func TestRunExitsWhenDesktopPipeCloses(t *testing.T) {
	browserInput, browserWriter := io.Pipe()
	defer browserWriter.Close()
	desktopHost, desktopPeer := net.Pipe()
	defer desktopPeer.Close()

	done := make(chan error, 1)
	var output bytes.Buffer
	go func() {
		done <- run(browserInput, &output, func() (io.ReadWriteCloser, error) {
			return desktopHost, nil
		})
	}()

	if err := nativeprotocol.Write(browserWriter, nativeprotocol.Message{
		Type:    "start",
		Browser: "firefox",
		TabID:   17,
	}); err != nil {
		t.Fatalf("write browser start: %v", err)
	}

	var start nativeprotocol.Message
	if err := nativeprotocol.Read(desktopPeer, &start); err != nil {
		t.Fatalf("read desktop start: %v", err)
	}
	if start.Type != "start" || start.TabID != 17 {
		t.Fatalf("unexpected desktop start: %+v", start)
	}
	if err := nativeprotocol.Write(desktopPeer, nativeprotocol.Message{
		Type:          "started",
		SessionID:     "session",
		SessionSecret: "secret",
	}); err != nil {
		t.Fatalf("write desktop start response: %v", err)
	}

	if err := desktopPeer.Close(); err != nil {
		t.Fatalf("close desktop pipe: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned an error after desktop shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("native host did not exit after the desktop pipe closed")
	}
}
