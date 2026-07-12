package app

import (
	"path/filepath"
	"testing"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/contact"
	"aulyc.local/aulycmail/internal/database"
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
