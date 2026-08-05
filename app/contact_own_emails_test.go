package app

import (
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/contact"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
)

func newContactOwnEmailsTestApp(t *testing.T) (*App, *account.Account, *account.AccountConfig) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	accountStore := account.NewStore(db)
	contactStore := contact.NewStore(db.DB)
	cfg := &account.AccountConfig{
		Name:         "Owner account",
		DisplayName:  "Owner",
		Email:        "owner@example.com",
		IMAPHost:     "imap.example.com",
		SMTPHost:     "smtp.example.com",
		SMTPUsername: "smtp-owner",
		Username:     "owner@example.com",
	}
	created, err := accountStore.Create(cfg)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	a := &App{
		db:           db,
		accountStore: accountStore,
		contactStore: contactStore,
	}
	a.initSyncBridge()
	a.initDraftBridge()
	a.updateDBConnectionPool()
	return a, created, cfg
}

func assertContactMissing(t *testing.T, store *contact.Store, email string) {
	t.Helper()
	got, err := store.Get(email)
	if err != nil {
		t.Fatalf("Get(%q): %v", email, err)
	}
	if got != nil {
		t.Fatalf("Get(%q) = %+v, want nil", email, got)
	}
}

func assertContactPresent(t *testing.T, store *contact.Store, email string) {
	t.Helper()
	got, err := store.Get(email)
	if err != nil {
		t.Fatalf("Get(%q): %v", email, err)
	}
	if got == nil {
		t.Fatalf("Get(%q) = nil, want contact", email)
	}
}

func TestIdentityCRUDImmediatelyRefreshesOwnEmailExclusions(t *testing.T) {
	a, created, _ := newContactOwnEmailsTestApp(t)
	if err := a.contactStore.AddOrUpdate("alias@example.com", "Future alias"); err != nil {
		t.Fatalf("seed future alias contact: %v", err)
	}
	assertContactPresent(t, a.contactStore, "alias@example.com")
	alias, err := a.CreateIdentity(created.ID, account.IdentityConfig{
		Email: "alias@example.com",
		Name:  "Alias",
	})
	if err != nil {
		t.Fatalf("CreateIdentity(): %v", err)
	}
	assertContactMissing(t, a.contactStore, "alias@example.com")

	if err := a.contactStore.AddOrUpdate("new-alias@example.com", "Future new alias"); err != nil {
		t.Fatalf("seed future updated alias contact: %v", err)
	}
	if _, err := a.UpdateIdentity(alias.ID, account.IdentityConfig{
		Email: "new-alias@example.com",
		Name:  "New Alias",
	}); err != nil {
		t.Fatalf("UpdateIdentity(): %v", err)
	}
	if err := a.contactStore.AddOrUpdate("alias@example.com", "Former alias"); err != nil {
		t.Fatalf("collect former alias: %v", err)
	}
	if err := a.contactStore.AddOrUpdate("new-alias@example.com", "Current alias"); err != nil {
		t.Fatalf("collect current alias: %v", err)
	}
	assertContactPresent(t, a.contactStore, "alias@example.com")
	assertContactMissing(t, a.contactStore, "new-alias@example.com")

	if err := a.DeleteIdentity(alias.ID); err != nil {
		t.Fatalf("DeleteIdentity(): %v", err)
	}
	if err := a.contactStore.AddOrUpdate("new-alias@example.com", "Former alias"); err != nil {
		t.Fatalf("collect deleted alias: %v", err)
	}
	assertContactPresent(t, a.contactStore, "new-alias@example.com")
}

func TestUpdateAccountImmediatelyRefreshesOwnEmailExclusions(t *testing.T) {
	a, created, cfg := newContactOwnEmailsTestApp(t)
	if err := a.contactStore.AddOrUpdate("new-owner@example.com", "Future owner"); err != nil {
		t.Fatalf("seed future account email contact: %v", err)
	}
	cfg.Email = "new-owner@example.com"
	cfg.Username = cfg.Email
	if _, err := a.UpdateAccount(created.ID, *cfg); err != nil {
		t.Fatalf("UpdateAccount(): %v", err)
	}

	assertContactMissing(t, a.contactStore, "new-owner@example.com")
}

func TestRefreshContactsFromMailPurgesAccountAndAliasHistory(t *testing.T) {
	a, created, _ := newContactOwnEmailsTestApp(t)
	if _, err := a.accountStore.CreateIdentity(created.ID, &account.IdentityConfig{
		Email: "alias@example.com",
		Name:  "Alias",
	}); err != nil {
		t.Fatalf("CreateIdentity fixture: %v", err)
	}

	// Seed historical rows before refreshing the own-address set.
	a.contactStore.SetOwnEmails(nil)
	for _, email := range []string{"owner@example.com", "external@example.com"} {
		if err := a.contactStore.AddOrUpdate(email, email); err != nil {
			t.Fatalf("seed contact %q: %v", email, err)
		}
	}
	if err := a.contactStore.Create("alias@example.com", "Own alias manual record"); err != nil {
		t.Fatalf("seed manual alias contact: %v", err)
	}

	if _, err := a.RefreshContactsFromMail(); err != nil {
		t.Fatalf("RefreshContactsFromMail(): %v", err)
	}
	assertContactMissing(t, a.contactStore, "owner@example.com")
	assertContactMissing(t, a.contactStore, "alias@example.com")
	assertContactPresent(t, a.contactStore, "external@example.com")
	records, err := a.contactStore.ListRecords(contact.RecordFilter{Source: "local"})
	if err != nil {
		t.Fatalf("ListRecords(): %v", err)
	}
	if len(records) != 1 || len(records[0].Emails) != 1 || records[0].Emails[0].Email != "external@example.com" {
		t.Fatalf("contacts after own-address purge = %+v, want only external@example.com", records)
	}
}

