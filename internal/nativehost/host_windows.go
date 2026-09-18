//go:build windows

package nativehost

import (
	"fmt"
	"os"

	"github.com/opendownload/opendownload/internal/nativepipe"
	"github.com/opendownload/opendownload/internal/nativeprotocol"
)

// Run bridges browser native messaging to the already-running OpenDownload app.
func Run() error {
	var pipe interface {
		Read([]byte) (int, error)
		Write([]byte) (int, error)
		Close() error
	}
	var sessionID, secret string
	var tabID int64
	started := false

	for {
		var message nativeprotocol.Message
		if err := nativeprotocol.Read(os.Stdin, &message); err != nil {
			return err
		}
		switch message.Type {
		case "start":
			if pipe != nil {
				_ = pipe.Close()
				pipe = nil
			}
			connection, err := nativepipe.Dial()
			if err != nil {
				return writeNativeError("APP_NOT_RUNNING", err)
			}
			pipe = connection
			if err := nativeprotocol.Write(connection, message); err != nil {
				return err
			}
			var response nativeprotocol.Message
			if err := nativeprotocol.Read(connection, &response); err != nil {
				return err
			}
			if response.Type != "started" {
				return writeNativeError(response.ErrorCode, fmt.Errorf("%s", response.Error))
			}
			sessionID, secret, tabID, started = response.SessionID, response.SessionSecret, message.TabID, true
			// Session credentials never cross the native-host boundary into the extension.
			if err := nativeprotocol.Write(os.Stdout, nativeprotocol.Message{Type: "started"}); err != nil {
				return err
			}
		case "stream":
			if !started || pipe == nil {
				if err := writeNativeError("APP_NOT_RUNNING", fmt.Errorf("capture has not started")); err != nil {
					return err
				}
				continue
			}
			if message.TabID != tabID {
				if err := writeNativeError("CAPTURE_REQUEST_REJECTED", fmt.Errorf("stream tab is not selected tab")); err != nil {
					return err
				}
				continue
			}
			message.SessionID, message.SessionSecret = sessionID, secret
			if err := nativeprotocol.Write(pipe, message); err != nil {
				return err
			}
			var response nativeprotocol.Message
			if err := nativeprotocol.Read(pipe, &response); err != nil {
				return err
			}
			if response.Type == "error" {
				if err := writeNativeError(response.ErrorCode, fmt.Errorf("%s", response.Error)); err != nil {
					return err
				}
				continue
			}
			if err := nativeprotocol.Write(os.Stdout, nativeprotocol.Message{Type: "accepted"}); err != nil {
				return err
			}
		case "stop":
			if pipe != nil {
				_ = nativeprotocol.Write(pipe, nativeprotocol.Message{Type: "stop"})
				_ = pipe.Close()
			}
			return nil
		default:
			if err := writeNativeError("IPC_PROTOCOL_INVALID", fmt.Errorf("unsupported native message")); err != nil {
				return err
			}
		}
	}
}

func writeNativeError(code string, err error) error {
	return nativeprotocol.Write(os.Stdout, nativeprotocol.Message{Type: "error", ErrorCode: code, Error: err.Error()})
}
