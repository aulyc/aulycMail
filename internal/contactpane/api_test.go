package contactpane

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/contact"
	contactdto "aulyc.local/aulycmail/internal/contactdto"
	"aulyc.local/aulycmail/internal/database"
)

func openContactPaneTestAPI(t *testing.T) (*database.DB, *contact.Store, *API) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	store := contact.NewStore(db.DB)
	return db, store, NewAPI(store)
}

func TestSourceScopesAndConversionDefaults(t *testing.T) {
	for _, tc := range []struct {
		sourceID           string
		wantKind, wantRole string
		wantAccount        string
	}{
		{sourceID: SourceIDLocalManual, wantKind: "manual"},
		{sourceID: SourceIDLocalCollected, wantKind: "collected"},
		{sourceID: SourceIDRoleSender, wantRole: contact.RoleSender},
		{sourceID: SourceIDRoleRecipient, wantRole: contact.RoleRecipient},
		{sourceID: SourceIDRoleCc, wantRole: contact.RoleCc},
		{sourceID: SourceIDRoleBcc, wantRole: contact.RoleBcc},
		{sourceID: SourceIDRoleCcBcc, wantRole: contact.RoleCcBcc},
		{sourceID: "account:account-1:sender", wantRole: contact.RoleSender, wantAccount: "account-1"},
		{sourceID: "account:account-2:unknown", wantAccount: "account-2"},
		{sourceID: SourceIDLocal},
	} {
		t.Run(tc.sourceID, func(t *testing.T) {
			scope := browseScopeFromSourceID(tc.sourceID)
			if scope.kind != tc.wantKind || scope.role != tc.wantRole || scope.accountID != tc.wantAccount {
				t.Fatalf("scope = %#v, want kind=%q role=%q account=%q", scope, tc.wantKind, tc.wantRole, tc.wantAccount)
			}
		})
	}
	if roleFromKey("unknown") != "" || roleFromSourceID("unknown") != "" || localKindFromSourceID("unknown") != "" {
		t.Fatal("unknown scope keys must not create filters")
	}

	if got := fromRecord(nil); got.Emails == nil || len(got.Emails) != 0 {
		t.Fatalf("fromRecord(nil) emails = %#v, want non-nil empty slice", got.Emails)
	}
	if got := fromRecordSummary(nil); got.Emails == nil || len(got.Emails) != 0 {
		t.Fatalf("fromRecordSummary(nil) emails = %#v, want non-nil empty slice", got.Emails)
	}
	now := time.Now().UTC()
	local := fromLocal(&contact.Contact{Email: "local@example.com", DisplayName: "Local", Source: "aulycmail", CreatedAt: now})
	if local.ID != "local@example.com" || !local.UpdatedAt.Equal(now) {
		t.Fatalf("fromLocal created-at fallback = %#v", local)
	}
}

