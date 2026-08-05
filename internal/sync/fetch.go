package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	imapPkg "aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/message"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	gomessage "github.com/emersion/go-message"
)

type bodyFetchTarget struct {
	LocalMessageID string
	RFCMessageID   string
}

// ProcessedBody holds the parsed body content and attachments for a message
type ProcessedBody struct {
	MessageID      string
	BodyHTML       string
	BodyText       string
	Snippet        string
	HasAttachments bool
	Attachments    []*message.Attachment // Extracted during parsing (no re-parse needed)
	RawBytes       []byte                // For on-demand attachment content fetch

	// Size signals used by shouldChargeFailure to distinguish "true empty
	// body" from "server-side truncation". ReportedSize comes from the
	// IMAP RFC822.SIZE response item; ReceivedBytes is what we actually
	// read from the BODY[] literal. A meaningful shortfall between the
	// two means the FETCH was likely truncated and we should NOT persist
	// the failure — the next sync may succeed.
	ReportedSize  int64
	ReceivedBytes int64
}

type bodyFetchSkipReason string

const (
	bodyFetchSkipEmpty    bodyFetchSkipReason = "empty"
	bodyFetchSkipTooLarge bodyFetchSkipReason = "too_large"
)

type bodyFetchSkipped struct {
	Reason        bodyFetchSkipReason
	ReportedSize  int64
	ReceivedBytes int64
}

// RawMessageStreamResult summarizes a raw message stream operation.
type RawMessageStreamResult struct {
	BytesWritten int64
	ReportedSize int64
}

// RawMessageNotFoundError means the server did not return a requested UID from
// the selected mailbox. The local DB can still contain stale rows until a sync
// reconciles them.
type RawMessageNotFoundError struct {
	UID uint32
}

func (e RawMessageNotFoundError) Error() string {
	return fmt.Sprintf("message not found: UID %d", e.UID)
}

// MessageIdentityMismatchError means BODY[] returned a different RFC 822
// message than the local row requested for the selected mailbox UID. Persisting
// it would corrupt the local cache, so callers must discard the IMAP connection.
type MessageIdentityMismatchError struct {
	UID      uint32
	Expected string
	Actual   string
}

func (e MessageIdentityMismatchError) Error() string {
	return fmt.Sprintf("message identity mismatch for UID %d: expected %q, got %q", e.UID, e.Expected, e.Actual)
}

func IsMessageIdentityMismatchError(err error) bool {
	var target MessageIdentityMismatchError
	return errors.As(err, &target)
}

func normalizeRFCMessageID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "<")
	value = strings.TrimSuffix(value, ">")
	return strings.TrimSpace(value)
}

func validateFetchedMessageIdentity(uid uint32, expected string, raw []byte) error {
	return validateFetchedMessageIdentityFromReader(uid, expected, bytes.NewReader(raw))
}

func validateFetchedMessageIdentityFromReader(uid uint32, expected string, raw io.Reader) error {
	expected = normalizeRFCMessageID(expected)
	if expected == "" {
		// Legacy and malformed messages may legitimately have no Message-ID in
		// the cached envelope, leaving no stable identity to cross-check.
		return nil
	}

	entity, err := gomessage.Read(raw)
	if err != nil {
		return fmt.Errorf("failed to parse message identity for UID %d: %w", uid, err)
	}
	actual := normalizeRFCMessageID(entity.Header.Get("Message-ID"))
	if actual != expected {
		return MessageIdentityMismatchError{UID: uid, Expected: expected, Actual: actual}
	}
	return nil
}

func IsRawMessageNotFoundError(err error) bool {
	var notFound RawMessageNotFoundError
	return errors.As(err, &notFound)
}

// RawMessageTooLargeError means a caller asked for a raw message as []byte but
// the message exceeds the in-memory safety cap. Backup/export paths should use
// StreamRawMessage instead.
type RawMessageTooLargeError struct {
	UID        uint32
	LimitBytes int64
}

func (e RawMessageTooLargeError) Error() string {
	return fmt.Sprintf("message too large to load into memory: UID %d exceeds %d bytes", e.UID, e.LimitBytes)
}

type HeaderLiteralTooLargeError struct {
	LimitBytes int64
}

func (e HeaderLiteralTooLargeError) Error() string {
	return fmt.Sprintf("header literal exceeds %d bytes", e.LimitBytes)
}

// RawMessageStreamHandler streams one raw RFC822 literal to its destination.
// The handler must fully consume body unless it returns an error; the caller
// drains any remaining literal data so the IMAP stream stays aligned.
type RawMessageStreamHandler func(uid uint32, body io.Reader) (int64, error)

const (
	rawMessageFetchIdleTimeout = 2 * time.Minute
	maxBackgroundRawBodyBytes  = 25 * 1024 * 1024
	maxHeaderLiteralBytes      = 2 * 1024 * 1024
)

type rawMessageProgressReader struct {
	reader io.Reader
	mark   func()
}

func (r rawMessageProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.mark != nil {
		r.mark()
	}
	return n, err
}

