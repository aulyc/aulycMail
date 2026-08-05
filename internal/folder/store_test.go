package folder

import (
	"errors"
	"path/filepath"
	"testing"

	"aulyc.local/aulycmail/internal/database"
)

func TestRequireSelectable(t *testing.T) {
	selectable := &Folder{Path: "INBOX"}
	if err := selectable.RequireSelectable(); err != nil {
		t.Fatalf("selectable folder rejected: %v", err)
	}

	directory := &Folder{Path: "Other", NoSelect: true}
	if err := directory.RequireSelectable(); !errors.Is(err, ErrNotSelectable) {
		t.Fatalf("RequireSelectable error = %v, want ErrNotSelectable", err)
	}
}

func openTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// createTestAccount inserts a minimal account row so foreign key constraints are satisfied.
func createTestAccount(t *testing.T, db *database.DB, id string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO accounts (id, name, email, imap_host, imap_port, smtp_host, smtp_port, auth_type, username)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, "Test", id+"@test.com", "imap.test.com", 993, "smtp.test.com", 587, "password", id)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndGet(t *testing.T) {
	db := openTestDB(t)
	createTestAccount(t, db, "acc1")
	store := NewStore(db)

	f := &Folder{
		AccountID: "acc1",
		Name:      "Inbox",
		Path:      "INBOX",
		Type:      TypeInbox,
	}

	if err := store.Create(f); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}
	if f.ID == "" {
		t.Fatal("expected ID to be set after create")
	}

	got, err := store.Get(f.ID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got == nil {
		t.Fatal("expected folder, got nil")
	}
	if got.Name != "Inbox" {
		t.Errorf("name: got %q, want %q", got.Name, "Inbox")
	}
	if got.Path != "INBOX" {
		t.Errorf("path: got %q, want %q", got.Path, "INBOX")
	}
	if got.Type != TypeInbox {
		t.Errorf("type: got %q, want %q", got.Type, TypeInbox)
	}
	if got.AccountID != "acc1" {
		t.Errorf("accountID: got %q, want %q", got.AccountID, "acc1")
	}
}

func TestGetByPath(t *testing.T) {
	db := openTestDB(t)
	createTestAccount(t, db, "acc1")
	store := NewStore(db)

	f := &Folder{
		AccountID: "acc1",
		Name:      "Inbox",
		Path:      "INBOX",
		Type:      TypeInbox,
	}
	if err := store.Create(f); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	got, err := store.GetByPath("acc1", "INBOX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected folder, got nil")
	}
	if got.ID != f.ID {
		t.Errorf("id: got %q, want %q", got.ID, f.ID)
	}

	// Nonexistent path
	got, err = store.GetByPath("acc1", "NONEXISTENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent path")
	}
}

func TestList(t *testing.T) {
	db := openTestDB(t)
	createTestAccount(t, db, "acc1")
	store := NewStore(db)

	for _, name := range []string{"Inbox", "Sent"} {
		f := &Folder{
			AccountID: "acc1",
			Name:      name,
			Path:      name,
			Type:      TypeFolder,
		}
		if err := store.Create(f); err != nil {
			t.Fatalf("unexpected error creating %s: %v", name, err)
		}
	}

	folders, err := store.List("acc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(folders) != 2 {
		t.Errorf("got %d folders, want 2", len(folders))
	}
}

func TestNoSelectFolderRoundTripAndSelectableQueries(t *testing.T) {
	db := openTestDB(t)
	createTestAccount(t, db, "acc1")
	store := NewStore(db)

	directory := &Folder{
		AccountID:  "acc1",
		Name:       "Other",
		Path:       "Other",
		Type:       TypeFolder,
		Subscribed: true,
		NoSelect:   true,
	}
	selectable := &Folder{
		AccountID:  "acc1",
		Name:       "Child",
		Path:       "Other/Child",
		Type:       TypeFolder,
		Subscribed: true,
	}
	for _, f := range []*Folder{directory, selectable} {
		if err := store.Create(f); err != nil {
			t.Fatalf("Create(%s): %v", f.Path, err)
		}
	}

	got, err := store.Get(directory.ID)
	if err != nil {
		t.Fatalf("Get directory: %v", err)
	}
	if !got.NoSelect {
		t.Fatal("NoSelect was not preserved by the store")
	}

	selectableFolders, err := store.ListSelectable("acc1")
	if err != nil {
		t.Fatalf("ListSelectable: %v", err)
	}
	if len(selectableFolders) != 1 || selectableFolders[0].ID != selectable.ID {
		t.Fatalf("ListSelectable = %#v, want only %s", selectableFolders, selectable.ID)
	}

	subscribed, err := store.ListSubscribed("acc1")
	if err != nil {
		t.Fatalf("ListSubscribed: %v", err)
	}
	if len(subscribed) != 1 || subscribed[0].ID != selectable.ID {
		t.Fatalf("ListSubscribed = %#v, want only selectable folder", subscribed)
	}
}

