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
}

func TestReadRejectsOversizedPayload(t *testing.T) {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xff, 0xff, 0xff, 0x7f})
	if err := Read(&buffer, &Message{}); err == nil {
		t.Fatal("expected oversized payload error")
	}
}
