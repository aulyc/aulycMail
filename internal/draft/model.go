// Package draft provides local draft management with IMAP sync
package draft

import (
	"time"
)

// SyncStatus represents the sync state of a draft
type SyncStatus string

const (
	// SyncStatusPending indicates draft needs to be synced to IMAP
	SyncStatusPending SyncStatus = "pending"
	// SyncStatusSynced indicates draft is synced with IMAP
	SyncStatusSynced SyncStatus = "synced"
	// SyncStatusFailed indicates sync failed (will retry)
	SyncStatusFailed SyncStatus = "failed"
)

// Draft represents a locally stored draft email
type Draft struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`

	// Composer state (stored as JSON strings for lists)
	ToList   string `json:"toList"`
	CcList   string `json:"ccList"`
	BccList  string `json:"bccList"`
	Subject  string `json:"subject"`
	BodyHTML string `json:"bodyHtml"`
	BodyText string `json:"bodyText"`

	// Reply/forward context
	InReplyToID    string `json:"inReplyToId,omitempty"`
	ReplyType      string `json:"replyType,omitempty"` // "reply", "reply-all", "forward"
	ReferencesList string `json:"referencesList,omitempty"`

	// Identity
	IdentityID string `json:"identityId,omitempty"`

	// Attachment data (JSON-serialized)
	AttachmentsData []byte `json:"-"` // Not sent to frontend directly

	// IMAP sync state
	SyncStatus      SyncStatus `json:"syncStatus"`
	IMAPUID         uint32     `json:"imapUid,omitempty"`
	FolderID        string     `json:"folderId,omitempty"`
	LastSyncAttempt *time.Time `json:"lastSyncAttempt,omitempty"`
	SyncError       string     `json:"syncError,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// IsSynced returns true if the draft is synced with IMAP
func (d *Draft) IsSynced() bool {
	return d.SyncStatus == SyncStatusSynced && d.IMAPUID > 0
}

// NeedsSync returns true if the draft should be synced to IMAP
func (d *Draft) NeedsSync() bool {
	return d.SyncStatus == SyncStatusPending || d.SyncStatus == SyncStatusFailed
}