func TestAPIContactLifecycleFilteringAndRichPatch(t *testing.T) {
	db, store, api := openContactPaneTestAPI(t)
	if _, err := db.Exec(
		`INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		 VALUES ('account-1', 'Account One', 'owner@example.com', 'imap.example.com', 'smtp.example.com', 'owner@example.com')`,
	); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO folders (id, account_id, name, path, folder_type, selectable)
		 VALUES ('inbox-1', 'account-1', 'Inbox', 'INBOX', 'inbox', 1)`,
	); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO messages (id, account_id, folder_id, uid, subject, from_name, from_email, date)
		 VALUES ('message-1', 'account-1', 'inbox-1', 1, 'Hello', 'Rich Person', 'rich@example.com', '2026-07-15T00:00:00Z')`,
	); err != nil {
		t.Fatalf("seed associated message: %v", err)
	}

	minimalID, err := api.CreateContact(contactdto.ContactCreateInput{
		SourceID: SourceIDLocalManual,
		Email:    "  MINIMAL@Example.com ",
		Name:     " Minimal Person ",
	})
	if err != nil {
		t.Fatalf("CreateContact minimal: %v", err)
	}
	if minimalID != "minimal@example.com" {
		t.Fatalf("minimal ID = %q", minimalID)
	}
	if err := store.AddOrUpdateWithRole("sender@example.com", "Sender Person", contact.RoleSender); err != nil {
		t.Fatalf("seed collected sender: %v", err)
	}

	richID, err := api.CreateContact(contactdto.ContactCreateInput{
		SourceID:   SourceIDLocal,
		Email:      "rich@example.com",
		Name:       " Rich Person ",
		Nickname:   " Richy ",
		Org:        " Example Org ",
		Title:      " Engineer ",
		Note:       " Original note ",
		Bday:       " 2000-01-02 ",
		Categories: []string{"Friends", "Work"},
		Emails: []contactdto.ContactEmail{
			{Email: " RICH@example.com ", Type: "work", IsPrimary: true},
			{Email: "alias@example.com", Type: "home"},
		},
		Phones:    []contactdto.ContactPhone{{Number: " 12345 ", Type: "mobile", IsPrimary: true}},
		Addresses: []contactdto.ContactAddress{{Type: "work", Street: " 1 Main St ", City: " Shanghai ", Region: " SH ", Postcode: " 200000 ", Country: " CN "}},
		URLs:      []contactdto.ContactURL{{URL: " https://example.com ", Type: "work"}},
		IMPPs:     []contactdto.ContactIMPP{{Handle: " matrix:rich ", Type: "matrix"}},
		Photo:     &contactdto.ContactPhoto{Data: " aW1hZ2U= ", MediaType: " image/png "},
	})
	if err != nil {
		t.Fatalf("CreateContact rich: %v", err)
	}
	if richID == "" || richID == "rich@example.com" {
		t.Fatalf("rich contact ID = %q, want record UUID", richID)
	}

	detail, err := api.GetContact(richID)
	if err != nil {
		t.Fatalf("GetContact by ID: %v", err)
	}
	if detail == nil || detail.Name != "Rich Person" || detail.SourceID != "aulycmail" || len(detail.EmailItems) != 2 || len(detail.Phones) != 1 || len(detail.AssociatedAccounts) != 1 {
		t.Fatalf("unexpected rich detail: %#v", detail)
	}
	if detail.Org != "Example Org" || detail.PhotoData != "aW1hZ2U=" || detail.PhotoMediaType != "image/png" {
		t.Fatalf("rich scalar fields were not normalized: %#v", detail)
	}
	detailByEmail, err := api.GetContact("RICH@example.com")
	if err != nil || detailByEmail == nil || detailByEmail.ID != richID {
		t.Fatalf("GetContact by email = %#v, err=%v", detailByEmail, err)
	}

	search, err := api.SearchContacts("minimal", 10)
	if err != nil || len(search) != 1 || search[0].ID != "minimal@example.com" {
		t.Fatalf("SearchContacts = %#v, err=%v", search, err)
	}
	manual, err := api.ListContacts(contactdto.ContactFilter{SourceID: SourceIDLocalManual, Limit: 20})
	if err != nil || len(manual) != 2 {
		t.Fatalf("manual contacts = %#v, err=%v", manual, err)
	}
	collected, err := api.ListContacts(contactdto.ContactFilter{SourceID: SourceIDLocalCollected, Limit: 20})
	if err != nil || len(collected) != 1 || collected[0].Name != "Sender Person" {
		t.Fatalf("collected contacts = %#v, err=%v", collected, err)
	}
	senders, err := api.ListContacts(contactdto.ContactFilter{SourceID: SourceIDRoleSender, Limit: 20})
	if err != nil || len(senders) != 1 || senders[0].Name != "Sender Person" {
		t.Fatalf("sender contacts = %#v, err=%v", senders, err)
	}
	browse, err := api.BrowseContacts(contactdto.ContactFilter{SourceID: SourceIDLocal, SortOrder: contact.RecordSortNameDesc, Limit: 2})
	if err != nil || browse.Total != 3 || len(browse.Items) != 2 {
		t.Fatalf("BrowseContacts = %#v, err=%v", browse, err)
	}
	accountScoped, err := api.BrowseContacts(contactdto.ContactFilter{SourceID: "account:account-1:sender", Limit: 10})
	if err != nil || accountScoped.Total != 1 || len(accountScoped.Items) != 1 || accountScoped.Items[0].ID != richID {
		t.Fatalf("account-scoped contacts = %#v, err=%v", accountScoped, err)
	}
	groups, err := api.ListAccountGroups()
	if err != nil || len(groups) != 1 || groups[0].AccountID != "account-1" || groups[0].SenderCount != 1 {
		t.Fatalf("ListAccountGroups = %#v, err=%v", groups, err)
	}

	name := " Updated Person "
	nickname := " Up "
	org := " Updated Org "
	title := " Lead "
	note := " Updated note "
	bday := " 1999-12-31 "
	emails := []contactdto.ContactEmail{{Email: " UPDATED@example.com ", Type: "work", IsPrimary: true}}
	phones := []contactdto.ContactPhone{{Number: " 67890 ", Type: "work"}}
	addresses := []contactdto.ContactAddress{{Type: "home", Street: " 2 Side St ", City: " Beijing "}}
	urls := []contactdto.ContactURL{{URL: " https://updated.example.com ", Type: "home"}}
	impps := []contactdto.ContactIMPP{{Handle: " signal:updated ", Type: "signal"}}
	categories := []string{"Updated"}
	if err := api.UpdateContact(richID, contactdto.ContactPatch{
		Name: &name, Nickname: &nickname, Org: &org, Title: &title, Note: &note, Bday: &bday,
		Emails: &emails, Phones: &phones, Addresses: &addresses, URLs: &urls, IMPPs: &impps,
		Categories: &categories, Photo: &contactdto.ContactPhoto{},
	}); err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}
	updated, err := api.GetContact("updated@example.com")
	if err != nil || updated == nil {
		t.Fatalf("GetContact after update = %#v, err=%v", updated, err)
	}
	if updated.Name != "Updated Person" || updated.Org != "Updated Org" || updated.PhotoData != "" || len(updated.Emails) != 1 || updated.Emails[0] != "updated@example.com" {
		t.Fatalf("unexpected updated detail: %#v", updated)
	}
	if err := api.UpdateContact(richID, contactdto.ContactPatch{}); err != nil {
		t.Fatalf("empty patch must be a no-op: %v", err)
	}
	if err := api.UpdateContact("missing", contactdto.ContactPatch{Name: &name}); err != nil {
		t.Fatalf("missing update must be idempotent: %v", err)
	}
	if err := api.DeleteContact("updated@example.com"); err != nil {
		t.Fatalf("DeleteContact by email: %v", err)
	}
	if got, err := api.GetContact(richID); err != nil || got != nil {
		t.Fatalf("deleted contact = %#v, err=%v", got, err)
	}
	if err := api.DeleteContact("missing"); err != nil {
		t.Fatalf("missing delete must be idempotent: %v", err)
	}
}

