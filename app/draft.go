package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aulyc/aulycmail/internal/account"
	"github.com/aulyc/aulycmail/internal/draft"
	"github.com/aulyc/aulycmail/internal/folder"
	"github.com/aulyc/aulycmail/internal/imap"
	"github.com/aulyc/aulycmail/internal/logging"
	"github.com/aulyc/aulycmail/internal/message"
	"github.com/aulyc/aulycmail/internal/smtp"
	goImap "github.com/emersion/go-imap/v2"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DraftResult represents the result of saving a draft
type DraftResult struct {
	Draft *draft.Draft `json:"draft"`
}

// draftBody holds the resolved body fields for draft storage
type draftBody struct {
	bodyHTML        string
	bodyText        string
	attachmentsData []byte
}

// syncStatusEmitter is a callback for emitting draft sync status changes to a Wails context
type syncStatusEmitter func(status draft.SyncStatus, imapUID uint32, syncError string)

// ============================================================================
// draftOps — shared draft logic
// ============================================================================

// draftOps contains shared draft operation logic for compose flows.
type draftOps struct {
	accountStore *account.Store
	folderStore  *folder.Store
	messageStore *message.Store
	draftStore   *draft.Store
	imapPool     *imap.Pool
}

// getSpecialFolder looks up a special folder for an account, checking manual
// mappings first and falling back to auto-detected folder type.
func (ops *draftOps) getSpecialFolder(accountID string, folderType folder.Type) (*folder.Folder, error) {
	acc, err := ops.accountStore.Get(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	if acc == nil {
		return nil, fmt.Errorf("account not found: %s", accountID)
	}

	mappedPath := acc.GetFolderMapping(string(folderType))
	if mappedPath != "" {
		f, err := ops.folderStore.GetByPath(accountID, mappedPath)
		if err != nil {
			return nil, err
		}
		if f != nil {
			return f, nil
		}
	}

	return ops.folderStore.GetByType(accountID, folderType)
}

// resolveAttachmentContent resolves ContentBase64 to Content for all attachments,
// normalizing the representation for storage and processing.
func resolveAttachmentContent(attachments []smtp.Attachment) ([]smtp.Attachment, error) {
	resolved := make([]smtp.Attachment, len(attachments))
	for i, att := range attachments {
		content, err := att.ResolveContent()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve content for %s: %w", att.Filename, err)
		}
		resolved[i] = att
		resolved[i].Content = content
		resolved[i].ContentBase64 = "" // Clear to avoid storing both
	}
	return resolved, nil
}

// prepareDraftBody resolves attachment content and serializes attachments for
// storage. Drafts are stored unencrypted.
func (ops *draftOps) prepareDraftBody(msg smtp.ComposeMessage) (*draftBody, error) {
	log := logging.WithComponent("draft")

	// Resolve ContentBase64 → Content for all attachments before processing
	if len(msg.Attachments) > 0 {
		resolved, err := resolveAttachmentContent(msg.Attachments)
		if err != nil {
			return nil, err
		}
		msg.Attachments = resolved
	}

	result := &draftBody{
		bodyHTML: msg.HTMLBody,
		bodyText: msg.TextBody,
	}

	if len(msg.Attachments) > 0 {
		attJSON, attErr := json.Marshal(msg.Attachments)
		if attErr != nil {
			log.Warn().Err(attErr).Msg("Failed to serialize draft attachments")
		} else {
			result.attachmentsData = attJSON
		}
	}

	return result, nil
}

// saveDraftToDB creates or updates a draft in the local database.
// If localDraft is non-nil, updates it; otherwise creates a new draft.
func (ops *draftOps) saveDraftToDB(accountID string, localDraft *draft.Draft, msg smtp.ComposeMessage, body *draftBody) (*draft.Draft, error) {
	log := logging.WithComponent("draft")

	if localDraft != nil {
		// Update existing draft
		localDraft.ToList = addressListToJSON(msg.To)
		localDraft.CcList = addressListToJSON(msg.Cc)
		localDraft.BccList = addressListToJSON(msg.Bcc)
		localDraft.Subject = msg.Subject
		localDraft.BodyHTML = body.bodyHTML
		localDraft.BodyText = body.bodyText
		localDraft.InReplyToID = msg.InReplyTo
		localDraft.AttachmentsData = body.attachmentsData
		localDraft.SyncStatus = draft.SyncStatusPending

		if err := ops.draftStore.Update(localDraft); err != nil {
			return nil, fmt.Errorf("failed to update draft: %w", err)
		}
		log.Debug().Str("draftID", localDraft.ID).Msg("Updated existing draft")
		return localDraft, nil
	}

	// Create new draft
	localDraft = &draft.Draft{
		AccountID:       accountID,
		ToList:          addressListToJSON(msg.To),
		CcList:          addressListToJSON(msg.Cc),
		BccList:         addressListToJSON(msg.Bcc),
		Subject:         msg.Subject,
		BodyHTML:        body.bodyHTML,
		BodyText:        body.bodyText,
		InReplyToID:     msg.InReplyTo,
		AttachmentsData: body.attachmentsData,
		SyncStatus:      draft.SyncStatusPending,
	}

	if err := ops.draftStore.Create(localDraft); err != nil {
		return nil, fmt.Errorf("failed to create draft: %w", err)
	}
	log.Debug().Str("draftID", localDraft.ID).Msg("Created new draft")
	return localDraft, nil
}

