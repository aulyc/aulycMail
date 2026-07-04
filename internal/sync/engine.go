// Package sync provides IMAP synchronization functionality
package sync

import (
	"context"
	"io"

	"github.com/aulyc/aulycmail/internal/account"
	"github.com/aulyc/aulycmail/internal/contact"
	"github.com/aulyc/aulycmail/internal/email"
	"github.com/aulyc/aulycmail/internal/folder"
	imapPkg "github.com/aulyc/aulycmail/internal/imap"
	"github.com/aulyc/aulycmail/internal/logging"
	"github.com/aulyc/aulycmail/internal/message"
	gomessage "github.com/emersion/go-message"
	"github.com/rs/zerolog"
)

func init() {
	// Don't let go-message decode charset - return raw bytes and we'll decode ourselves
	// This gives us full control over charset detection and handling of mislabeled encodings
	gomessage.CharsetReader = func(charsetName string, r io.Reader) (io.Reader, error) {
		// Just return the original reader - we'll handle charset conversion in decodeCharset()
		return r, nil
	}
}

// Batch sizes for incremental sync
const (
	headerBatchSize = 50 // Messages per batch for header fetch
)

// Body fetch batch limits (hybrid byte + count based, like Geary)
const (
	bodyBatchMaxBytes    = 512 * 1024 // 512KB max per batch (memory safety)
	bodyBatchMaxMessages = 50         // Never more than 50 messages per batch
	bodyBatchMinMessages = 1          // At least 1 message per batch (for oversized emails)
	bodyBatchQueryLimit  = 200        // Query more candidates to allow byte-based batching
)

// Size limits for MIME parsing and inline content storage.
const (
	maxPartSize          = 10 * 1024 * 1024 // 10MB max for a single MIME part
	maxInlineContentSize = 5 * 1024 * 1024  // 5MB max for inline image content (stored in DB)
)

// ParsedBody holds the result of parsing a message body, including attachments
type ParsedBody struct {
	BodyText       string
	BodyHTML       string
	HasAttachments bool
	Attachments    []*message.Attachment // Extracted attachment metadata (content only for inline)
	UnsafeContent  bool                  // True if message has non-compliant encoding
}

// Retry limits for error recovery
const (
	maxMessageRetries    = 3 // Max retries per message before giving up
	maxConnectionRetries = 3 // Max connection recovery attempts before aborting
)

// SyncProgress holds progress information for sync operations
type SyncProgress struct {
	AccountID string `json:"accountId"`
	FolderID  string `json:"folderId"`
	Fetched   int    `json:"fetched"`
	Total     int    `json:"total"`
	Phase     string `json:"phase"` // "headers" or "bodies"
}

// ProgressCallback is called with sync progress updates
type ProgressCallback func(progress SyncProgress)

// Engine handles synchronization between IMAP server and local storage
type Engine struct {
	pool             *imapPkg.Pool
	accountStore     *account.Store
	folderStore      *folder.Store
	messageStore     *message.Store
	attachmentStore  *message.AttachmentStore
	contactStore     *contact.Store
	attachExtractor  *email.AttachmentExtractor
	sanitizer        *email.Sanitizer
	log              zerolog.Logger
	progressCallback ProgressCallback
}

// NewEngine creates a new sync engine
func NewEngine(pool *imapPkg.Pool, accountStore *account.Store, folderStore *folder.Store, messageStore *message.Store, attachmentStore *message.AttachmentStore, contactStore *contact.Store) *Engine {
	return &Engine{
		pool:            pool,
		accountStore:    accountStore,
		folderStore:     folderStore,
		messageStore:    messageStore,
		attachmentStore: attachmentStore,
		contactStore:    contactStore,
		attachExtractor: email.NewAttachmentExtractor(),
		sanitizer:       email.NewSanitizer(),
		log:             logging.WithComponent("sync"),
	}
}

// GetPoolConnection acquires a connection from the IMAP connection pool.
// Caller must release with ReleasePoolConnection when done.
func (e *Engine) GetPoolConnection(ctx context.Context, accountID string) (*imapPkg.PooledConnection, error) {
	return e.pool.GetConnection(ctx, accountID)
}

// ReleasePoolConnection returns a connection to the pool.
func (e *Engine) ReleasePoolConnection(conn *imapPkg.PooledConnection) {
	e.pool.Release(conn)
}

// SetProgressCallback sets the callback function for progress updates
func (e *Engine) SetProgressCallback(callback ProgressCallback) {
	e.progressCallback = callback
}

// emitProgress sends progress updates if a callback is set
func (e *Engine) emitProgress(accountID, folderID string, fetched, total int, phase string) {
	if e.progressCallback != nil {
		e.progressCallback(SyncProgress{
			AccountID: accountID,
			FolderID:  folderID,
			Fetched:   fetched,
			Total:     total,
			Phase:     phase,
		})
	}
}
