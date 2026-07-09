package sync

import (
	"fmt"
	"strings"
	"testing"
)

func TestRawMessageStreamErrorAllowsServerSizeMismatch(t *testing.T) {
	result := &RawMessageStreamResult{
		BytesWritten: 2205,
		ReportedSize: 2203,
	}

	if err := rawMessageStreamError(true, result, 16640); err != nil {
		t.Fatalf("expected size mismatch to be allowed, got %v", err)
	}
	if !rawMessageSizeMismatch(result) {
		t.Fatal("expected mismatch to be detectable for logging")
	}
}

func TestRawMessageStreamErrorRejectsMissingBody(t *testing.T) {
	if err := rawMessageStreamError(false, &RawMessageStreamResult{}, 16640); err == nil {
		t.Fatal("expected missing body section to fail")
	}
	if err := rawMessageStreamError(true, &RawMessageStreamResult{}, 16640); err == nil {
		t.Fatal("expected empty body to fail")
	}
}

func TestIsRawMessageNotFoundError(t *testing.T) {
	err := fmt.Errorf("fetch failed: %w", RawMessageNotFoundError{UID: 21127})
	if !IsRawMessageNotFoundError(err) {
		t.Fatal("expected wrapped raw message not-found error to be detected")
	}
	if IsRawMessageNotFoundError(fmt.Errorf("message body not found")) {
		t.Fatal("expected unrelated errors not to be treated as UID not found")
	}
}

func TestReadRawMessageLiteralWithLimitRejectsOversized(t *testing.T) {
	_, err := readRawMessageLiteralWithLimit(42, strings.NewReader("abcdef"), 5)
	if err == nil {
		t.Fatal("expected oversized raw message to fail")
	}
	if !IsRawMessageTooLargeError(fmt.Errorf("wrapped: %w", err)) {
		t.Fatalf("expected RawMessageTooLargeError, got %T %v", err, err)
	}
}

func TestReadRawMessageLiteralWithLimitAllowsAtLimit(t *testing.T) {
	raw, err := readRawMessageLiteralWithLimit(42, strings.NewReader("abcde"), 5)
	if err != nil {
		t.Fatalf("expected at-limit raw message to succeed: %v", err)
	}
	if string(raw) != "abcde" {
		t.Fatalf("raw = %q, want abcde", string(raw))
	}
}