// deleteDraftCore deletes a draft from IMAP (if synced), cleans up the message
// row, and removes the draft from the local database. Returns the drafts folder
// (non-nil if the draft was synced) so callers can emit events and trigger syncs.
func (ops *draftOps) deleteDraftCore(ctx context.Context, d *draft.Draft) (*folder.Folder, error) {
	log := logging.WithComponent("draft")

	// Re-read from DB to get latest sync state (IMAPUID may have been updated
	// by a background sync goroutine since the caller obtained this draft object)
	if fresh, err := ops.draftStore.Get(d.ID); err == nil && fresh != nil {
		d = fresh
	}

	var draftsFolder *folder.Folder
	if d.IsSynced() {
		draftsFolder, _ = ops.getSpecialFolder(d.AccountID, folder.TypeDrafts)
		if draftsFolder != nil {
			poolConn, err := ops.imapPool.GetConnection(ctx, d.AccountID)
			if err == nil {
				defer ops.imapPool.Release(poolConn)
				conn := poolConn.Client()
				if _, err := conn.SelectMailbox(ctx, draftsFolder.Path); err == nil {
					if err := conn.DeleteMessageByUID(goImap.UID(d.IMAPUID)); err != nil {
						log.Warn().Err(err).Uint32("uid", d.IMAPUID).Msg("Failed to delete draft from IMAP")
					}
				}
			}

			// Clean up the message row that syncDraftToIMAP's SyncFolder may have created.
			// Done directly because the post-delete SyncFolder may be debounced (500ms)
			// if a recent draft sync just ran.
			if err := ops.messageStore.DeleteByUID(draftsFolder.ID, d.IMAPUID); err != nil {
				log.Warn().Err(err).Uint32("uid", d.IMAPUID).Str("folderID", draftsFolder.ID).Msg("Failed to clean up draft message row")
			}
		}
	}

	// Delete from local database
	if err := ops.draftStore.Delete(d.ID); err != nil {
		return draftsFolder, fmt.Errorf("failed to delete draft: %w", err)
	}

	return draftsFolder, nil
}

