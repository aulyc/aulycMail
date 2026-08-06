package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
)

func TestMessageActionValidationRejectsUnsafeFoldersWithoutMutation(t *testing.T) {
	fixture := newPublicActionFixture(t)
	inbox := fixture.folders[folder.TypeInbox]
	archive := fixture.folders[folder.TypeArchive]
	fixture.appendLocalMessage(t, folder.TypeInbox, "validation-message", "validation-message@example.com")

	noSelect := &folder.Folder{
		ID: "validation-group", AccountID: fixture.account.ID, Name: "Group", Path: "Group",
		Type: folder.TypeFolder, NoSelect: true,
	}
	if err := fixture.app.folderStore.Create(noSelect); err != nil {
		t.Fatalf("create no-select folder: %v", err)
	}
	noSelectMessage := &message.Message{
		ID: "validation-no-select", AccountID: fixture.account.ID, FolderID: noSelect.ID,
		UID: 1, Date: time.Now().UTC(), ReceivedAt: time.Now().UTC(),
	}
	if err := fixture.messages.Create(noSelectMessage); err != nil {
		t.Fatalf("create no-select message: %v", err)
	}

	for name, action := range map[string]func() error{
		"mark read":          func() error { return fixture.app.MarkAsRead([]string{noSelectMessage.ID}) },
		"mark unread":        func() error { return fixture.app.MarkAsUnread([]string{noSelectMessage.ID}) },
		"star":               func() error { return fixture.app.Star([]string{noSelectMessage.ID}) },
		"unstar":             func() error { return fixture.app.Unstar([]string{noSelectMessage.ID}) },
		"permanent delete":   func() error { return fixture.app.DeletePermanently([]string{noSelectMessage.ID}) },
		"remove Gmail label": func() error { return fixture.app.gmailRemoveLabel([]*message.Message{noSelectMessage}) },
	} {
		if err := action(); !errors.Is(err, folder.ErrNotSelectable) {
			t.Fatalf("%s no-select error = %v", name, err)
		}
	}

	if err := fixture.app.MoveToFolder([]string{"validation-message"}, "missing-destination"); err == nil || !strings.Contains(err.Error(), "destination folder not found") {
		t.Fatalf("MoveToFolder(missing destination) error = %v", err)
	}
	if err := fixture.app.CopyToFolder([]string{"validation-message"}, "missing-destination"); err == nil || !strings.Contains(err.Error(), "destination folder not found") {
		t.Fatalf("CopyToFolder(missing destination) error = %v", err)
	}
	if err := fixture.app.MoveToFolder([]string{"validation-message"}, noSelect.ID); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("MoveToFolder(no-select destination) error = %v", err)
	}
	if err := fixture.app.CopyToFolder([]string{"validation-message"}, noSelect.ID); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("CopyToFolder(no-select destination) error = %v", err)
	}

	for name, action := range map[string]func() error{
		"mark missing read": func() error { return fixture.app.MarkAsRead([]string{"missing"}) },
		"star missing":      func() error { return fixture.app.Star([]string{"missing"}) },
		"move missing":      func() error { return fixture.app.MoveToFolder([]string{"missing"}, archive.ID) },
		"copy missing":      func() error { return fixture.app.CopyToFolder([]string{"missing"}, archive.ID) },
		"delete missing":    func() error { return fixture.app.DeletePermanently([]string{"missing"}) },
	} {
		if err := action(); err != nil {
			t.Fatalf("%s should be a no-op, got %v", name, err)
		}
	}

	invalidUIDs := []*message.Message{
		{AccountID: fixture.account.ID, FolderID: inbox.ID, UID: 0},
		{AccountID: fixture.account.ID, FolderID: inbox.ID, UID: ^uint32(0)},
	}
	if err := fixture.app.removeFromIMAPFolder(invalidUIDs, inbox.ID); err != nil {
		t.Fatalf("removeFromIMAPFolder(temp UIDs) error = %v", err)
	}
	if err := fixture.app.deleteMessagesFromIMAP(invalidUIDs, inbox.ID); err != nil {
		t.Fatalf("deleteMessagesFromIMAP(temp UIDs) error = %v", err)
	}
	if err := fixture.app.moveMessagesToIMAP(nil, inbox.ID, archive); err != nil {
		t.Fatalf("moveMessagesToIMAP(empty) error = %v", err)
	}
	if err := fixture.app.copyMessagesToIMAP(nil, inbox.ID, archive); err != nil {
		t.Fatalf("copyMessagesToIMAP(empty) error = %v", err)
	}
}

