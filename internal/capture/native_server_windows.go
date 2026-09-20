//go:build windows

package capture

import (
	"context"
	"net"
	"sync"

	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/nativepipe"
	"github.com/opendownload/opendownload/internal/nativeprotocol"
)

// StartNativeServer accepts only the current user's native-host pipe.
func (m *Manager) StartNativeServer(ctx context.Context) (func(), error) {
	listener, err := nativepipe.Listen()
	if err != nil {
		return nil, err
	}
	stopSignal := make(chan struct{})
	var stopOnce sync.Once
	var connectionsMu sync.Mutex
	connections := make(map[net.Conn]struct{})
	stopped := false
	stop := func() {
		stopOnce.Do(func() {
			close(stopSignal)
			_ = listener.Close()

			connectionsMu.Lock()
			stopped = true
			for conn := range connections {
				_ = conn.Close()
			}
			connectionsMu.Unlock()

			m.Stop()
		})
	}
	go func() {
		select {
		case <-ctx.Done():
			stop()
		case <-stopSignal:
		}
	}()
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}

			connectionsMu.Lock()
			if stopped {
				connectionsMu.Unlock()
				_ = conn.Close()
				continue
			}
			connections[conn] = struct{}{}
			connectionsMu.Unlock()

			go func() {
				defer func() {
					connectionsMu.Lock()
					delete(connections, conn)
					connectionsMu.Unlock()
				}()
				m.serveNativeConnection(conn)
			}()
		}
	}()
	return stop, nil
}

func (m *Manager) serveNativeConnection(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	var capability NativeCapability
	for {
		var message nativeprotocol.Message
		if err := nativeprotocol.Read(conn, &message); err != nil {
			return
		}
		if err := message.ValidateProtocol(); err != nil {
			_ = writePipeFailure(conn, pipeFailure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture needs an update before it can connect."))
			continue
		}
		switch message.Type {
		case "start":
			created, err := m.StartAutomatic(message.Browser, message.TabID)
			if err != nil {
				_ = writePipeFailure(conn, pipeFailure(diagnostics.CaptureRequestRejected, true, "Could not start automatic capture."))
				continue
			}
			capability = created
			if err := nativeprotocol.Write(conn, nativeprotocol.Message{
				Type:           "started",
				SessionID:      capability.SessionID,
				SessionSecret:  capability.Secret,
				DiagnosticMode: capability.DiagnosticMode,
			}); err != nil {
				return
			}
		case "stream":
			if message.Stream == nil {
				_ = writePipeFailure(conn, pipeFailure(diagnostics.CapturePayloadInvalid, false, "The browser sent an invalid media request."))
				continue
			}
			err := m.AcceptAutomaticStreamWithDebug(message.SessionID, message.SessionSecret, message.TabID, *message.Stream, message.Debug)
			if err != nil {
				_ = writePipeFailure(conn, pipeFailure(diagnostics.CaptureRequestRejected, false, "OpenDownload did not accept this media request."))
				continue
			}
			if err := nativeprotocol.Write(conn, nativeprotocol.Message{Type: "accepted"}); err != nil {
				return
			}
		case "stop":
			m.StopAutomatic(capability.SessionID, capability.Secret)
			_ = nativeprotocol.Write(conn, nativeprotocol.Message{Type: "stopped"})
			return
		default:
			_ = writePipeFailure(conn, pipeFailure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture received an unsupported message."))
		}
	}
}

func pipeFailure(code diagnostics.Code, retryable bool, userMessage string) *nativeprotocol.Failure {
	return &nativeprotocol.Failure{
		Code:         string(code),
		UserMessage:  userMessage,
		Retryable:    retryable,
		DiagnosticID: diagnostics.NewID(),
	}
}

func writePipeFailure(conn net.Conn, failure *nativeprotocol.Failure) error {
	return nativeprotocol.Write(conn, nativeprotocol.Message{Type: "error", Failure: failure})
}