// syncToIMAP syncs a draft to the IMAP server. The emitStatus callback lets each
// caller emit events to its own Wails context. Returns the drafts folder on success
// (nil on failure) so callers can perform post-append work (SyncFolder or IPC notify).
func (ops *draftOps) syncToIMAP(ctx context.Context, localDraft *draft.Draft, msg smtp.ComposeMessage, emitStatus syncStatusEmitter) *folder.Folder {
	log := logging.WithComponent("draft")

	// Find the Drafts folder for this account
	draftsFolder, err := ops.getSpecialFolder(localDraft.AccountID, folder.TypeDrafts)
	if err != nil || draftsFolder == nil {
		log.Warn().Err(err).Str("account_id", localDraft.AccountID).Msg("No drafts folder found, skipping IMAP sync")
		_ = ops.draftStore.UpdateSyncStatus(localDraft.ID, draft.SyncStatusFailed, 0, "", "no drafts folder found")
		emitStatus(draft.SyncStatusFailed, 0, "no drafts folder found")
		return nil
	}

	// Get IMAP connection from pool
	poolConn, err := ops.imapPool.GetConnection(ctx, localDraft.AccountID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get IMAP connection, will retry later")
		_ = ops.draftStore.UpdateSyncStatus(localDraft.ID, draft.SyncStatusFailed, 0, "", err.Error())
		emitStatus(draft.SyncStatusFailed, 0, err.Error())
		return nil
	}
	defer ops.imapPool.Release(poolConn)

	conn := poolConn.Client()

	// Delete old IMAP draft if it exists
	if localDraft.IMAPUID > 0 && localDraft.FolderID != "" {
		if _, err := conn.SelectMailbox(ctx, draftsFolder.Path); err == nil {
			if err := conn.DeleteMessageByUID(goImap.UID(localDraft.IMAPUID)); err != nil {
				log.Warn().Err(err).Uint32("uid", localDraft.IMAPUID).Msg("Failed to delete old draft from IMAP")
			}
		}
	}

	// Build RFC822 message
	rawMsg, err := msg.ToRFC822()
	if err != nil {
		log.Error().Err(err).Msg("Failed to build RFC822 message")
		_ = ops.draftStore.UpdateSyncStatus(localDraft.ID, draft.SyncStatusFailed, 0, "", err.Error())
		emitStatus(draft.SyncStatusFailed, 0, err.Error())
		return nil
	}

	// Re-check if draft still exists (may have been deleted by concurrent DeleteDraft)
	if d, _ := ops.draftStore.Get(localDraft.ID); d == nil {
		log.Debug().Str("draftID", localDraft.ID).Msg("Draft deleted during sync, skipping IMAP append")
		return nil
	}

	// Check if cancelled before the irreversible IMAP append
	if ctx.Err() != nil {
		log.Debug().Str("draftID", localDraft.ID).Msg("Draft sync cancelled before IMAP append")
		return nil
	}

	// Append to IMAP Drafts folder with \Draft and \Seen flags
	flags := []goImap.Flag{goImap.FlagDraft, goImap.FlagSeen}
	uid, err := conn.AppendMessage(draftsFolder.Path, flags, time.Now(), rawMsg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to append draft to IMAP")
		_ = ops.draftStore.UpdateSyncStatus(localDraft.ID, draft.SyncStatusFailed, 0, "", err.Error())
		emitStatus(draft.SyncStatusFailed, 0, err.Error())
		return nil
	}

	// Post-APPEND guard (mirrors the pre-APPEND guards above): if the draft was
	// cancelled or deleted while we were appending, the just-appended message is
	// an orphan on the server. Clean it up here so deleteDraftCore doesn't need
	// to know about UIDs the local DB never recorded.
	if ctx.Err() != nil {
		log.Debug().Str("draftID", localDraft.ID).Uint32("uid", uint32(uid)).Msg("Draft cancelled during APPEND, cleaning up orphan")
		if _, selErr := conn.SelectMailbox(ctx, draftsFolder.Path); selErr == nil {
			if delErr := conn.DeleteMessageByUID(uid); delErr != nil {
				log.Warn().Err(delErr).Uint32("uid", uint32(uid)).Msg("Failed to clean up orphan after cancel-during-APPEND")
			}
		}
		return nil
	}
	if d, _ := ops.draftStore.Get(localDraft.ID); d == nil {
		log.Debug().Str("draftID", localDraft.ID).Uint32("uid", uint32(uid)).Msg("Draft deleted during APPEND, cleaning up orphan")
		if _, selErr := conn.SelectMailbox(ctx, draftsFolder.Path); selErr == nil {
			if delErr := conn.DeleteMessageByUID(uid); delErr != nil {
				log.Warn().Err(delErr).Uint32("uid", uint32(uid)).Msg("Failed to clean up orphan after delete-during-APPEND")
			}
		}
		return nil
	}

	// Update local draft with sync status
	if err := ops.draftStore.UpdateSyncStatus(localDraft.ID, draft.SyncStatusSynced, uint32(uid), draftsFolder.ID, ""); err != nil {
		log.Warn().Err(err).Msg("Failed to update draft sync status")
	}
	emitStatus(draft.SyncStatusSynced, uint32(uid), "")

	log.Info().
		Str("id", localDraft.ID).
		Uint32("imap_uid", uint32(uid)).
		Msg("Draft synced to IMAP")

	return draftsFolder
}

// toComposeMessage converts a draft to a ComposeMessage.
func (ops *draftOps) toComposeMessage(d *draft.Draft) *smtp.ComposeMessage {
	var attachments []smtp.Attachment

	if len(d.AttachmentsData) > 0 {
		if err := json.Unmarshal(d.AttachmentsData, &attachments); err != nil {
			log := logging.WithComponent("draft")
			log.Warn().Err(err).Str("draftID", d.ID).Msg("Failed to unmarshal draft attachments")
		}
	}

	return &smtp.ComposeMessage{
		To:          parseAddressList(d.ToList),
		Cc:          parseAddressList(d.CcList),
		Bcc:         parseAddressList(d.BccList),
		Subject:     d.Subject,
		HTMLBody:    d.BodyHTML,
		TextBody:    d.BodyText,
		Attachments: attachments,
		InReplyTo:   d.InReplyToID,
	}
}