func TestAPIValidationAndUnavailableStore(t *testing.T) {
	api := NewAPI(nil)
	if got, err := api.SearchContacts("x", 10); err != nil || got != nil {
		t.Fatalf("nil-store SearchContacts = %#v, err=%v", got, err)
	}
	if got, err := api.GetContact("x@example.com"); err != nil || got != nil {
		t.Fatalf("nil-store GetContact = %#v, err=%v", got, err)
	}
	if got, err := api.ListContacts(contactdto.ContactFilter{}); err != nil || got != nil {
		t.Fatalf("nil-store ListContacts = %#v, err=%v", got, err)
	}
	if got, err := api.BrowseContacts(contactdto.ContactFilter{}); err != nil || got.Total != 0 || got.Items != nil {
		t.Fatalf("nil-store BrowseContacts = %#v, err=%v", got, err)
	}
	if got, err := api.ListAccountGroups(); err != nil || got != nil {
		t.Fatalf("nil-store ListAccountGroups = %#v, err=%v", got, err)
	}
	for _, input := range []contactdto.ContactCreateInput{
		{},
		{Email: "not-an-email"},
		{Email: "valid@example.com", SourceID: SourceIDLocalCollected},
		{Email: "valid@example.com", SourceID: "remote"},
		{Email: "valid@example.com", SourceID: SourceIDLocal},
	} {
		if _, err := api.CreateContact(input); err == nil {
			t.Fatalf("CreateContact(%#v) error = nil", input)
		}
	}
	if err := api.UpdateContact("", contactdto.ContactPatch{}); err == nil {
		t.Fatal("UpdateContact empty ID error = nil")
	}
	if err := api.UpdateContact("id", contactdto.ContactPatch{}); err == nil {
		t.Fatal("UpdateContact nil store error = nil")
	}
	if err := api.DeleteContact(""); err == nil {
		t.Fatal("DeleteContact empty ID error = nil")
	}
	if err := api.DeleteContact("id"); err == nil {
		t.Fatal("DeleteContact nil store error = nil")
	}
}