func TestGmailLabelRemovalThroughPublicTrashAndSpamActions(t *testing.T) {
	for _, action := range []struct {
		name string
		run  func(*App, []string) (bool, error)
	}{
		{name: "trash", run: func(a *App, ids []string) (bool, error) { return a.Trash(ids) }},
		{name: "spam", run: func(a *App, ids []string) (bool, error) { return a.MarkAsSpam(ids) }},
	} {
		t.Run(action.name, func(t *testing.T) {
			fixture := newPublicActionFixture(t)
			if _, err := fixture.app.db.Exec(`UPDATE accounts SET imap_host = 'imap.gmail.com' WHERE id = ?`, fixture.account.ID); err != nil {
				t.Fatalf("mark fixture account as Gmail: %v", err)
			}
			messageID := "gmail-public-" + action.name + "@example.com"
			inboxID := "gmail-public-" + action.name + "-inbox"
			archiveID := "gmail-public-" + action.name + "-archive"
			fixture.appendLocalMessage(t, folder.TypeInbox, inboxID, messageID)
			fixture.appendLocalMessage(t, folder.TypeArchive, archiveID, messageID)

			moved, err := action.run(fixture.app, []string{inboxID})
			if err != nil || moved {
				t.Fatalf("Gmail %s copy action = (%v, %v), want label-only removal", action.name, moved, err)
			}
			if stored, err := fixture.messages.Get(inboxID); err != nil || stored != nil {
				t.Fatalf("Gmail %s local inbox copy = (%#v, %v), want deleted", action.name, stored, err)
			}
			if stored, err := fixture.messages.Get(archiveID); err != nil || stored == nil {
				t.Fatalf("Gmail %s archive copy = (%#v, %v), want preserved", action.name, stored, err)
			}
			waitForActionCondition(t, "Gmail "+action.name+" remote label removal", func() bool {
				return fixture.harness.status(t, "INBOX").Messages == 0 &&
					fixture.harness.status(t, "Archive").Messages == 1
			})
		})
	}
}

func TestMessageActionsSurfaceClosedDatabaseFailures(t *testing.T) {
	fixture := newPublicActionFixture(t)
	if err := fixture.app.db.Close(); err != nil {
		t.Fatalf("close fixture database: %v", err)
	}

	for name, action := range map[string]func() error{
		"mark all read":   func() error { return fixture.app.MarkAllFolderMessagesAsRead(fixture.folders[folder.TypeInbox].ID) },
		"mark all unread": func() error { return fixture.app.MarkAllFolderMessagesAsUnread(fixture.folders[folder.TypeInbox].ID) },
		"mark read":       func() error { return fixture.app.MarkAsRead([]string{"closed"}) },
		"star":            func() error { return fixture.app.Star([]string{"closed"}) },
		"move": func() error {
			return fixture.app.MoveToFolder([]string{"closed"}, fixture.folders[folder.TypeArchive].ID)
		},
		"copy": func() error {
			return fixture.app.CopyToFolder([]string{"closed"}, fixture.folders[folder.TypeArchive].ID)
		},
		"delete permanently": func() error { return fixture.app.DeletePermanently([]string{"closed"}) },
		"empty trash":        func() error { return fixture.app.EmptyTrash(fixture.account.ID, fixture.folders[folder.TypeTrash].ID) },
		"partition": func() error {
			_, err := fixture.app.partitionByAccount([]string{"closed"})
			return err
		},
	} {
		if err := action(); err == nil {
			t.Fatalf("%s unexpectedly succeeded with closed database", name)
		}
	}
	if moved, err := fixture.app.gmailTrashOrSpam([]string{"closed"}, fixture.folders[folder.TypeTrash]); err == nil || moved {
		t.Fatalf("gmailTrashOrSpam(closed DB) = (%v, %v)", moved, err)
	}
}