// ============================================================================
// Draft API - Exposed to frontend via Wails bindings
// ============================================================================

// cancelDraftSync cancels any in-flight syncDraftToIMAP goroutine for the given draft
// and waits for it to finish. This prevents the race where DeleteDraft runs while
// a background goroutine is still uploading the draft to IMAP.
func (a *App) cancelDraftSync(draftID string) {
	a.syncMu.Lock()
	cancel, hasCancel := a.draftSyncContexts[draftID]
	a.syncMu.Unlock()

	if !hasCancel {
		return
	}
	// Signal the goroutine to abort and return immediately. The goroutine
	// self-cleans via the post-APPEND guard in syncToIMAP if it had already
	// committed an APPEND to the server.
	cancel()
}

// SaveDraft saves or updates a draft email to the local database and syncs to IMAP.
// If existingDraftID is provided and exists, updates that draft; otherwise creates a new one.
func (a *App) SaveDraft(accountID string, msg smtp.ComposeMessage, existingDraftID string) (*DraftResult, error) {
	log := logging.WithComponent("app")

	log.Debug().
		Str("accountID", accountID).
		Str("existingDraftID", existingDraftID).
		Str("subject", msg.Subject).
		Msg("SaveDraft called")

	var localDraft *draft.Draft

	// Try to load existing draft if ID provided
	if existingDraftID != "" {
		existing, err := a.draftStore.Get(existingDraftID)
		if err != nil {
			log.Warn().Err(err).Str("draftID", existingDraftID).Msg("Failed to load existing draft")
		}
		if err == nil && existing != nil {
			localDraft = existing
			log.Debug().Str("draftID", existingDraftID).Msg("Loaded existing draft for update")
		}
	}

	body, err := a.draftOps.prepareDraftBody(msg)
	if err != nil {
		return nil, err
	}

	localDraft, err = a.draftOps.saveDraftToDB(accountID, localDraft, msg, body)
	if err != nil {
		return nil, err
	}

	// Cancel any previous in-flight sync for this draft before starting a new one
	a.cancelDraftSync(localDraft.ID)

	// Sync to IMAP in background with cancellation support
	ctx, cancel := context.WithCancel(a.ctx)
	done := make(chan struct{})
	a.syncMu.Lock()
	a.draftSyncContexts[localDraft.ID] = cancel
	a.draftSyncDone[localDraft.ID] = done
	a.syncMu.Unlock()

	go func() {
		defer recoverPanic("app.draft", "sync draft to IMAP")
		defer close(done)
		defer func() {
			a.syncMu.Lock()
			if cur, exists := a.draftSyncDone[localDraft.ID]; exists && cur == done {
				delete(a.draftSyncContexts, localDraft.ID)
				delete(a.draftSyncDone, localDraft.ID)
			}
			a.syncMu.Unlock()
		}()
		a.syncDraftToIMAP(ctx, localDraft, msg)
	}()

	log.Info().Str("draftID", localDraft.ID).Msg("Draft saved locally, syncing to IMAP")
	return &DraftResult{Draft: localDraft}, nil
}

// syncDraftToIMAP syncs a draft to the IMAP server
func (a *App) syncDraftToIMAP(ctx context.Context, localDraft *draft.Draft, msg smtp.ComposeMessage) {
	log := logging.WithComponent("app")

	emitStatus := func(status draft.SyncStatus, imapUID uint32, syncError string) {
		wailsRuntime.EventsEmit(a.ctx, "draft:syncStatusChanged", map[string]interface{}{
			"draftId":    localDraft.ID,
			"syncStatus": status,
			"imapUid":    imapUID,
			"error":      syncError,
		})
	}

	draftsFolder := a.draftOps.syncToIMAP(ctx, localDraft, msg, emitStatus)
	if draftsFolder == nil {
		return
	}

	// Sync the Drafts folder so the main window's message list shows the updated draft
	// Do this after IMAP upload completes to ensure the draft is available
	if ctx.Err() != nil {
		log.Debug().Str("draftID", localDraft.ID).Msg("Draft sync cancelled, skipping folder sync")
		return
	}
	if err := a.SyncFolder(localDraft.AccountID, draftsFolder.ID); err != nil {
		log.Warn().Err(err).Str("folderID", draftsFolder.ID).Msg("Failed to sync Drafts folder after draft save")
		return
	}
	log.Debug().Str("folderID", draftsFolder.ID).Msg("Synced Drafts folder after draft save")
}