func TestUpdate(t *testing.T) {
	db := openTestDB(t)
	createTestAccount(t, db, "acc1")
	store := NewStore(db)

	f := &Folder{
		AccountID: "acc1",
		Name:      "OldName",
		Path:      "INBOX",
		Type:      TypeInbox,
	}
	if err := store.Create(f); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	f.Name = "NewName"
	if err := store.Update(f); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	got, err := store.Get(f.ID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got.Name != "NewName" {
		t.Errorf("name: got %q, want %q", got.Name, "NewName")
	}
}

func TestUpdateSyncState(t *testing.T) {
	db := openTestDB(t)
	createTestAccount(t, db, "acc1")
	store := NewStore(db)

	f := &Folder{
		AccountID: "acc1",
		Name:      "Inbox",
		Path:      "INBOX",
		Type:      TypeInbox,
	}
	if err := store.Create(f); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	if err := store.UpdateSyncState(f.ID, 100, 200, 300, 50, 10); err != nil {
		t.Fatalf("unexpected error on update sync state: %v", err)
	}

	got, err := store.Get(f.ID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got.UIDValidity != 100 {
		t.Errorf("uidValidity: got %d, want 100", got.UIDValidity)
	}
	if got.UIDNext != 200 {
		t.Errorf("uidNext: got %d, want 200", got.UIDNext)
	}
	if got.HighestModSeq != 300 {
		t.Errorf("highestModSeq: got %d, want 300", got.HighestModSeq)
	}
	if got.TotalCount != 50 {
		t.Errorf("totalCount: got %d, want 50", got.TotalCount)
	}
	if got.UnreadCount != 10 {
		t.Errorf("unreadCount: got %d, want 10", got.UnreadCount)
	}
	if got.LastSync == nil {
		t.Error("expected lastSync to be set")
	}
}

func TestDelete(t *testing.T) {
	db := openTestDB(t)
	createTestAccount(t, db, "acc1")
	store := NewStore(db)

	f := &Folder{
		AccountID: "acc1",
		Name:      "Inbox",
		Path:      "INBOX",
		Type:      TypeInbox,
	}
	if err := store.Create(f); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	if err := store.Delete(f.ID); err != nil {
		t.Fatalf("unexpected error on delete: %v", err)
	}

	got, err := store.Get(f.ID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete, got folder")
	}
}

func TestUpsert(t *testing.T) {
	db := openTestDB(t)
	createTestAccount(t, db, "acc1")
	store := NewStore(db)

	f := &Folder{
		AccountID: "acc1",
		Name:      "Inbox",
		Path:      "INBOX",
		Type:      TypeInbox,
	}
	if err := store.Create(f); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Upsert with same account+path but different name
	f2 := &Folder{
		AccountID: "acc1",
		Name:      "Updated Inbox",
		Path:      "INBOX",
		Type:      TypeInbox,
	}
	if err := store.Upsert(f2); err != nil {
		t.Fatalf("unexpected error on upsert: %v", err)
	}

	// Should have reused the same ID
	if f2.ID != f.ID {
		t.Errorf("upsert should reuse ID: got %q, want %q", f2.ID, f.ID)
	}

	got, err := store.Get(f.ID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got.Name != "Updated Inbox" {
		t.Errorf("name: got %q, want %q", got.Name, "Updated Inbox")
	}

	// Verify only one folder exists
	folders, err := store.List("acc1")
	if err != nil {
		t.Fatalf("unexpected error on list: %v", err)
	}
	if len(folders) != 1 {
		t.Errorf("got %d folders, want 1", len(folders))
	}
}

func TestStoreCountSubscriptionAndAccountCleanup(t *testing.T) {
	db := openTestDB(t)
	createTestAccount(t, db, "acc1")
	createTestAccount(t, db, "acc2")
	store := NewStore(db)

	first := &Folder{AccountID: "acc1", Name: "Projects", Path: "Projects", Type: TypeFolder}
	second := &Folder{AccountID: "acc1", Name: "Receipts", Path: "Receipts", Type: TypeFolder}
	other := &Folder{AccountID: "acc2", Name: "INBOX", Path: "INBOX", Type: TypeInbox}
	for _, item := range []*Folder{first, second, other} {
		if err := store.Create(item); err != nil {
			t.Fatalf("Create(%s) error = %v", item.Path, err)
		}
	}

	if err := store.UpdateCounts(first.ID, 12, 5); err != nil {
		t.Fatalf("UpdateCounts() error = %v", err)
	}
	if err := store.UpdateSubscribed(first.ID, true); err != nil {
		t.Fatalf("UpdateSubscribed(true) error = %v", err)
	}
	got, err := store.Get(first.ID)
	if err != nil {
		t.Fatalf("Get(updated folder) error = %v", err)
	}
	if got.TotalCount != 12 || got.UnreadCount != 5 || !got.Subscribed {
		t.Fatalf("updated folder = %+v, want counts 12/5 and subscribed", got)
	}
	if err := store.UpdateSubscribed(first.ID, false); err != nil {
		t.Fatalf("UpdateSubscribed(false) error = %v", err)
	}
	got, err = store.Get(first.ID)
	if err != nil || got.Subscribed {
		t.Fatalf("folder after unsubscribe = %+v, %v", got, err)
	}

	if err := store.DeleteByAccount("acc1"); err != nil {
		t.Fatalf("DeleteByAccount() error = %v", err)
	}
	remaining, err := store.List("acc1")
	if err != nil || len(remaining) != 0 {
		t.Fatalf("acc1 folders after delete = %#v, %v", remaining, err)
	}
	otherAfter, err := store.Get(other.ID)
	if err != nil || otherAfter == nil {
		t.Fatalf("other account folder = %#v, %v; want preserved", otherAfter, err)
	}
}
