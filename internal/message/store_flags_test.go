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

func TestUpdateFlagsBatchUpdatesOnlyRequestedFlags(t *testing.T) {
	s, accountID, folderID := newBodyFailedTestStore(t)
	for index, fixture := range []struct {
		id        string
		isRead    bool
		isStarred bool
	}{
		{id: "first", isRead: false, isStarred: true},
		{id: "second", isRead: true, isStarred: false},
		{id: "untouched", isRead: false, isStarred: false},
	} {
		if err := s.Create(&Message{
			ID:        fixture.id,
			AccountID: accountID,
			FolderID:  folderID,
			UID:       uint32(index + 1),
			Date:      time.Now().UTC(),
			IsRead:    fixture.isRead,
			IsStarred: fixture.isStarred,
		}); err != nil {
			t.Fatalf("Create(%s): %v", fixture.id, err)
		}
	}

	if err := s.UpdateFlagsBatch(nil, boolPointer(true), boolPointer(true)); err != nil {
		t.Fatalf("UpdateFlagsBatch(nil): %v", err)
	}
	if err := s.UpdateFlagsBatch([]string{"first"}, nil, nil); err != nil {
		t.Fatalf("UpdateFlagsBatch(no flags): %v", err)
	}

	if err := s.UpdateFlagsBatch([]string{"first", "second", "missing"}, boolPointer(false), nil); err != nil {
		t.Fatalf("UpdateFlagsBatch(read): %v", err)
	}
	assertMessageFlags(t, s, "first", false, true)
	assertMessageFlags(t, s, "second", false, false)

	if err := s.UpdateFlagsBatch([]string{"first", "second"}, nil, boolPointer(true)); err != nil {
		t.Fatalf("UpdateFlagsBatch(starred): %v", err)
	}
	assertMessageFlags(t, s, "first", false, true)
	assertMessageFlags(t, s, "second", false, true)
	assertMessageFlags(t, s, "untouched", false, false)

	if err := s.db.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if err := s.UpdateFlagsBatch([]string{"first"}, boolPointer(true), nil); err == nil {
		t.Fatal("UpdateFlagsBatch() error = nil after database close")
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func assertMessageFlags(t *testing.T, s *Store, id string, wantRead, wantStarred bool) {
	t.Helper()
	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get(%s): %v", id, err)
	}
	if got == nil {
		t.Fatalf("Get(%s) = nil", id)
	}
	if got.IsRead != wantRead || got.IsStarred != wantStarred {
		t.Fatalf("Get(%s) flags = read %t starred %t, want read %t starred %t", id, got.IsRead, got.IsStarred, wantRead, wantStarred)
	}
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