func readRawMessageLiteralWithLimit(uid uint32, literal io.Reader, limitBytes int64) ([]byte, error) {
	rawBytes, err := io.ReadAll(io.LimitReader(literal, limitBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(rawBytes)) > limitBytes {
		_, _ = io.Copy(io.Discard, literal)
		return nil, RawMessageTooLargeError{UID: uid, LimitBytes: limitBytes}
	}
	return rawBytes, nil
}

func readHeaderLiteralWithLimit(literal io.Reader) ([]byte, error) {
	headerBytes, err := io.ReadAll(io.LimitReader(literal, maxHeaderLiteralBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(headerBytes)) > maxHeaderLiteralBytes {
		_, _ = io.Copy(io.Discard, literal)
		return nil, HeaderLiteralTooLargeError{LimitBytes: maxHeaderLiteralBytes}
	}
	return headerBytes, nil
}

// FetchMessageBody fetches the body for a single message on-demand.
// Uses streaming fetch internally to avoid blocking on .Collect().
func (e *Engine) FetchMessageBody(ctx context.Context, accountID, messageID string) (*message.Message, error) {
	// Get the complete local identity so BODY[] can be cross-checked before it
	// is allowed to update this row.
	localMessage, err := e.messageStore.Get(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message info: %w", err)
	}
	if localMessage == nil {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}
	uid := localMessage.UID
	folderID := localMessage.FolderID

	// Get folder to get path
	f, err := e.folderStore.Get(folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}
	if f == nil {
		return nil, fmt.Errorf("folder not found: %s", folderID)
	}
	if err := f.RequireSelectable(); err != nil {
		return nil, err
	}

	e.log.Debug().
		Str("messageID", messageID).
		Uint32("uid", uid).
		Str("folder", f.Path).
		Msg("Fetching message body on-demand")

	// Get a connection from the pool
	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	discardConnection := false
	defer func() {
		if discardConnection {
			e.pool.Discard(conn)
		} else {
			e.pool.Release(conn)
		}
	}()

	// Select the mailbox
	_, err = conn.Client().SelectMailbox(ctx, f.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Use fetchMessageBodiesBatch for streaming fetch (avoids .Collect() blocking)
	targets := map[uint32]bodyFetchTarget{
		uid: {LocalMessageID: messageID, RFCMessageID: localMessage.MessageID},
	}
	results, skipped, err := e.fetchMessageBodiesBatch(ctx, conn.Client().RawClient(), targets)
	if err != nil {
		if IsMessageIdentityMismatchError(err) || imapPkg.IsConnectionError(err) {
			discardConnection = true
		}
		return nil, fmt.Errorf("fetch body failed: %w", err)
	}

	result, ok := results[uid]
	if !ok || result == nil {
		if skippedResult, skippedOK := skipped[uid]; skippedOK {
			if skippedResult.Reason == bodyFetchSkipTooLarge {
				return nil, RawMessageTooLargeError{UID: uid, LimitBytes: maxBackgroundRawBodyBytes}
			}
			return nil, fmt.Errorf("message body unavailable: UID %d returned no usable body", uid)
		}
		return nil, RawMessageNotFoundError{UID: uid}
	}

	// Replace body and attachment metadata together so a successful re-fetch
	// cannot leave stale or duplicated attachment records behind.
	if err := e.messageStore.RestoreBody(messageID, result.BodyHTML, result.BodyText, result.Snippet, result.HasAttachments, result.Attachments); err != nil {
		return nil, fmt.Errorf("failed to update message body: %w", err)
	}

	// Return updated message
	return e.messageStore.Get(messageID)
}

// fetchMessageBodiesBatch fetches bodies for multiple messages in a single IMAP command
// The mailbox must already be selected by the caller.
// Returns a map of UID -> ProcessedBody for successfully fetched messages and a
// map of UID -> bodyFetchSkipped for UIDs the server returned but that could not
// be processed without corrupting or overloading the background fetch path.
//
// Uses streaming (Next() loop) instead of Collect() to:
// - Avoid indefinite blocking if connection hangs
// - Allow context cancellation between messages
// - Return partial results if connection dies mid-batch
func (e *Engine) fetchMessageBodiesBatch(ctx context.Context, client *imapclient.Client, targets map[uint32]bodyFetchTarget) (map[uint32]*ProcessedBody, map[uint32]bodyFetchSkipped, error) {
	if len(targets) == 0 {
		return make(map[uint32]*ProcessedBody), make(map[uint32]bodyFetchSkipped), nil
	}

	// Check context
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	// Build UID set for batch fetch
	uidSet := imap.UIDSet{}
	for uid := range targets {
		uidSet.AddNum(imap.UID(uid))
	}

	e.log.Debug().
		Int("count", len(targets)).
		Msg("Fetching message bodies in batch")

	fetchOptions := &imap.FetchOptions{
		UID: true,
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierNone, // Full message
				Peek:      true,                   // Don't mark as read
			},
		},
		RFC822Size: true,
	}

	fetchCmd := client.Fetch(uidSet, fetchOptions)
	results := make(map[uint32]*ProcessedBody)
	skipped := make(map[uint32]bodyFetchSkipped)

	// Stream messages one at a time instead of blocking on Collect()
	// This allows cancellation between messages and returns partial results on error
	for {
		// Check for cancellation between messages
		if ctx.Err() != nil {
			fetchCmd.Close()
			e.log.Warn().
				Int("fetched", len(results)).
				Int("requested", len(targets)).
				Msg("Fetch cancelled, returning partial results")
			return results, skipped, ctx.Err()
		}

		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		// Extract UID and body section from streamed message
		var fetchedUID imap.UID
		var rawBytes []byte
		var gotBodySection bool
		var bodyTooLarge bool
		var reportedSize int64 // RFC822.SIZE; 0 if server didn't return it
		var receivedBytes int64

		for {
			item := msg.Next()
			if item == nil {
				break
			}

			switch data := item.(type) {
			case imapclient.FetchItemDataUID:
				fetchedUID = data.UID
			case imapclient.FetchItemDataRFC822Size:
				// Captured so shouldChargeFailure can compare against bytes
				// actually read and detect server-side truncation before
				// persisting body_failed = 1.
				reportedSize = int64(data.Size)
			case imapclient.FetchItemDataBodySection:
				gotBodySection = true
				// Read the full BODY[] literal. Provider-side mailbox limits already
				// constrain message size, and partial reads corrupt raw .eml data.
				if data.Literal != nil {
					var err error
					rawBytes, err = io.ReadAll(io.LimitReader(data.Literal, maxBackgroundRawBodyBytes+1))
					receivedBytes = int64(len(rawBytes))
					if err != nil {
						e.log.Warn().
							Err(err).
							Uint32("uid", uint32(fetchedUID)).
							Msg("Failed to read body literal, continuing with partial data")
						// Keep whatever we got (may be partial)
					}
					if len(rawBytes) > maxBackgroundRawBodyBytes {
						if _, drainErr := io.Copy(io.Discard, data.Literal); drainErr != nil {
							e.log.Warn().Err(drainErr).Uint32("uid", uint32(fetchedUID)).Msg("Failed to drain oversized body literal")
						}
						bodyTooLarge = true
						rawBytes = nil
						e.log.Warn().
							Uint32("uid", uint32(fetchedUID)).
							Int64("maxBytes", maxBackgroundRawBodyBytes).
							Msg("Skipping oversized message body during background sync")
					}
				} else {
					e.log.Warn().
						Uint32("uid", uint32(fetchedUID)).
						Msg("Body section has nil Literal reader")
				}
			}
		}

		// Log if we didn't receive a body section at all
		if !gotBodySection && fetchedUID != 0 {
			e.log.Warn().
				Uint32("uid", uint32(fetchedUID)).
				Msg("No body section in IMAP response for message")
		}

		uid := uint32(fetchedUID)
		if uid == 0 {
			e.log.Warn().Msg("Received message without UID in batch response")
			continue
		}

		target, ok := targets[uid]
		if !ok {
			e.log.Warn().Uint32("uid", uid).Msg("Received unexpected UID in batch response")
			continue
		}

		if len(rawBytes) == 0 {
			if bodyTooLarge {
				skipped[uid] = bodyFetchSkipped{
					Reason:        bodyFetchSkipTooLarge,
					ReportedSize:  reportedSize,
					ReceivedBytes: receivedBytes,
				}
				continue
			}
			e.log.Warn().Uint32("uid", uid).Str("messageID", target.LocalMessageID).Msg("Empty message body returned by IMAP")
			skipped[uid] = bodyFetchSkipped{
				Reason:        bodyFetchSkipEmpty,
				ReportedSize:  reportedSize,
				ReceivedBytes: receivedBytes,
			}
			continue
		}

		e.log.Debug().
			Uint32("uid", uid).
			Int("bodySize", len(rawBytes)).
			Msg("Processing message body")

		if err := validateFetchedMessageIdentity(uid, target.RFCMessageID, rawBytes); err != nil {
			e.log.Error().Err(err).
				Str("localMessageID", target.LocalMessageID).
				Msg("Refusing to persist body returned for a different message")
			// Do not wait for the rest of this command: callers discard the
			// connection on this error, which safely terminates the in-flight FETCH.
			return nil, nil, err
		}

		processed := e.processRawMessageBody(rawBytes, target.LocalMessageID)
		processed.ReportedSize = reportedSize
		results[uid] = processed
	}

	if err := fetchCmd.Close(); err != nil {
		e.log.Warn().Err(err).
			Int("fetched", len(results)).
			Int("requested", len(targets)).
			Msg("Fetch close error, returning partial results")
		// Return what we have, don't fail completely
		// Partial content is better than no content
	}

	e.log.Debug().
		Int("fetched", len(results)).
		Int("requested", len(targets)).
		Msg("Batch fetch complete")

	return results, skipped, nil
}

// bodyTruncationThreshold is the fraction of the IMAP-reported size below
// which a received body is considered suspiciously short — likely truncated
// in transit rather than legitimately empty. Tuned for the typical case of
// a multipart message: small IMAP framing variations stay above this floor,
// but a real loss of bytes (a clearly-cut-off response) falls under it.
const bodyTruncationThreshold = 0.8

// shouldChargeFailure decides whether a message that came back with no
// usable body should be persisted as body_failed=1. Returns false when the
// signals suggest the failure was a transient truncation, in which case
// the next sync may succeed — so we want to retry rather than permanently
// skip the message.
//
// Decision table (all comparisons in bytes):
//
//	reportedSize == 0          → charge   (no signal to defer on; treat as definitive)
//	received    <  reported*T  → DON'T    (clear shortfall; likely server-side truncation)
//	otherwise                  → charge   (received is close enough to expected; the empty body is real)
//
// Kept as a pure function (no Engine receiver) so it can be unit-tested
// against synthetic inputs without standing up a full sync engine.
func shouldChargeFailure(receivedBytes, reportedSize int64) bool {
	if reportedSize <= 0 {
		return true
	}
	threshold := int64(float64(reportedSize) * bodyTruncationThreshold)
	return receivedBytes >= threshold
}

// markUnresolvedAsFailed persists messages.body_failed=1 for any requested ID
// whose body parsed to empty AND wasn't encrypted, OR whose UID the server
// didn't return at all. Encrypted messages (which legitimately carry empty
// plaintext until view-time decryption) are treated as resolved. The flag
// excludes the message from future body-fetch queries via the body_failed=0
// guard in GetMessagesWithoutBody/AndSize/Count (see migration v39).
//
// sizes carries the per-message (received, reported) byte counts captured
// during FETCH so shouldChargeFailure can defer marking when the response
// was likely server-side truncation. IDs absent from sizes — typically
// requested-but-not-returned — fall through to the "no signal" branch and
// are charged (server returned nothing despite RFC822.SIZE being requested:
// either a server bug or a persistent permission issue, both worth bounding).
//
// Without this persistence, the previous in-memory failedParseAttempts map
// reset every sync cycle and re-fetched the same unparseable IDs forever.
// fetchedSize records what the IMAP server said the message size was
// (RFC822.SIZE) vs what we actually read from the BODY[] literal. Used by
// shouldChargeFailure to decide whether to defer marking a message failed.
type fetchedSize struct {
	received int64
	reported int64
}

// bodyFetchProgress is shared with the heartbeat logger while a body-fetch
// loop updates its counters. Atomic counters keep long-running sync telemetry
// race-free without putting the single-writer processing path behind a mutex.
type bodyFetchProgress struct {
	fetched atomic.Int64
	failed  atomic.Int64
}

type bodyProcessingResult struct {
	requestedIDs []string
	bodyUpdates  []message.BodyUpdate
	attachments  []*message.Attachment
	sizes        map[string]fetchedSize
	fetchedCount int
}

func (p *bodyFetchProgress) addFetched(delta int) {
	p.fetched.Add(int64(delta))
}

func (p *bodyFetchProgress) addFailed(delta int) {
	p.failed.Add(int64(delta))
}

func (p *bodyFetchProgress) snapshot() (fetched, failed int64) {
	return p.fetched.Load(), p.failed.Load()
}

func (p *bodyFetchProgress) fetchedCount() int {
	return int(p.fetched.Load())
}

func (p *bodyFetchProgress) failedCount() int {
	return int(p.failed.Load())
}

type bodyFetchBatchPlan struct {
	messageIDs   []string
	oversizedIDs []string
	bytes        int64
	maxMessages  int
	maxBytes     int64
}

func planBodyFetchBatch(candidates []message.MessageWithSize, totalWithoutBody int) bodyFetchBatchPlan {
	plan := bodyFetchBatchPlan{
		maxMessages: bodyBatchMaxMessages,
		maxBytes:    int64(bodyBatchMaxBytes),
	}
	if totalWithoutBody > 1000 {
		plan.maxMessages = 25
		plan.maxBytes = 256 * 1024
	}

	for _, candidate := range candidates {
		messageSize := int64(candidate.Size)
		if messageSize <= 0 {
			messageSize = 10 * 1024
		}
		if messageSize > maxBackgroundRawBodyBytes {
			plan.oversizedIDs = append(plan.oversizedIDs, candidate.ID)
			continue
		}

		wouldExceedBytes := plan.bytes+messageSize > plan.maxBytes && len(plan.messageIDs) >= bodyBatchMinMessages
		wouldExceedCount := len(plan.messageIDs) >= plan.maxMessages
		if wouldExceedBytes || wouldExceedCount {
			break
		}

		plan.messageIDs = append(plan.messageIDs, candidate.ID)
		plan.bytes += messageSize
	}

	return plan
}

type bodyFetchConnectionPool interface {
	GetConnection(context.Context, string) (*imapPkg.PooledConnection, error)
	Discard(*imapPkg.PooledConnection)
	Release(*imapPkg.PooledConnection)
}

type bodyFetchMailboxSelector func(context.Context, *imapPkg.PooledConnection, string) error

func recoverBodyFetchConnection(
	ctx context.Context,
	pool bodyFetchConnectionPool,
	failed *imapPkg.PooledConnection,
	accountID string,
	mailboxPath string,
	selectMailbox bodyFetchMailboxSelector,
) (*imapPkg.PooledConnection, error) {
	pool.Discard(failed)

	conn, err := pool.GetConnection(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get new connection after error: %w", err)
	}
	if err := selectMailbox(ctx, conn, mailboxPath); err != nil {
		pool.Release(conn)
		return nil, fmt.Errorf("failed to select mailbox on new connection: %w", err)
	}
	return conn, nil
}

func (e *Engine) applyBodyProcessingResult(result bodyProcessingResult, progress *bodyFetchProgress, accountID, folderID string, total int) {
	e.markUnresolvedAsFailed(result.requestedIDs, result.bodyUpdates, result.sizes)

	if len(result.bodyUpdates) == 0 {
		e.log.Warn().Int("fetchedCount", result.fetchedCount).Msg("No body updates in result")
	} else if err := e.messageStore.UpdateBodiesBatch(result.bodyUpdates); err != nil {
		e.log.Warn().Err(err).Msg("Failed to batch update bodies")
		progress.addFailed(result.fetchedCount)
	} else {
		progress.addFetched(result.fetchedCount)
		e.log.Debug().Int("fetched", progress.fetchedCount()).Int("total", total).Msg("DB update successful")
	}

	if len(result.attachments) > 0 {
		if err := e.attachmentStore.CreateBatch(result.attachments); err != nil {
			e.log.Warn().Err(err).Msg("Failed to batch create attachments")
		}
	}

	e.log.Debug().Int("fetched", progress.fetchedCount()).Int("total", total).Msg("Emitting progress")
	e.emitProgress(accountID, folderID, progress.fetchedCount(), total, "bodies")
}

func skippedBodySizesByMessageID(uidToMessageID map[uint32]string, skipped map[uint32]bodyFetchSkipped) map[string]fetchedSize {
	if len(skipped) == 0 {
		return nil
	}
	sizes := make(map[string]fetchedSize, len(skipped))
	for uid, skip := range skipped {
		messageID := uidToMessageID[uid]
		if messageID == "" {
			continue
		}
		if skip.Reason == bodyFetchSkipTooLarge {
			// Oversized messages are an intentional background-sync skip, not a
			// transient truncation. Force shouldChargeFailure down the "charge"
			// branch so they are marked body_failed and do not loop forever.
			sizes[messageID] = fetchedSize{received: skip.ReceivedBytes, reported: 0}
			continue
		}
		sizes[messageID] = fetchedSize{received: skip.ReceivedBytes, reported: skip.ReportedSize}
	}
	return sizes
}

func (e *Engine) markUnresolvedAsFailed(requestedIDs []string, updates []message.BodyUpdate, sizes map[string]fetchedSize) {
	if len(requestedIDs) == 0 {
		return
	}
	resolved := make(map[string]bool, len(updates))
	for _, u := range updates {
		if u.BodyHTML != "" || u.BodyText != "" {
			resolved[u.MessageID] = true
		}
	}
	var failedIDs []string
	var deferredCount int
	for _, id := range requestedIDs {
		if resolved[id] {
			continue
		}
		// Defer marking when the response looks truncated — retrying next
		// sync may succeed. shouldChargeFailure returns true when no size
		// signal exists (id not in sizes map → zero values → reported=0).
		s := sizes[id]
		if !shouldChargeFailure(s.received, s.reported) {
			deferredCount++
			continue
		}
		failedIDs = append(failedIDs, id)
	}
	if deferredCount > 0 {
		e.log.Info().Int("count", deferredCount).Msg("Deferred marking bodies as parse-failed; FETCH looked truncated, will retry next sync")
	}
	if len(failedIDs) == 0 {
		return
	}
	if err := e.messageStore.MarkBodyFailed(failedIDs); err != nil {
		e.log.Warn().Err(err).Int("count", len(failedIDs)).Msg("Failed to persist body-parse-failed flag")
		return
	}
	e.log.Info().Int("count", len(failedIDs)).Msg("Marked message bodies as parse-failed; excluded from future sync")
}

// FetchBodiesInBackground fetches bodies for messages that don't have them yet.
// This is called after headers sync to fetch bodies in the background.
// syncPeriodDays limits body fetching to messages within the sync period (0 = all messages).
//
// OPTIMIZED: Uses batch IMAP FETCH to fetch multiple message bodies in a single command,
// reducing network round-trips significantly. Uses hybrid byte+count batching (like Geary)
// for memory safety and efficiency:
//   - Max 512KB per batch (memory bounded)
//   - Max 50 messages per batch (even if small)
//   - Min 1 message per batch (handles oversized emails)
//
// Pipeline design for maximum throughput:
//  1. Wait for previous batch's goroutine (if any)
//  2. Apply DB updates from previous batch
//  3. Query candidates and build byte-aware batch
//  4. Fetch bodies via IMAP
//  5. Launch goroutine to parse/sanitize (DB update happens in step 2 of next iteration)
//  6. Repeat
//
// This allows IMAP fetch (network-bound) to run in parallel with parsing (CPU-bound).
// DB updates are synchronous relative to the next DB query to prevent race conditions.
//
// Uses a single IMAP connection for efficiency (reuses connection for all body fetches).
// Includes error recovery: on connection errors, discards dead connection and gets a new one.
// Returns error only if connection recovery fails - individual message failures are logged and skipped.
func (e *Engine) FetchBodiesInBackground(ctx context.Context, accountID, folderID string, syncPeriodDays int) error {
	// Check context at start
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Get folder to get path
	f, err := e.folderStore.Get(folderID)
	if err != nil {
		return fmt.Errorf("failed to get folder: %w", err)
	}
	if f == nil {
		return fmt.Errorf("folder not found: %s", folderID)
	}
	if err := f.RequireSelectable(); err != nil {
		return err
	}

	// Calculate sync date cutoff
	var sinceDate time.Time
	if syncPeriodDays > 0 {
		sinceDate = time.Now().AddDate(0, 0, -syncPeriodDays)
	}

	e.log.Debug().
		Str("account", accountID).
		Str("folder", f.Path).
		Int("syncPeriodDays", syncPeriodDays).
		Msg("Fetching message bodies in background (hybrid batch mode)")

	// Get a SINGLE connection from the pool - reused for all body fetches
	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	// Note: We manage connection lifecycle manually due to recovery logic
	// Don't use defer e.pool.Release(conn) - we handle it explicitly

	// Select the mailbox ONCE
	_, err = conn.Client().SelectMailbox(ctx, f.Path)
	if err != nil {
		e.pool.Release(conn)
		return fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Get total count of messages without body (respecting sync period)
	totalWithoutBody, err := e.messageStore.CountMessagesWithoutBody(folderID, sinceDate)
	if err != nil {
		e.pool.Release(conn)
		return fmt.Errorf("failed to count messages without body: %w", err)
	}

	if totalWithoutBody == 0 {
		e.log.Debug().Msg("All messages have bodies, nothing to fetch")
		// Emit 1/1 so frontend shows 100% complete for bodies phase
		e.emitProgress(accountID, folderID, 1, 1, "bodies")
		e.pool.Release(conn)
		return nil
	}

	e.log.Info().Int("count", totalWithoutBody).Msg("Fetching message bodies (hybrid batch mode)")

	// Emit initial progress so frontend knows body fetch has started
	e.emitProgress(accountID, folderID, 0, totalWithoutBody, "bodies")

	// Tracking for error recovery and progress
	failedBatches := 0      // consecutive batch failures
	connectionFailures := 0 // total connection recovery attempts
	var progress bodyFetchProgress

	// Note: parse-failure tracking is persisted in messages.body_failed (v39).
	// Once a fetch returns nothing usable for a message, MarkBodyFailed flags
	// it and GetMessagesWithoutBodyAndSize excludes it from future queries —
	// so no in-memory dedup is needed and the cap survives across sessions.

	// Channel and pending state for pipelined processing
	var pendingResultChan chan bodyProcessingResult

	// Start heartbeat logging for long operations - shows sync is alive during long fetches
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fetched, failed := progress.snapshot()
				e.log.Info().
					Int64("fetched", fetched).
					Int("total", totalWithoutBody).
					Int64("failed", failed).
					Str("folder", f.Path).
					Msg("Body fetch in progress (heartbeat)")
			case <-heartbeatCtx.Done():
				return
			}
		}
	}()

	for {
		// Step 1: Wait for previous batch's goroutine (if any)
		// Step 2: Apply DB updates from previous batch
		if pendingResultChan != nil {
			e.log.Debug().Msg("Waiting for previous batch goroutine to complete")
			result := <-pendingResultChan
			e.log.Debug().
				Int("bodyUpdates", len(result.bodyUpdates)).
				Int("attachments", len(result.attachments)).
				Int("fetchedCount", result.fetchedCount).
				Msg("Received result from processing goroutine")

			e.applyBodyProcessingResult(result, &progress, accountID, folderID, totalWithoutBody)
			pendingResultChan = nil
		}

		// Check context before starting new batch
		if ctx.Err() != nil {
			e.log.Debug().Msg("Body fetch cancelled")
			e.pool.Release(conn)
			return ctx.Err()
		}

		// Step 3: Query candidates and build byte-aware batch
		// Get more candidates than we'll use to allow for byte-based selection
		candidates, err := e.messageStore.GetMessagesWithoutBodyAndSize(folderID, bodyBatchQueryLimit, sinceDate)
		if err != nil {
			e.pool.Release(conn)
			return fmt.Errorf("failed to get messages without body: %w", err)
		}

		e.log.Debug().
			Int("candidates", len(candidates)).
			Int("fetched", progress.fetchedCount()).
			Int("failed", progress.failedCount()).
			Msg("Queried candidates for next batch")

		if len(candidates) == 0 {
			e.log.Debug().Msg("No more candidates, body sync complete")
			break // All done
		}

		// Note: candidates already excludes any message with body_failed=1
		// (enforced by GetMessagesWithoutBodyAndSize since v39), so the
		// in-memory filter that used to live here is gone.

		batchPlan := planBodyFetchBatch(candidates, totalWithoutBody)
		if totalWithoutBody > 1000 {
			// Log only once (when we first enter the large mailbox mode)
			if progress.fetchedCount() == 0 && progress.failedCount() == 0 {
				e.log.Info().
					Int("totalMessages", totalWithoutBody).
					Int("batchMaxMessages", batchPlan.maxMessages).
					Int64("batchMaxBytes", batchPlan.maxBytes).
					Msg("Using smaller batches for large mailbox")
			}
		}

		batchIDs := batchPlan.messageIDs
		if len(batchPlan.oversizedIDs) > 0 {
			e.log.Warn().
				Int("count", len(batchPlan.oversizedIDs)).
				Int64("maxBytes", maxBackgroundRawBodyBytes).
				Msg("Skipping oversized message bodies during background sync")
			if err := e.messageStore.MarkBodyFailed(batchPlan.oversizedIDs); err != nil {
				e.log.Warn().Err(err).Int("count", len(batchPlan.oversizedIDs)).Msg("Failed to mark oversized bodies as skipped")
			} else {
				progress.addFailed(len(batchPlan.oversizedIDs))
			}
		}

		if len(batchIDs) == 0 {
			if len(batchPlan.oversizedIDs) > 0 {
				continue
			}
			e.log.Warn().Msg("No messages selected for batch")
			break
		}

		e.log.Debug().
			Int("batchSize", len(batchIDs)).
			Int64("batchBytes", batchPlan.bytes).
			Msg("Processing batch")

		// Get UIDs for all messages in batch (single DB query)
		uidInfos, err := e.messageStore.GetMessageUIDsAndFolder(batchIDs)
		if err != nil {
			e.log.Warn().Err(err).Msg("Failed to get UIDs for batch, skipping")
			failedBatches++
			if failedBatches > maxMessageRetries {
				e.log.Error().Int("failedBatches", failedBatches).Msg("Too many consecutive batch failures")
				break
			}
			continue
		}

		// Build UID -> messageID map for batch fetch
		uidToMessageID := make(map[uint32]string)
		fetchTargets := make(map[uint32]bodyFetchTarget)
		for msgID, info := range uidInfos {
			if info.FolderID != folderID {
				e.log.Warn().
					Str("messageID", msgID).
					Str("expectedFolderID", folderID).
					Str("actualFolderID", info.FolderID).
					Msg("Skipping body candidate that moved to another folder")
				continue
			}
			uidToMessageID[info.UID] = msgID
			fetchTargets[info.UID] = bodyFetchTarget{
				LocalMessageID: msgID,
				RFCMessageID:   info.RFCMessageID,
			}
		}

		if len(uidToMessageID) == 0 {
			e.log.Warn().Int("requested", len(batchIDs)).Msg("No valid UIDs found for batch")
			continue
		}

		// Step 4: Fetch bodies via IMAP - single round-trip for all messages in batch
		bodies, skipped, fetchErr := e.fetchMessageBodiesBatch(ctx, conn.Client().RawClient(), fetchTargets)
		if fetchErr != nil {
			// A mismatched Message-ID proves that this connection is selected on
			// the wrong mailbox (or returned corrupted data). Treat it exactly like
			// a broken connection: discard it and retry the batch after SELECT on a
			// fresh connection.
			if imapPkg.IsConnectionError(fetchErr) || IsMessageIdentityMismatchError(fetchErr) {
				connectionFailures++

				// Check if we've exhausted connection recovery attempts
				if connectionFailures > maxConnectionRetries {
					e.log.Error().
						Err(fetchErr).
						Int("connectionFailures", connectionFailures).
						Msg("Body fetch aborted - connection recovery failed")
					e.pool.Discard(conn)
					return fmt.Errorf("body fetch recovery failed after %d attempts: %w", connectionFailures, fetchErr)
				}

				e.log.Debug().
					Err(fetchErr).
					Int("attempt", connectionFailures).
					Msg("Body fetch connection is unusable, attempting recovery")

				conn, err = recoverBodyFetchConnection(ctx, e.pool, conn, accountID, f.Path, func(ctx context.Context, conn *imapPkg.PooledConnection, mailboxPath string) error {
					_, selectErr := conn.Client().SelectMailbox(ctx, mailboxPath)
					return selectErr
				})
				if err != nil {
					return err
				}

				e.log.Debug().Msg("Connection recovered successfully, retrying batch")
				continue // Retry same batch
			}

			// Non-connection error
			e.log.Warn().Err(fetchErr).Msg("Batch fetch failed with non-connection error")
			failedBatches++
			if failedBatches > maxMessageRetries {
				e.log.Error().Int("failedBatches", failedBatches).Msg("Too many consecutive batch failures")
				break
			}
			continue
		}

		// Reset failure counters on success
		failedBatches = 0

		// If we got no bodies or explicit skipped results back, persist the
		// failure for every requested ID so they are excluded from future
		// syncs forever — otherwise the next cycle would query and FETCH the
		// same UIDs again.
		if len(bodies) == 0 && len(skipped) == 0 {
			e.log.Warn().Int("requested", len(uidToMessageID)).Msg("IMAP returned no bodies for batch")
			// No size signals here — server returned nothing, so every ID
			// falls through to the "no signal" branch of shouldChargeFailure
			// and gets charged. Pass nil to keep the call sites uniform.
			e.markUnresolvedAsFailed(batchIDs, nil, nil)
			progress.addFailed(len(uidToMessageID))
			continue
		}
		if len(bodies) == 0 {
			e.log.Warn().Int("requested", len(uidToMessageID)).Int("skipped", len(skipped)).Msg("IMAP returned only skipped bodies for batch")
			e.markUnresolvedAsFailed(batchIDs, nil, skippedBodySizesByMessageID(uidToMessageID, skipped))
			progress.addFailed(len(uidToMessageID))
			continue
		}

		// Step 5: Launch goroutine to build body updates
		// DB update will happen in step 2 of the NEXT iteration
		// Attachments were already extracted during parsing - no re-parse needed!
		resultChan := make(chan bodyProcessingResult, 1)
		currentBodies := bodies // capture for goroutine
		currentSkipped := skipped
		currentUIDToMessageID := uidToMessageID

		go func() {
			startTime := time.Now()
			var bodyUpdates []message.BodyUpdate
			var allAttachments []*message.Attachment
			sizes := make(map[string]fetchedSize, len(currentBodies)+len(currentSkipped))
			for messageID, size := range skippedBodySizesByMessageID(currentUIDToMessageID, currentSkipped) {
				sizes[messageID] = size
			}

			for _, pb := range currentBodies {
				// Build body update
				bu := message.BodyUpdate{
					MessageID:      pb.MessageID,
					BodyHTML:       pb.BodyHTML,
					BodyText:       pb.BodyText,
					Snippet:        pb.Snippet,
					HasAttachments: pb.HasAttachments,
				}
				bodyUpdates = append(bodyUpdates, bu)

				sizes[pb.MessageID] = fetchedSize{received: pb.ReceivedBytes, reported: pb.ReportedSize}

				// Use pre-extracted attachments (no re-parsing!)
				if len(pb.Attachments) > 0 {
					allAttachments = append(allAttachments, pb.Attachments...)
				}
			}

			e.log.Debug().
				Int("bodyUpdates", len(bodyUpdates)).
				Int("attachments", len(allAttachments)).
				Dur("elapsed", time.Since(startTime)).
				Msg("Built body updates and attachments for batch")

			resultChan <- bodyProcessingResult{
				requestedIDs: batchIDs,
				bodyUpdates:  bodyUpdates,
				attachments:  allAttachments,
				sizes:        sizes,
				fetchedCount: len(currentBodies),
			}
		}()

		// Mark that we have pending work - will be processed in step 1-2 of next iteration
		pendingResultChan = resultChan
	}

	// Handle final batch if there's pending work
	if pendingResultChan != nil {
		result := <-pendingResultChan

		e.applyBodyProcessingResult(result, &progress, accountID, folderID, totalWithoutBody)
	}

	// Release connection when done
	e.pool.Release(conn)

	// Log summary
	if progress.failedCount() > 0 {
		e.log.Info().
			Int("fetched", progress.fetchedCount()).
			Int("failed", progress.failedCount()).
			Int("total", totalWithoutBody).
			Msg("Body fetch complete with failures (hybrid batch mode)")
	} else {
		e.log.Info().
			Int("fetched", progress.fetchedCount()).
			Int("total", totalWithoutBody).
			Msg("Body fetch complete (hybrid batch mode)")
	}

	return nil
}

