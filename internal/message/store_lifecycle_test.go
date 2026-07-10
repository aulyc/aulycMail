package message

import (
	"testing"
	"time"
)

func TestUIDsWithCopiesInFolderTypes(t *testing.T) {
	s, accountID, inboxID := newBodyFailedTestStore(t)
	if _, err := s.db.Exec(`
		INSERT INTO folders (id, account_id, name, path, folder_type)
		VALUES
			('trash-folder', ?, 'Trash', '[Gmail]/Trash', 'trash'),
			('spam-folder', ?, 'Spam', '[Gmail]/Spam', 'spam')
	`, accountID, accountID); err != nil {
		t.Fatalf("seed folders: %v", err)
	}

	now := time.Now().UTC()
	messages := []*Message{
		{ID: "inbox-hidden", AccountID: accountID, FolderID: inboxID, UID: 10, MessageID: "same-hidden", Date: now},
		{ID: "trash-copy", AccountID: accountID, FolderID: "trash-folder", UID: 20, MessageID: "same-hidden", Date: now},
		{ID: "inbox-visible", AccountID: accountID, FolderID: inboxID, UID: 11, MessageID: "visible-only", Date: now},
		{ID: "trash-self", AccountID: accountID, FolderID: "trash-folder", UID: 21, MessageID: "trash-only", Date: now},
	}
	if err := s.UpsertBatch(messages); err != nil {
		t.Fatalf("seed messages: %v", err)
	}

	hidden, err := s.UIDsWithCopiesInFolderTypes(inboxID, accountID, []uint32{10, 11}, []string{"trash", "spam"})
	if err != nil {
		t.Fatalf("UIDsWithCopiesInFolderTypes inbox: %v", err)
	}
	if !hidden[10] {
		t.Fatal("expected inbox UID 10 to be hidden by Trash copy")
	}
	if hidden[11] {
		t.Fatal("did not expect inbox UID 11 without Trash/Spam copy to be hidden")
	}

	trashHidden, err := s.UIDsWithCopiesInFolderTypes("trash-folder", accountID, []uint32{21}, []string{"trash", "spam"})
	if err != nil {
		t.Fatalf("UIDsWithCopiesInFolderTypes trash: %v", err)
	}
	if trashHidden[21] {
		t.Fatal("did not expect the source Trash row itself to count as a hidden copy")
	}
}
