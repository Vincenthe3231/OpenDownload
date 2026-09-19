package nativeprotocol

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRoundTripUsesLittleEndianLengthPrefix(t *testing.T) {
	input := Message{Type: "stream", TabID: 7, Stream: &Stream{URL: "https://example.test/media.mp4"}}
	var buffer bytes.Buffer
	if err := Write(&buffer, input); err != nil {
		t.Fatal(err)
	}
	if buffer.Bytes()[0] == 0 || buffer.Bytes()[1] != 0 {
		t.Fatalf("unexpected length prefix: %v", buffer.Bytes()[:4])
	}
	var output Message
	if err := Read(&buffer, &output); err != nil {
		t.Fatal(err)
	}
	if output.Type != input.Type || output.TabID != input.TabID || output.Stream.URL != input.Stream.URL {
		t.Fatalf("round trip mismatch: %#v", output)
	}
	if output.ProtocolVersion != ProtocolVersion {
		t.Fatalf("protocol version = %d, want %d", output.ProtocolVersion, ProtocolVersion)
	}
}

func TestReadRejectsOversizedPayload(t *testing.T) {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xff, 0xff, 0xff, 0x7f})
	if err := Read(&buffer, &Message{}); err == nil {
		t.Fatal("expected oversized payload error")
	}
}

func TestValidateProtocolRejectsUnknownVersion(t *testing.T) {
	message := Message{ProtocolVersion: ProtocolVersion + 1, Type: "start"}
	if err := message.ValidateProtocol(); err == nil {
		t.Fatal("expected unsupported protocol version error")
	}
}

func TestValidateProtocolRejectsMissingVersion(t *testing.T) {
	message := Message{Type: "start"}
	if err := message.ValidateProtocol(); err == nil {
		t.Fatal("expected missing protocol version error")
	}
}

func TestWriteRejectsUnsupportedVersion(t *testing.T) {
	message := Message{ProtocolVersion: ProtocolVersion + 1, Type: "start"}
	var buffer bytes.Buffer
	if err := Write(&buffer, message); err == nil {
		t.Fatal("expected unsupported protocol version error")
	}
}

func TestStartedMessageCarriesDiagnosticMode(t *testing.T) {
	input := Message{Type: "started", DiagnosticMode: true}
	var buffer bytes.Buffer
	if err := Write(&buffer, input); err != nil {
		t.Fatal(err)
	}
	var output Message
	if err := Read(&buffer, &output); err != nil {
		t.Fatal(err)
	}
	if err := output.ValidateProtocol(); err != nil {
		t.Fatal(err)
	}
	if output.Type != "started" || !output.DiagnosticMode {
		t.Fatalf("unexpected start response: %#v", output)
	}
}

func TestFailureCannotContainTechnicalData(t *testing.T) {
	input := Message{
		Type: "error",
		Failure: &Failure{
			Code:         "APP_NOT_RUNNING",
			UserMessage:  "Open OpenDownload, then try again.",
			Retryable:    true,
			DiagnosticID: "diagnostic-1",
		},
	}
	var buffer bytes.Buffer
	if err := Write(&buffer, input); err != nil {
		t.Fatal(err)
	}

	payload := buffer.Bytes()[4:]
	if bytes.Contains(payload, []byte("technicalDetail")) ||
		bytes.Contains(payload, []byte("requestHeaders")) ||
		bytes.Contains(payload, []byte("responseHeaders")) ||
		bytes.Contains(payload, []byte("rawError")) ||
		bytes.Contains(payload, []byte("nativeHostDetail")) ||
		bytes.Contains(payload, []byte("pipeDetail")) ||
		bytes.Contains(payload, []byte("sessionSecret")) ||
		bytes.Contains(payload, []byte("cookie")) ||
		bytes.Contains(payload, []byte("token")) {
		t.Fatalf("browser failure payload exposed private data: %s", buffer.String())
	}

	var envelope struct {
		Failure map[string]json.RawMessage `json:"failure"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	for key := range envelope.Failure {
		switch key {
		case "code", "userMessage", "retryable", "diagnosticId":
		default:
			t.Fatalf("failure payload exposed unsupported field %q", key)
		}
	}
	if strings.Contains(string(payload), "Authorization:") {
		t.Fatalf("browser failure payload exposed header data: %s", payload)
	}
}