// FetchRawMessage fetches the raw RFC822 content of a message from the IMAP server.
// Uses streaming fetch to avoid blocking on .Collect().
func (e *Engine) FetchRawMessage(ctx context.Context, accountID, folderID string, uid uint32) ([]byte, error) {
	// Get folder path
	f, err := e.folderStore.Get(folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}
	if f == nil {
		return nil, fmt.Errorf("folder not found: %s", folderID)
	}
	if err := f.RequireSelectable(); err != nil {
		return nil, err
	}

	// Get a connection from the pool
	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer e.pool.Release(conn)

	// Select the mailbox
	_, err = conn.Client().SelectMailbox(ctx, f.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	// Fetch the raw message
	uidSet := imap.UIDSet{}
	uidSet.AddNum(imap.UID(uid))

	fetchOptions := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierNone,
				Peek:      true,
			},
		},
	}

	fetchCmd := conn.Client().RawClient().Fetch(uidSet, fetchOptions)

	// Stream the single message instead of blocking on Collect()
	var rawBytes []byte

	msg := fetchCmd.Next()
	if msg == nil {
		fetchCmd.Close()
		return nil, RawMessageNotFoundError{UID: uid}
	}

	// Extract body section from streamed message
	for {
		item := msg.Next()
		if item == nil {
			break
		}

		if data, ok := item.(imapclient.FetchItemDataBodySection); ok {
			if data.Literal != nil {
				rawBytes, err = readRawMessageLiteralWithLimit(uid, data.Literal, maxBackgroundRawBodyBytes)
				if err != nil {
					fetchCmd.Close()
					return nil, fmt.Errorf("failed to read message body: %w", err)
				}
				break
			}
		}
	}

	fetchCmd.Close()

	if len(rawBytes) == 0 {
		return nil, fmt.Errorf("message body not found: UID %d", uid)
	}

	return rawBytes, nil
}

