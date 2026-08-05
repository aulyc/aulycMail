package app

import (
	"path/filepath"
	"testing"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
)

const (
	draftTestAccountID = "draft-account"
	draftTestFolderID  = "drafts-folder"
)

func newDraftSpecialFolderFixture(t *testing.T) (*database.DB, *draftOps) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		draftTestAccountID, "Draft Test", "draft@example.com", "imap.example.com", "smtp.example.com", "draft@example.com",
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type, uid_validity, selectable)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		draftTestFolderID, draftTestAccountID, "Drafts", "Drafts", "drafts", 9, 1,
	); err != nil {
		t.Fatalf("seed Drafts folder: %v", err)
	}
	return db, &draftOps{
		accountStore: account.NewStore(db),
		folderStore:  folder.NewStore(db),
	}
}

func TestDraftOpsSpecialFolderUsesMappingThenFallsBackToType(t *testing.T) {
	db, ops := newDraftSpecialFolderFixture(t)

	auto, err := ops.getSpecialFolder(draftTestAccountID, folder.TypeDrafts)
	if err != nil || auto == nil || auto.ID != draftTestFolderID {
		t.Fatalf("auto-detected Drafts folder = %#v, err=%v", auto, err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type, selectable)
		 VALUES ('custom-drafts', ?, 'Custom Drafts', 'Custom Drafts', 'folder', 1)`,
		draftTestAccountID,
	); err != nil {
		t.Fatalf("seed mapped folder: %v", err)
	}
	if _, err := db.Exec(`UPDATE accounts SET drafts_folder_path = 'Custom Drafts' WHERE id = ?`, draftTestAccountID); err != nil {
		t.Fatalf("set Drafts mapping: %v", err)
	}
	mapped, err := ops.getSpecialFolder(draftTestAccountID, folder.TypeDrafts)
	if err != nil || mapped == nil || mapped.ID != "custom-drafts" {
		t.Fatalf("mapped Drafts folder = %#v, err=%v", mapped, err)
	}
	if _, err := db.Exec(`UPDATE folders SET selectable = 0 WHERE id = 'custom-drafts'`); err != nil {
		t.Fatalf("mark mapped Drafts folder as non-selectable: %v", err)
	}
	fallback, err := ops.getSpecialFolder(draftTestAccountID, folder.TypeDrafts)
	if err != nil || fallback == nil || fallback.ID != draftTestFolderID {
		t.Fatalf("non-selectable mapping fallback = %#v, err=%v", fallback, err)
	}

	if _, err := db.Exec(`UPDATE accounts SET drafts_folder_path = 'Missing Drafts' WHERE id = ?`, draftTestAccountID); err != nil {
		t.Fatalf("set missing mapping: %v", err)
	}
	fallback, err = ops.getSpecialFolder(draftTestAccountID, folder.TypeDrafts)
	if err != nil || fallback == nil || fallback.ID != draftTestFolderID {
		t.Fatalf("fallback Drafts folder = %#v, err=%v", fallback, err)
	}
	if _, err := ops.getSpecialFolder("missing-account", folder.TypeDrafts); err == nil {
		t.Fatal("missing account must fail special-folder lookup")
	}
}
