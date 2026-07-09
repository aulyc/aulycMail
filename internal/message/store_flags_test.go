package message

import (
	"testing"
	"time"
)

func TestUpdateFlagsSkipsNoopWrites(t *testing.T) {
	s, accountID, folderID := newBodyFailedTestStore(t)
	msg := &Message{
		ID:        "msg-1",
		AccountID: accountID,
		FolderID:  folderID,
		UID:       42,
		Date:      time.Now().UTC(),
		IsRead:    true,
	}
	if err := s.Create(msg); err != nil {
		t.Fatalf("Create message: %v", err)
	}

	if _, err := s.db.Exec(`
		CREATE TABLE update_probe (count INTEGER NOT NULL DEFAULT 0);
		INSERT INTO update_probe (count) VALUES (0);
		CREATE TRIGGER probe_messages_update AFTER UPDATE ON messages
		BEGIN
			UPDATE update_probe SET count = count + 1;
		END;
	`); err != nil {
		t.Fatalf("create probe trigger: %v", err)
	}

	if err := s.UpdateFlagsByUIDBatch(folderID, []FlagUpdate{{
		UID:    42,
		IsRead: true,
	}}); err != nil {
		t.Fatalf("UpdateFlagsByUIDBatch noop: %v", err)
	}
	assertProbeCount(t, s, 0)

	if err := s.UpdateFlagsByUID(folderID, 42, false, false, false, false, false, false); err != nil {
		t.Fatalf("UpdateFlagsByUID changed: %v", err)
	}
	assertProbeCount(t, s, 1)

	if err := s.UpdateFlags("msg-1", false, false, false, false, false, false); err != nil {
		t.Fatalf("UpdateFlags noop: %v", err)
	}
	assertProbeCount(t, s, 1)
}

func assertProbeCount(t *testing.T, s *Store, want int) {
	t.Helper()
	var got int
	if err := s.db.QueryRow(`SELECT count FROM update_probe`).Scan(&got); err != nil {
		t.Fatalf("read probe count: %v", err)
	}
	if got != want {
		t.Fatalf("update probe count = %d, want %d", got, want)
	}
}