// StreamRawMessage streams the full raw RFC822 content of a message from the
// IMAP server to dst without applying a client-side size cap. This is intended
// for backup/export paths where truncating the MIME payload would corrupt the
// resulting .eml file.
func (e *Engine) StreamRawMessage(ctx context.Context, accountID, folderID string, uid uint32, dst io.Writer) (*RawMessageStreamResult, error) {
	f, err := e.folderStore.Get(folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}
	if f == nil {
		return nil, fmt.Errorf("folder not found: %s", folderID)
	}
	if err := f.RequireSelectable(); err != nil {
		return nil, err
	}

	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer e.pool.Release(conn)

	if _, err := conn.Client().SelectMailbox(ctx, f.Path); err != nil {
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	uidSet := imap.UIDSet{}
	uidSet.AddNum(imap.UID(uid))

	fetchOptions := &imap.FetchOptions{
		RFC822Size: true,
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierNone,
				Peek:      true,
			},
		},
	}
	fetchCmd := conn.Client().RawClient().Fetch(uidSet, fetchOptions)

	msg := fetchCmd.Next()
	if msg == nil {
		fetchCmd.Close()
		return nil, fmt.Errorf("message not found: UID %d", uid)
	}

	result := &RawMessageStreamResult{}
	var gotBodySection bool
	var copyErr error

	for {
		item := msg.Next()
		if item == nil {
			break
		}

		switch data := item.(type) {
		case imapclient.FetchItemDataRFC822Size:
			result.ReportedSize = data.Size
		case imapclient.FetchItemDataBodySection:
			if data.Literal == nil {
				continue
			}
			gotBodySection = true
			result.BytesWritten, copyErr = io.Copy(dst, data.Literal)
		}
	}

	fetchCmd.Close()

	if copyErr != nil {
		return result, fmt.Errorf("failed to stream message body: %w", copyErr)
	}
	if err := rawMessageStreamError(gotBodySection, result, uid); err != nil {
		return result, err
	}
	if rawMessageSizeMismatch(result) {
		e.log.Warn().
			Uint32("uid", uid).
			Int64("reportedSize", result.ReportedSize).
			Int64("bytesWritten", result.BytesWritten).
			Msg("Raw message size differs from RFC822.SIZE; keeping streamed backup data")
	}

	return result, nil
}

