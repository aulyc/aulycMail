package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	imapPkg "aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/message"
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
	var tooLarge RawMessageTooLargeError
	if !errors.As(fmt.Errorf("wrapped: %w", err), &tooLarge) {
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

func TestBodyFetchProgressSupportsConcurrentHeartbeats(t *testing.T) {
	var progress bodyFetchProgress

	const workers = 8
	const incrementsPerWorker = 1_000
	var writers sync.WaitGroup
	writers.Add(workers)
	for range workers {
		go func() {
			defer writers.Done()
			for range incrementsPerWorker {
				progress.addFetched(1)
				progress.addFailed(1)
				_, _ = progress.snapshot()
			}
		}()
	}
	writers.Wait()

	fetched, failed := progress.snapshot()
	want := int64(workers * incrementsPerWorker)
	if fetched != want || failed != want {
		t.Fatalf("progress snapshot = (%d, %d), want (%d, %d)", fetched, failed, want, want)
	}
}

func TestMessageIdentityHelpersNormalizeAndClassifyWrappedErrors(t *testing.T) {
	if got := normalizeRFCMessageID("  <message@example.com>  "); got != "message@example.com" {
		t.Fatalf("normalizeRFCMessageID = %q, want message@example.com", got)
	}
	mismatch := MessageIdentityMismatchError{UID: 7, Expected: "expected", Actual: "actual"}
	wrapped := fmt.Errorf("fetch failed: %w", mismatch)
	if !IsMessageIdentityMismatchError(wrapped) {
		t.Fatal("wrapped identity mismatch should be detected")
	}
	if IsMessageIdentityMismatchError(errors.New("unrelated")) {
		t.Fatal("unrelated error should not be classified as an identity mismatch")
	}
	if !strings.Contains(mismatch.Error(), "UID 7") || !strings.Contains(mismatch.Error(), "expected") {
		t.Fatalf("identity mismatch error omitted useful context: %q", mismatch.Error())
	}
}

func TestValidateFetchedMessageIdentityReportsMalformedMessage(t *testing.T) {
	err := validateFetchedMessageIdentityFromReader(9, "expected@example.com", strings.NewReader("not an RFC822 message"))
	if err == nil || !strings.Contains(err.Error(), "failed to parse message identity for UID 9") {
		t.Fatalf("malformed message error = %v, want identity parse failure", err)
	}
}

func TestRawMessageProgressReaderMarksOnlySuccessfulReads(t *testing.T) {
	marks := 0
	reader := rawMessageProgressReader{
		reader: strings.NewReader("abcdef"),
		mark:   func() { marks++ },
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "abcdef" || marks != 1 {
		t.Fatalf("read = %q, marks = %d, want abcdef/1", string(data), marks)
	}

	withoutCallback, err := io.ReadAll(rawMessageProgressReader{reader: strings.NewReader("ok")})
	if err != nil || string(withoutCallback) != "ok" {
		t.Fatalf("reader without callback = (%q, %v)", string(withoutCallback), err)
	}
}

func TestSkippedBodySizesIgnoresUnmappedUIDsAndEmptyInput(t *testing.T) {
	if got := skippedBodySizesByMessageID(nil, nil); got != nil {
		t.Fatalf("empty skipped bodies = %#v, want nil", got)
	}
	got := skippedBodySizesByMessageID(
		map[uint32]string{1: "known"},
		map[uint32]bodyFetchSkipped{
			1: {Reason: bodyFetchSkipEmpty, ReportedSize: 10, ReceivedBytes: 2},
			2: {Reason: bodyFetchSkipEmpty, ReportedSize: 20, ReceivedBytes: 3},
		},
	)
	if len(got) != 1 || got["known"].reported != 10 || got["known"].received != 2 {
		t.Fatalf("mapped skipped body sizes = %#v", got)
	}
}

func TestFetchBodiesInBackgroundHonorsCancelledContextBeforeDependencies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := (&Engine{}).FetchBodiesInBackground(ctx, "account", "folder", 30)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchBodiesInBackground error = %v, want context.Canceled", err)
	}
}