// SyncPendingDrafts syncs any pending drafts for an account
func (a *App) SyncPendingDrafts(accountID string) error {
	log := logging.WithComponent("app")

	pending, err := a.draftStore.ListPendingSync(accountID)
	if err != nil {
		return fmt.Errorf("failed to list pending drafts: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	log.Info().Int("count", len(pending)).Str("accountID", accountID).Msg("Syncing pending drafts")

	for _, d := range pending {
		msg := a.draftToComposeMessage(d)
		a.syncDraftToIMAP(a.ctx, d, *msg)
	}

	return nil
}

// syncAllPendingDrafts syncs pending drafts for all accounts
func (a *App) syncAllPendingDrafts() {
	defer recoverPanic("app.draft", "sync pending drafts")
	log := logging.WithComponent("app")

	accounts, err := a.accountStore.List()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list accounts for draft sync")
		return
	}

	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}
		if err := a.SyncPendingDrafts(acc.ID); err != nil {
			log.Warn().Err(err).Str("accountID", acc.ID).Msg("Failed to sync pending drafts")
		}
	}
}

// draftToComposeMessage converts a draft to a ComposeMessage.
func (a *App) draftToComposeMessage(d *draft.Draft) *smtp.ComposeMessage {
	return a.draftOps.toComposeMessage(d)
}

// DeleteDraft deletes a draft from local DB and IMAP
func (a *App) DeleteDraft(draftID string) error {
	log := logging.WithComponent("app")

	// Cancel any in-flight IMAP sync goroutine and wait for it to finish.
	// This ensures the goroutine can't upload the draft after we delete it.
	a.cancelDraftSync(draftID)

	// Get the draft to find IMAP UID (re-read after cancel to get latest state)
	d, err := a.draftStore.Get(draftID)
	if err != nil {
		return fmt.Errorf("failed to get draft: %w", err)
	}
	if d == nil {
		return nil // Already deleted
	}

	draftsFolder, err := a.draftOps.deleteDraftCore(a.ctx, d)
	if err != nil {
		return err
	}

	// Notify frontend to refresh the message list for this folder
	if draftsFolder != nil {
		wailsRuntime.EventsEmit(a.ctx, "messages:updated", map[string]interface{}{
			"accountId": d.AccountID,
			"folderId":  draftsFolder.ID,
		})
	}

	// Sync the Drafts folder so the message list and sidebar counts update
	if draftsFolder != nil {
		accountID := d.AccountID
		folderID := draftsFolder.ID
		go func() {
			defer recoverPanic("app.draft", "sync drafts folder after delete")
			if err := a.SyncFolder(accountID, folderID); err != nil {
				log.Warn().Err(err).Str("folderID", folderID).Msg("Failed to sync Drafts folder after draft delete")
			}
		}()
	}

	log.Info().Str("draftID", draftID).Msg("Draft deleted")
	return nil
}

// GetDraft returns a draft by ID as a ComposeMessage (for editing in composer)
// The ID can be either a draft ID or a message ID (from the Drafts folder)
func (a *App) GetDraft(id string) (*smtp.ComposeMessage, error) {
	log := logging.WithComponent("app")

	// First, try to get it as a draft ID
	d, err := a.draftStore.Get(id)
	if err != nil {
		return nil, err
	}
	if d != nil {
		log.Debug().Str("draftID", id).Msg("Found draft by draft ID")
		return a.draftToComposeMessage(d), nil
	}

	// Not found as draft ID - try as message ID
	// Get the message to find its IMAP UID and folder
	msg, err := a.messageStore.Get(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	if msg == nil {
		return nil, nil
	}

	// Look up draft by IMAP UID and folder
	d, err = a.draftStore.GetByIMAPUID(msg.FolderID, msg.UID)
	if err != nil {
		return nil, err
	}
	if d != nil {
		log.Debug().Str("messageID", id).Str("draftID", d.ID).Msg("Found draft by message IMAP UID")
		return a.draftToComposeMessage(d), nil
	}

	// No draft found - this might be a draft that was created outside aulycmail
	// (e.g., from webmail). Build a ComposeMessage from the message itself.
	log.Debug().Str("messageID", id).Msg("No local draft found, building from message")
	return a.messageToComposeMessage(msg), nil
}

// messageToComposeMessage converts a message (from Drafts folder) to a ComposeMessage
func (a *App) messageToComposeMessage(msg *message.Message) *smtp.ComposeMessage {
	return &smtp.ComposeMessage{
		To:        parseAddressList(msg.ToList),
		Cc:        parseAddressList(msg.CcList),
		Bcc:       parseAddressList(msg.BccList),
		Subject:   msg.Subject,
		HTMLBody:  msg.BodyHTML,
		TextBody:  msg.BodyText,
		InReplyTo: msg.InReplyTo,
	}
}