// StreamRawMessages streams full raw RFC822 content for multiple UIDs from a
// single selected mailbox. It keeps per-message failures isolated so backup can
// continue after a corrupt, missing, or transiently unavailable message.
func (e *Engine) StreamRawMessages(ctx context.Context, accountID, folderID string, uids []uint32, handle RawMessageStreamHandler) (map[uint32]*RawMessageStreamResult, map[uint32]error, error) {
	results := make(map[uint32]*RawMessageStreamResult)
	failures := make(map[uint32]error)
	if len(uids) == 0 {
		return results, failures, nil
	}
	if handle == nil {
		return nil, nil, errors.New("raw message stream handler is nil")
	}
	if err := ctx.Err(); err != nil {
		return results, failures, err
	}

	f, err := e.folderStore.Get(folderID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get folder: %w", err)
	}
	if f == nil {
		return nil, nil, fmt.Errorf("folder not found: %s", folderID)
	}
	if err := f.RequireSelectable(); err != nil {
		return nil, nil, err
	}

	conn, err := e.pool.GetConnection(ctx, accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer e.pool.Release(conn)

	activity := make(chan struct{}, 1)
	done := make(chan struct{})
	var timedOut atomic.Bool
	markActivity := func() {
		select {
		case activity <- struct{}{}:
		default:
		}
	}
	go func() {
		timer := time.NewTimer(rawMessageFetchIdleTimeout)
		defer timer.Stop()
		for {
			select {
			case <-activity:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(rawMessageFetchIdleTimeout)
			case <-timer.C:
				timedOut.Store(true)
				e.log.Warn().
					Str("account", accountID).
					Str("folder", f.Path).
					Dur("timeout", rawMessageFetchIdleTimeout).
					Msg("Raw message batch fetch stalled; discarding IMAP connection")
				e.pool.Discard(conn)
				return
			case <-done:
				return
			}
		}
	}()
	defer close(done)
	markActivity()

	if _, err := conn.Client().SelectMailbox(ctx, f.Path); err != nil {
		return nil, nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	uidSet := imap.UIDSet{}
	requested := make(map[uint32]bool, len(uids))
	for _, uid := range uids {
		if uid == 0 || requested[uid] {
			continue
		}
		requested[uid] = true
		uidSet.AddNum(imap.UID(uid))
	}
	if len(requested) == 0 {
		return results, failures, nil
	}

	fetchOptions := &imap.FetchOptions{
		UID:        true,
		RFC822Size: true,
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierNone,
				Peek:      true,
			},
		},
	}
	fetchCmd := conn.Client().RawClient().Fetch(uidSet, fetchOptions)
	seen := make(map[uint32]bool, len(requested))

	for {
		if err := ctx.Err(); err != nil {
			fetchCmd.Close()
			return results, failures, err
		}

		msg := fetchCmd.Next()
		if msg == nil {
			break
		}
		markActivity()

		result := &RawMessageStreamResult{}
		var fetchedUID imap.UID
		var gotBodySection bool
		var bodyErr error
		var bodyArrivedBeforeUID bool

		for {
			item := msg.Next()
			if item == nil {
				break
			}

			switch data := item.(type) {
			case imapclient.FetchItemDataUID:
				markActivity()
				fetchedUID = data.UID
			case imapclient.FetchItemDataRFC822Size:
				markActivity()
				result.ReportedSize = data.Size
			case imapclient.FetchItemDataBodySection:
				if data.Literal == nil {
					continue
				}
				markActivity()
				gotBodySection = true
				uid := uint32(fetchedUID)
				if uid == 0 {
					bodyArrivedBeforeUID = true
					_, _ = io.Copy(io.Discard, data.Literal)
					continue
				}

				result.BytesWritten, bodyErr = handle(uid, rawMessageProgressReader{
					reader: data.Literal,
					mark:   markActivity,
				})
				if bodyErr != nil {
					_, _ = io.Copy(io.Discard, data.Literal)
				}
			}
		}

		uid := uint32(fetchedUID)
		if uid == 0 {
			e.log.Warn().Msg("Received raw message without UID in batch response")
			continue
		}
		if !requested[uid] {
			e.log.Warn().Uint32("uid", uid).Msg("Received unexpected UID in raw batch response")
			continue
		}
		seen[uid] = true

		if bodyArrivedBeforeUID {
			failures[uid] = fmt.Errorf("message body arrived before UID: UID %d", uid)
			continue
		}
		if bodyErr != nil {
			failures[uid] = fmt.Errorf("failed to stream message body: %w", bodyErr)
			continue
		}
		if err := rawMessageStreamError(gotBodySection, result, uid); err != nil {
			failures[uid] = err
			continue
		}
		if rawMessageSizeMismatch(result) {
			e.log.Warn().
				Uint32("uid", uid).
				Int64("reportedSize", result.ReportedSize).
				Int64("bytesWritten", result.BytesWritten).
				Msg("Raw message size differs from RFC822.SIZE; keeping streamed backup data")
		}

		results[uid] = result
	}

	if err := fetchCmd.Close(); err != nil {
		e.log.Warn().
			Err(err).
			Int("fetched", len(results)).
			Int("requested", len(requested)).
			Msg("Raw batch fetch close error, keeping partial backup results")
	}
	if timedOut.Load() {
		return results, failures, fmt.Errorf("raw message batch fetch stalled for %s", rawMessageFetchIdleTimeout)
	}

	for uid := range requested {
		if !seen[uid] {
			failures[uid] = RawMessageNotFoundError{UID: uid}
		}
	}

	return results, failures, nil
}