func TestPlanBodyFetchBatchAppliesCountByteAndOversizeRules(t *testing.T) {
	oversized := maxBackgroundRawBodyBytes + 1
	tests := []struct {
		name          string
		total         int
		candidates    []message.MessageWithSize
		wantIDs       []string
		wantOversized []string
		wantBytes     int64
	}{
		{
			name: "unknown sizes use the planning estimate",
			candidates: []message.MessageWithSize{
				{ID: "unknown", Size: 0},
				{ID: "known", Size: 2 * 1024},
			},
			wantIDs:   []string{"unknown", "known"},
			wantBytes: 12 * 1024,
		},
		{
			name: "oversized messages are returned separately",
			candidates: []message.MessageWithSize{
				{ID: "oversized", Size: oversized},
				{ID: "normal", Size: 1024},
			},
			wantIDs:       []string{"normal"},
			wantOversized: []string{"oversized"},
			wantBytes:     1024,
		},
		{
			name:  "large mailboxes use the reduced count limit",
			total: 1001,
			candidates: func() []message.MessageWithSize {
				items := make([]message.MessageWithSize, 30)
				for i := range items {
					items[i] = message.MessageWithSize{ID: fmt.Sprintf("message-%02d", i), Size: 1024}
				}
				return items
			}(),
			wantIDs: func() []string {
				ids := make([]string, 25)
				for i := range ids {
					ids[i] = fmt.Sprintf("message-%02d", i)
				}
				return ids
			}(),
			wantBytes: 25 * 1024,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := planBodyFetchBatch(test.candidates, test.total)
			if fmt.Sprint(plan.messageIDs) != fmt.Sprint(test.wantIDs) ||
				fmt.Sprint(plan.oversizedIDs) != fmt.Sprint(test.wantOversized) ||
				plan.bytes != test.wantBytes {
				t.Fatalf("plan = %#v, want ids=%v oversized=%v bytes=%d", plan, test.wantIDs, test.wantOversized, test.wantBytes)
			}
		})
	}
}

type fakeBodyFetchConnectionPool struct {
	getErr       error
	getCalls     int
	discardCalls int
	releaseCalls int
}

func (p *fakeBodyFetchConnectionPool) GetConnection(context.Context, string) (*imapPkg.PooledConnection, error) {
	p.getCalls++
	return nil, p.getErr
}

func (p *fakeBodyFetchConnectionPool) Discard(*imapPkg.PooledConnection) {
	p.discardCalls++
}

func (p *fakeBodyFetchConnectionPool) Release(*imapPkg.PooledConnection) {
	p.releaseCalls++
}

func TestRecoverBodyFetchConnectionOwnsDiscardAcquireSelectAndFailureCleanup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := &fakeBodyFetchConnectionPool{}
		selectedPath := ""
		conn, err := recoverBodyFetchConnection(context.Background(), pool, nil, "account", "INBOX", func(_ context.Context, _ *imapPkg.PooledConnection, path string) error {
			selectedPath = path
			return nil
		})
		if err != nil || conn != nil || pool.discardCalls != 1 || pool.getCalls != 1 || pool.releaseCalls != 0 || selectedPath != "INBOX" {
			t.Fatalf("recovery = (%#v, %v), pool=%+v selected=%q", conn, err, pool, selectedPath)
		}
	})

	t.Run("acquire failure", func(t *testing.T) {
		pool := &fakeBodyFetchConnectionPool{getErr: errors.New("offline")}
		_, err := recoverBodyFetchConnection(context.Background(), pool, nil, "account", "INBOX", func(context.Context, *imapPkg.PooledConnection, string) error {
			t.Fatal("selector called after acquire failure")
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "failed to get new connection") || pool.releaseCalls != 0 {
			t.Fatalf("acquire failure = %v, pool=%+v", err, pool)
		}
	})

	t.Run("select failure releases replacement", func(t *testing.T) {
		pool := &fakeBodyFetchConnectionPool{}
		_, err := recoverBodyFetchConnection(context.Background(), pool, nil, "account", "INBOX", func(context.Context, *imapPkg.PooledConnection, string) error {
			return errors.New("select failed")
		})
		if err == nil || !strings.Contains(err.Error(), "failed to select mailbox") || pool.releaseCalls != 1 {
			t.Fatalf("select failure = %v, pool=%+v", err, pool)
		}
	})
}