func TestContactsBridgeDelegatesAndEmitsConflicts(t *testing.T) {
	db, _, _ := openContactPaneTestAPI(t)
	missing := NewContactsBridge(ContactsBridgeDeps{})
	if _, err := missing.Contacts_BrowseContacts("", "", "", 10, 0); err == nil {
		t.Fatal("bridge without DB must fail initialization")
	}

	var eventName string
	var eventPayload any
	bridge := NewContactsBridge(ContactsBridgeDeps{
		DB: db,
		Emitter: func(name string, payload any) {
			eventName = name
			eventPayload = payload
		},
	})
	id, err := bridge.Contacts_CreateContact(contactdto.ContactCreateInput{Email: "bridge@example.com", Name: "Bridge"})
	if err != nil || id != "bridge@example.com" {
		t.Fatalf("Contacts_CreateContact ID=%q err=%v", id, err)
	}
	listed, err := bridge.Contacts_ListContactsForBrowse("bridge", SourceIDLocalManual, 10, 0)
	if err != nil || len(listed) != 1 {
		t.Fatalf("Contacts_ListContactsForBrowse = %#v, err=%v", listed, err)
	}
	browsed, err := bridge.Contacts_BrowseContacts("", SourceIDLocal, contact.RecordSortNameAsc, 10, 0)
	if err != nil || browsed.Total != 1 {
		t.Fatalf("Contacts_BrowseContacts = %#v, err=%v", browsed, err)
	}
	detail, err := bridge.Contacts_GetContactDetail(id)
	if err != nil || detail == nil || detail.Name != "Bridge" {
		t.Fatalf("Contacts_GetContactDetail = %#v, err=%v", detail, err)
	}
	name := "Updated Bridge"
	if err := bridge.Contacts_UpdateContact(id, contactdto.ContactPatch{Name: &name}); err != nil {
		t.Fatalf("Contacts_UpdateContact: %v", err)
	}
	if _, err := bridge.Contacts_GetContactAccountGroups(); err != nil {
		t.Fatalf("Contacts_GetContactAccountGroups: %v", err)
	}
	if err := bridge.Contacts_DeleteLocalContact(id); err != nil {
		t.Fatalf("Contacts_DeleteLocalContact: %v", err)
	}

	conflict := &contactdto.ErrConflict{ContactID: "conflicted", Message: "newer edit won"}
	if !bridge.emitConflict(conflict) {
		t.Fatal("ErrConflict was not recognized")
	}
	if eventName != "contacts:conflict" {
		t.Fatalf("conflict event name = %q", eventName)
	}
	payload, ok := eventPayload.(map[string]string)
	if !ok || payload["contactId"] != "conflicted" || payload["message"] != "newer edit won" {
		t.Fatalf("conflict event payload = %#v", eventPayload)
	}
	if bridge.emitConflict(errors.New("ordinary failure")) {
		t.Fatal("ordinary error was misclassified as a conflict")
	}
	withoutEmitter := NewContactsBridge(ContactsBridgeDeps{DB: db})
	if !withoutEmitter.emitConflict(conflict) {
		t.Fatal("conflict without emitter must still be recognized")
	}
}