func rawMessageStreamError(gotBodySection bool, result *RawMessageStreamResult, uid uint32) error {
	if result == nil || !gotBodySection || result.BytesWritten == 0 {
		return fmt.Errorf("message body not found: UID %d", uid)
	}
	return nil
}

func rawMessageSizeMismatch(result *RawMessageStreamResult) bool {
	return result != nil && result.ReportedSize > 0 && result.BytesWritten != result.ReportedSize
}

// buildMessageFromStreamedData constructs a Message from streamed IMAP data.
// Used by FetchServerMessage for server search results.
func (e *Engine) buildMessageFromStreamedData(accountID, folderID string, uid imap.UID, envelope *imap.Envelope, flags []imap.Flag, rfc822Size int64, rawBytes []byte) *message.Message {
	m := &message.Message{
		AccountID:  accountID,
		FolderID:   folderID,
		UID:        uint32(uid),
		ReceivedAt: time.Now().UTC(),
		Size:       int(rfc822Size),
	}

	// Parse envelope using shared helper
	applyEnvelopeToMessage(m, envelope)

	// Extract References and Disposition-Notification-To from raw message
	var references []string
	if len(rawBytes) > 0 {
		references = e.extractReferences(rawBytes)
		m.ReadReceiptTo = e.extractDispositionNotificationTo(rawBytes)
	}

	// Store references as JSON array
	if len(references) > 0 {
		refsJSON, _ := json.Marshal(references)
		m.References = string(refsJSON)
	}

	// Parse flags using shared helper
	applyFlagsToMessage(m, flags)

	// Parse message body
	if len(rawBytes) > 0 {
		bodyText, bodyHTML, hasAttachments := e.parseMessageBody(rawBytes)
		m.BodyText = bodyText
		m.HasAttachments = hasAttachments

		// Sanitize HTML
		if bodyHTML != "" {
			m.BodyHTML = e.sanitizer.Sanitize(bodyHTML)
		}

		// Generate snippet
		if bodyText != "" {
			m.Snippet = generateSnippet(bodyText, 200)
		} else if bodyHTML != "" {
			m.Snippet = generateSnippet(stripHTMLTags(bodyHTML), 200)
		}
	}

	return m
}