func TestRefreshContactOwnEmailsKeepsLastKnownSetOnQueryError(t *testing.T) {
	a, _, _ := newContactOwnEmailsTestApp(t)
	if _, err := a.db.Exec(`DROP TABLE identities`); err != nil {
		t.Fatalf("drop identities fixture: %v", err)
	}
	if _, err := a.refreshContactOwnEmails(); err == nil {
		t.Fatal("refreshContactOwnEmails() error = nil, want query error")
	}

	// The failed refresh must not replace the previous set. The already-known
	// account address therefore remains protected from auto-collection.
	if err := a.contactStore.AddOrUpdate("OWNER@example.com", "Owner"); err != nil {
		t.Fatalf("collect protected owner after refresh error: %v", err)
	}
	assertContactMissing(t, a.contactStore, "owner@example.com")
}

func TestRefreshContactsCollectsObservableRolesAndRelatedMessages(t *testing.T) {
	a, created, _ := newContactOwnEmailsTestApp(t)
	a.folderStore = folder.NewStore(a.db)
	a.messageStore = message.NewStore(a.db)
	folders := map[folder.Type]*folder.Folder{}
	for _, folderType := range []folder.Type{folder.TypeInbox, folder.TypeSent, folder.TypeDrafts, folder.TypeSpam, folder.TypeTrash} {
		item := &folder.Folder{
			ID: "contacts-" + string(folderType), AccountID: created.ID,
			Name: string(folderType), Path: string(folderType), Type: folderType,
		}
		if err := a.folderStore.Create(item); err != nil {
			t.Fatalf("create %s folder: %v", folderType, err)
		}
		folders[folderType] = item
	}

	now := time.Now().UTC()
	messages := []*message.Message{
		{
			ID: "received-contact", AccountID: created.ID, FolderID: folders[folder.TypeInbox].ID, UID: 1,
			MessageID: "received-contact@example.com", Subject: "Received",
			FromName: "Sender", FromEmail: "sender@example.com", Date: now,
		},
		{
			ID: "sent-contact", AccountID: created.ID, FolderID: folders[folder.TypeSent].ID, UID: 2,
			MessageID: "sent-contact@example.com", Subject: "Sent", FromEmail: created.Email, Date: now,
			ToList:  `[{"name":"Recipient","email":"recipient@example.com"}]`,
			CcList:  `[{"name":"Copy","email":"copy@example.com"}]`,
			BccList: `[{"name":"Blind","email":"blind@example.com"},{"name":"Empty","email":""}]`,
		},
		{
			ID: "sent-malformed", AccountID: created.ID, FolderID: folders[folder.TypeSent].ID, UID: 3,
			MessageID: "sent-malformed@example.com", Subject: "Malformed", FromEmail: created.Email,
			ToList: `{broken`, Date: now,
		},
	}
	for index, folderType := range []folder.Type{folder.TypeDrafts, folder.TypeSpam, folder.TypeTrash} {
		messages = append(messages, &message.Message{
			ID: "skipped-contact-" + string(folderType), AccountID: created.ID,
			FolderID: folders[folderType].ID, UID: uint32(index + 10),
			MessageID: "skipped-" + string(folderType) + "@example.com",
			Subject:   "Skipped", FromEmail: "skip@example.com", Date: now,
		})
	}
	if err := a.messageStore.UpsertBatch(messages); err != nil {
		t.Fatalf("seed contact messages: %v", err)
	}

	scanned, err := a.RefreshContactsFromMail()
	if err != nil || scanned != len(messages) {
		t.Fatalf("RefreshContactsFromMail = (%d, %v), want %d", scanned, err, len(messages))
	}
	for _, email := range []string{"sender@example.com", "recipient@example.com", "copy@example.com", "blind@example.com"} {
		assertContactPresent(t, a.contactStore, email)
	}
	assertContactMissing(t, a.contactStore, "skip@example.com")

	results, err := a.SearchContacts("example.com", 2)
	if err != nil || len(results) != 2 {
		t.Fatalf("SearchContacts = (%#v, %v)", results, err)
	}
	related, err := a.GetContactMessages("sender@example.com", 10)
	if err != nil || len(related) != 1 || related[0].ID != "received-contact" {
		t.Fatalf("GetContactMessages = (%#v, %v)", related, err)
	}
	if empty, err := (&App{}).GetContactMessages("sender@example.com", 10); err != nil || len(empty) != 0 {
		t.Fatalf("unavailable GetContactMessages = (%#v, %v)", empty, err)
	}
	if _, err := (&App{}).RefreshContactsFromMail(); err == nil {
		t.Fatal("RefreshContactsFromMail without stores unexpectedly succeeded")
	}
}
