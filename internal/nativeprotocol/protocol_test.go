package nativeprotocol

import (
	"bytes"
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
	if bytes.Contains(buffer.Bytes(), []byte("technicalDetail")) || bytes.Contains(buffer.Bytes(), []byte("sessionSecret")) {
		t.Fatalf("browser failure payload exposed private data: %s", buffer.String())
	}
}
