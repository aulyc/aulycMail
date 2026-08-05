package email

import (
	"io"
	"strings"
	"testing"
)

func TestMIMECharsetReaderUsesKnownDecodersAndRejectsUnknownCharsets(t *testing.T) {
	reader, err := MIMECharsetReader("utf-8", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("MIMECharsetReader(utf-8) error = %v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil || string(decoded) != "hello" {
		t.Fatalf("decoded utf-8 = %q, %v", decoded, err)
	}

	reader, err = MIMECharsetReader("windows-1252", strings.NewReader("\x80"))
	if err != nil {
		t.Fatalf("MIMECharsetReader(windows-1252) error = %v", err)
	}
	decoded, err = io.ReadAll(reader)
	if err != nil || string(decoded) != "€" {
		t.Fatalf("decoded windows-1252 = %q, %v", decoded, err)
	}

	if _, err := MIMECharsetReader("x-synthetic-unknown", strings.NewReader("data")); err == nil {
		t.Fatal("unknown charset error = nil")
	}
}
