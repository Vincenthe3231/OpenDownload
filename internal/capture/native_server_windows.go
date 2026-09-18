//go:build windows

package capture

import (
	"context"
	"net"

	"github.com/opendownload/opendownload/internal/nativepipe"
	"github.com/opendownload/opendownload/internal/nativeprotocol"
)

// StartNativeServer accepts only the current user's native-host pipe.
func (m *Manager) StartNativeServer(ctx context.Context) (func(), error) {
	listener, err := nativepipe.Listen()
	if err != nil {
		return nil, err
	}
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = listener.Close()
		case <-stop:
		}
	}()
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go m.serveNativeConnection(conn)
		}
	}()
	return func() {
		close(stop)
		_ = listener.Close()
		m.Stop()
	}, nil
}

func (m *Manager) serveNativeConnection(conn net.Conn) {
	defer conn.Close()
	var capability NativeCapability
	for {
		var message nativeprotocol.Message
		if err := nativeprotocol.Read(conn, &message); err != nil {
			return
		}
		switch message.Type {
		case "start":
			created, err := m.StartAutomatic(message.Browser, message.TabID)
			if err != nil {
				_ = nativeprotocol.Write(conn, nativeprotocol.Message{Type: "error", Error: err.Error()})
				continue
			}
			capability = created
			if err := nativeprotocol.Write(conn, nativeprotocol.Message{Type: "started", SessionID: capability.SessionID, SessionSecret: capability.Secret}); err != nil {
				return
			}
		case "stream":
			if message.Stream == nil {
				_ = nativeprotocol.Write(conn, nativeprotocol.Message{Type: "error", ErrorCode: "CAPTURE_PAYLOAD_INVALID", Error: "capture stream is missing"})
				continue
			}
			err := m.AcceptAutomaticStream(message.SessionID, message.SessionSecret, message.TabID, *message.Stream)
			if err != nil {
				_ = nativeprotocol.Write(conn, nativeprotocol.Message{Type: "error", ErrorCode: "CAPTURE_REQUEST_REJECTED", Error: err.Error()})
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
			_ = nativeprotocol.Write(conn, nativeprotocol.Message{Type: "error", ErrorCode: "IPC_PROTOCOL_INVALID", Error: "unsupported native message"})
		}
	}
}
