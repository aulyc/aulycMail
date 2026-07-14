package sync

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestValidateFetchedMessageIdentityAcceptsMatchingMessageID(t *testing.T) {
	raw := []byte("From: sender@example.com\r\nMessage-ID: <expected@example.com>\r\n\r\nbody")

	if err := validateFetchedMessageIdentity(42, "expected@example.com", raw); err != nil {
		t.Fatalf("expected matching Message-ID to succeed, got %v", err)
	}
}

func TestValidateFetchedMessageIdentityRejectsDifferentMessageID(t *testing.T) {
	raw := []byte("From: sender@example.com\r\nMessage-ID: <actual@example.com>\r\n\r\nbody")

	err := validateFetchedMessageIdentity(42, "expected@example.com", raw)
	if err == nil {
		t.Fatal("expected mismatched Message-ID to fail")
	}

	var mismatch MessageIdentityMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected MessageIdentityMismatchError, got %T %v", err, err)
	}
	if mismatch.UID != 42 || mismatch.Expected != "expected@example.com" || mismatch.Actual != "actual@example.com" {
		t.Fatalf("unexpected mismatch details: %+v", mismatch)
	}
}

func TestValidateFetchedMessageIdentityRejectsMissingMessageID(t *testing.T) {
	raw := []byte("From: sender@example.com\r\nSubject: no id\r\n\r\nbody")

	err := validateFetchedMessageIdentity(42, "expected@example.com", raw)
	if err == nil {
		t.Fatal("expected missing Message-ID to fail when local metadata has one")
	}

	var mismatch MessageIdentityMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected MessageIdentityMismatchError, got %T %v", err, err)
	}
}

func TestValidateFetchedMessageIdentityAllowsMissingExpectedMessageID(t *testing.T) {
	raw := []byte("From: sender@example.com\r\nMessage-ID: <actual@example.com>\r\n\r\nbody")

	if err := validateFetchedMessageIdentity(42, "", raw); err != nil {
		t.Fatalf("messages without local Message-ID cannot be validated and should remain readable: %v", err)
	}
}

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

func TestReadHeaderLiteralWithLimitRejectsOversized(t *testing.T) {
	_, err := readHeaderLiteralWithLimit(strings.NewReader(strings.Repeat("a", maxHeaderLiteralBytes+1)))
	if err == nil {
		t.Fatal("expected oversized header literal to fail")
	}
	if _, ok := err.(HeaderLiteralTooLargeError); !ok {
		t.Fatalf("expected HeaderLiteralTooLargeError, got %T %v", err, err)
	}
}

func TestSkippedBodySizesTooLargeForcesFailureCharge(t *testing.T) {
	sizes := skippedBodySizesByMessageID(
		map[uint32]string{42: "msg-42"},
		map[uint32]bodyFetchSkipped{
			42: {
				Reason:        bodyFetchSkipTooLarge,
				ReportedSize:  100 * 1024 * 1024,
				ReceivedBytes: maxBackgroundRawBodyBytes + 1,
			},
		},
	)
	size, ok := sizes["msg-42"]
	if !ok {
		t.Fatal("missing skipped size for msg-42")
	}
	if !shouldChargeFailure(size.received, size.reported) {
		t.Fatal("oversized body skip should be charged so background sync does not retry forever")
	}
}

func TestSkippedBodySizesEmptyCanDeferTruncatedFetch(t *testing.T) {
	sizes := skippedBodySizesByMessageID(
		map[uint32]string{42: "msg-42"},
		map[uint32]bodyFetchSkipped{
			42: {
				Reason:        bodyFetchSkipEmpty,
				ReportedSize:  100_000,
				ReceivedBytes: 10,
			},
		},
	)
	size := sizes["msg-42"]
	if shouldChargeFailure(size.received, size.reported) {
		t.Fatal("empty body with a large reported-size shortfall should be deferred")
	}
}
