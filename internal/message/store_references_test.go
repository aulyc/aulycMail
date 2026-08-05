package message

import (
	"testing"
	"time"
)

func TestMessageGettersPreserveReferencesChain(t *testing.T) {
	store, accountID, folderID := newBodyFailedTestStore(t)
	wantReferences := `["<root@example.com>","<parent@example.com>"]`
	created := &Message{
		ID:         "message-with-references",
		AccountID:  accountID,
		FolderID:   folderID,
		UID:        77,
		MessageID:  "child@example.com",
		InReplyTo:  "parent@example.com",
		References: wantReferences,
		Subject:    "Threaded message",
		Date:       time.Now().UTC(),
	}
	if err := store.Create(created); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byID, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if byID == nil || byID.References != wantReferences {
		t.Fatalf("Get references = %#v, want %q", byID, wantReferences)
	}

	byUID, err := store.GetByUID(folderID, created.UID)
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if byUID == nil || byUID.References != wantReferences {
		t.Fatalf("GetByUID references = %#v, want %q", byUID, wantReferences)
	}
}
