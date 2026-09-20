//go:build windows

package nativehost

import (
	"io"
	"os"

	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/nativepipe"
	"github.com/opendownload/opendownload/internal/nativeprotocol"
)

// Run bridges browser native messaging to the already-running OpenDownload app.
func Run() error {
	return run(os.Stdin, os.Stdout, func() (io.ReadWriteCloser, error) {
		return nativepipe.Dial()
	})
}

type protocolEvent struct {
	message nativeprotocol.Message
	err     error
}

type pipeSession struct {
	connection io.ReadWriteCloser
	events     chan protocolEvent
}

func newPipeSession(connection io.ReadWriteCloser) *pipeSession {
	session := &pipeSession{
		connection: connection,
		events:     make(chan protocolEvent, 1),
	}
	go func() {
		for {
			var message nativeprotocol.Message
			if err := nativeprotocol.Read(connection, &message); err != nil {
				session.events <- protocolEvent{err: err}
				return
			}
			session.events <- protocolEvent{message: message}
		}
	}()
	return session
}

func (session *pipeSession) next() (nativeprotocol.Message, error) {
	event := <-session.events
	return event.message, event.err
}

func (session *pipeSession) close() {
	if session != nil && session.connection != nil {
		_ = session.connection.Close()
	}
}

func readBrowserMessages(input io.Reader) <-chan protocolEvent {
	events := make(chan protocolEvent, 1)
	go func() {
		for {
			var message nativeprotocol.Message
			if err := nativeprotocol.Read(input, &message); err != nil {
				events <- protocolEvent{err: err}
				return
			}
			events <- protocolEvent{message: message}
		}
	}()
	return events
}

func run(input io.Reader, output io.Writer, dial func() (io.ReadWriteCloser, error)) error {
	browserEvents := readBrowserMessages(input)
	var pipe *pipeSession
	var pipeEvents <-chan protocolEvent
	var sessionID, secret string
	var tabID int64
	started := false
	diagnosticMode := false

	for {
		select {
		case event := <-browserEvents:
			if event.err != nil {
				if pipe != nil {
					pipe.close()
				}
				return event.err
			}
			message := event.message
			if err := message.ValidateProtocol(); err != nil {
				if writeErr := writeNativeFailure(output, failure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture needs an update before it can connect.")); writeErr != nil {
					return writeErr
				}
				continue
			}
			switch message.Type {
			case "start":
				if pipe != nil {
					pipe.close()
					pipe = nil
					pipeEvents = nil
				}
				connection, err := dial()
				if err != nil {
					return writeNativeFailure(output, failure(diagnostics.AppNotRunning, true, "Open OpenDownload, then try automatic capture again."))
				}
				pipe = newPipeSession(connection)
				pipeEvents = pipe.events
				if err := nativeprotocol.Write(connection, message); err != nil {
					return writeNativeFailure(output, failure(diagnostics.NativeHostDisconnected, true, "OpenDownload stopped responding. Restart it, then try again."))
				}
				response, err := pipe.next()
				if err != nil {
					return writeNativeFailure(output, failure(diagnostics.NativeHostDisconnected, true, "OpenDownload stopped responding. Restart it, then try again."))
				}
				if err := response.ValidateProtocol(); err != nil {
					return writeNativeFailure(output, failure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture received an incompatible response."))
				}
				if response.Type != "started" {
					if response.Failure != nil {
						return writeNativeFailure(output, response.Failure)
					}
					return writeNativeFailure(output, failure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture received an invalid response."))
				}
				if response.SessionID == "" || response.SessionSecret == "" {
					return writeNativeFailure(output, failure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture received an incomplete session."))
				}
				sessionID, secret, tabID, started, diagnosticMode = response.SessionID, response.SessionSecret, message.TabID, true, response.DiagnosticMode
				// Session credentials never cross the native-host boundary into the extension.
				if err := nativeprotocol.Write(output, nativeprotocol.Message{Type: "started", DiagnosticMode: diagnosticMode}); err != nil {
					return err
				}
			case "stream":
				if !started || pipe == nil {
					if err := writeNativeFailure(output, failure(diagnostics.AppNotRunning, true, "Start automatic capture before sending media requests.")); err != nil {
						return err
					}
					continue
				}
				if message.TabID != tabID {
					if err := writeNativeFailure(output, failure(diagnostics.CaptureRequestRejected, false, "This media request is not from the selected tab.")); err != nil {
						return err
					}
					continue
				}
				message.SessionID, message.SessionSecret = sessionID, secret
				if !diagnosticMode {
					message.Debug = nil
				}
				if err := nativeprotocol.Write(pipe.connection, message); err != nil {
					return writeNativeFailure(output, failure(diagnostics.NativeHostDisconnected, true, "OpenDownload stopped responding. Restart it, then try again."))
				}
				response, err := pipe.next()
				if err != nil {
					return writeNativeFailure(output, failure(diagnostics.NativeHostDisconnected, true, "OpenDownload stopped responding. Restart it, then try again."))
				}
				if err := response.ValidateProtocol(); err != nil {
					return writeNativeFailure(output, failure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture received an incompatible response."))
				}
				if response.Type == "error" {
					if response.Failure == nil {
						return writeNativeFailure(output, failure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture received an invalid response."))
					}
					if err := writeNativeFailure(output, response.Failure); err != nil {
						return err
					}
					continue
				}
				if response.Type != "accepted" {
					return writeNativeFailure(output, failure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture received an invalid response."))
				}
				if err := nativeprotocol.Write(output, nativeprotocol.Message{Type: "accepted"}); err != nil {
					return err
				}
			case "stop":
				if pipe != nil {
					_ = nativeprotocol.Write(pipe.connection, nativeprotocol.Message{Type: "stop"})
					pipe.close()
				}
				return nil
			default:
				if err := writeNativeFailure(output, failure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture received an unsupported message.")); err != nil {
					return err
				}
			}
		case event := <-pipeEvents:
			if event.err != nil {
				if pipe != nil {
					pipe.close()
				}
				return nil
			}
			// The desktop app only sends a response after a browser request. If a
			// stale response arrives while idle, discard it and keep the bridge
			// available for the next browser message.
		}
	}
}

func failure(code diagnostics.Code, retryable bool, userMessage string) *nativeprotocol.Failure {
	return &nativeprotocol.Failure{
		Code:         string(code),
		UserMessage:  userMessage,
		Retryable:    retryable,
		DiagnosticID: diagnostics.NewID(),
	}
}

func writeNativeFailure(output io.Writer, failure *nativeprotocol.Failure) error {
	if failure == nil {
		failure = globalsafeFailure()
	}
	return nativeprotocol.Write(output, nativeprotocol.Message{Type: "error", Failure: failure})
}

func globalsafeFailure() *nativeprotocol.Failure {
	return failure(diagnostics.IPCProtocolInvalid, false, "OpenDownload Capture could not complete this request.")
}
