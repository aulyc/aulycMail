package app

import (
	"errors"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/activitylog"
	"aulyc.local/aulycmail/internal/certificate"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/settings"
)

func TestCertificateBridgeSupportsSessionAndPermanentTrust(t *testing.T) {
	a, _, _ := newContactOwnEmailsTestApp(t)
	a.certStore = certificate.NewStore(a.db.DB)

	session := certificate.CertificateInfo{Fingerprint: " session-fingerprint "}
	if err := a.AcceptCertificate(" IMAP.EXAMPLE.COM ", session, false); err != nil {
		t.Fatalf("AcceptCertificate(session) error = %v", err)
	}
	if !a.certStore.IsTrusted("imap.example.com", "session-fingerprint") {
		t.Fatal("session certificate was not trusted")
	}

	permanent := certificate.CertificateInfo{
		Fingerprint: "permanent-fingerprint",
		Subject:     "mail.example.com",
		Issuer:      "Synthetic Test CA",
	}
	if err := a.AcceptCertificate("mail.example.com", permanent, true); err != nil {
		t.Fatalf("AcceptCertificate(permanent) error = %v", err)
	}
	trusted, err := a.GetTrustedCertificates([]string{"MAIL.EXAMPLE.COM"})
	if err != nil || len(trusted) != 1 || trusted[0].Fingerprint != permanent.Fingerprint {
		t.Fatalf("GetTrustedCertificates() = %#v, %v", trusted, err)
	}
	if err := a.RemoveTrustedCertificate(permanent.Fingerprint); err != nil {
		t.Fatalf("RemoveTrustedCertificate() error = %v", err)
	}
	trusted, err = a.GetTrustedCertificates([]string{"mail.example.com"})
	if err != nil || len(trusted) != 0 {
		t.Fatalf("trusted certificates after removal = %#v, %v", trusted, err)
	}
}

func TestAccountBridgeListsReordersAndGroupsIdentities(t *testing.T) {
	a, first, _ := newContactOwnEmailsTestApp(t)
	a.settingsStore = settings.NewStore(a.db)

	secondConfig := account.AccountConfig{
		Name:        "Second account",
		DisplayName: "Second Owner",
		Email:       "second@example.com",
		IMAPHost:    "imap.example.com",
		SMTPHost:    "smtp.example.com",
		Username:    "second@example.com",
	}
	second, err := a.AddAccount(secondConfig)
	if err != nil {
		t.Fatal(err)
	}
	alias, err := a.CreateIdentity(first.ID, account.IdentityConfig{Email: "alias@example.com", Name: "Alias"})
	if err != nil {
		t.Fatal(err)
	}

	accounts, err := a.GetAccounts()
	if err != nil || len(accounts) != 2 {
		t.Fatalf("GetAccounts = %#v, %v", accounts, err)
	}
	got, err := a.GetAccount(second.ID)
	if err != nil || got.Email != secondConfig.Email {
		t.Fatalf("GetAccount = %#v, %v", got, err)
	}
	if _, err := a.GetAccount("missing"); !errors.Is(err, account.ErrAccountNotFound) {
		t.Fatalf("missing account error = %v", err)
	}

	if err := a.ReorderAccounts([]string{second.ID, first.ID}); err != nil {
		t.Fatal(err)
	}
	accounts, err = a.GetAccounts()
	if err != nil || accounts[0].ID != second.ID || accounts[1].ID != first.ID {
		t.Fatalf("reordered accounts = %#v, %v", accounts, err)
	}

	identities, err := a.GetIdentities(first.ID)
	if err != nil || len(identities) != 2 {
		t.Fatalf("GetIdentities = %#v, %v", identities, err)
	}
	if err := a.SetDefaultIdentity(first.ID, alias.ID); err != nil {
		t.Fatal(err)
	}
	identities, err = a.GetIdentities(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	defaultID := ""
	for _, identity := range identities {
		if identity.IsDefault {
			defaultID = identity.ID
		}
	}
	if defaultID != alias.ID {
		t.Fatalf("default identity = %q, want %q", defaultID, alias.ID)
	}

	groups, err := a.GetAllAccountIdentities()
	if err != nil || len(groups) != 2 {
		t.Fatalf("GetAllAccountIdentities = %#v, %v", groups, err)
	}
	if _, err := a.db.Exec("UPDATE accounts SET enabled = 0 WHERE id = ?", second.ID); err != nil {
		t.Fatal(err)
	}
	groups, err = a.GetAllAccountIdentities()
	if err != nil || len(groups) != 1 || groups[0].Account.ID != first.ID {
		t.Fatalf("enabled identity groups = %#v, %v", groups, err)
	}

	if result := a.TestConnection(account.AccountConfig{}); result.Success || result.Error == "" || result.CertificateRequired {
		t.Fatalf("invalid connection test = %#v", result)
	}

	if timestamp, err := a.GetAccountConnOK(first.ID); err != nil || timestamp != "" {
		t.Fatalf("initial connection timestamp = %q, %v", timestamp, err)
	}
	a.recordAccountConnOK(first.ID)
	timestamp, err := a.GetAccountConnOK(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		t.Fatalf("connection timestamp = %q: %v", timestamp, err)
	}
	withoutSettings := &App{}
	withoutSettings.recordAccountConnOK(first.ID)
	if timestamp, err := withoutSettings.GetAccountConnOK(first.ID); err != nil || timestamp != "" {
		t.Fatalf("nil settings result = %q, %v", timestamp, err)
	}
}

func TestFolderBridgeSortingTreeDetectionAndMappings(t *testing.T) {
	a, created, _ := newContactOwnEmailsTestApp(t)
	a.folderStore = folder.NewStore(a.db)

	parent := &folder.Folder{AccountID: created.ID, Name: "Projects", Path: "Projects", Type: folder.TypeFolder, NoSelect: true}
	if err := a.folderStore.Create(parent); err != nil {
		t.Fatal(err)
	}
	child := &folder.Folder{AccountID: created.ID, Name: "Alpha", Path: "Projects/Alpha", Type: folder.TypeFolder, ParentID: parent.ID}
	inbox := &folder.Folder{AccountID: created.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox}
	sent := &folder.Folder{AccountID: created.ID, Name: "Sent", Path: "Sent", Type: folder.TypeSent}
	for _, item := range []*folder.Folder{child, sent, inbox} {
		if err := a.folderStore.Create(item); err != nil {
			t.Fatal(err)
		}
	}

	folders, err := a.GetFolders(created.ID)
	if err != nil || len(folders) != 3 {
		t.Fatalf("GetFolders = %#v, %v", folders, err)
	}
	if folders[0].ID != inbox.ID || folders[1].ID != sent.ID || folders[2].ID != child.ID {
		t.Fatalf("folder sort order = %#v", folders)
	}
	mappingFolders, err := a.GetAccountFoldersForMapping(created.ID)
	if err != nil || len(mappingFolders) != 3 {
		t.Fatalf("mapping folders = %#v, %v", mappingFolders, err)
	}

	tree, err := a.GetFolderTree(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	var parentNode *folder.FolderTree
	for _, node := range tree {
		if node.Folder.ID == parent.ID {
			parentNode = node
		}
	}
	if parentNode == nil || len(parentNode.Children) != 1 || parentNode.Children[0].Folder.ID != child.ID {
		t.Fatalf("folder tree = %#v", tree)
	}

	detected, err := a.GetAutoDetectedFolders(created.ID)
	if err != nil || detected[string(folder.TypeSent)] != sent.Path {
		t.Fatalf("auto-detected folders = %#v, %v", detected, err)
	}
	if _, ok := detected[string(folder.TypeInbox)]; ok {
		t.Fatalf("inbox must not be returned as a configurable special mapping: %#v", detected)
	}

	special, err := a.getSpecialFolder(created.ID, folder.TypeSent)
	if err != nil || special == nil || special.ID != sent.ID {
		t.Fatalf("auto special folder = %#v, %v", special, err)
	}
	if _, err := a.db.Exec("UPDATE accounts SET sent_folder_path = ? WHERE id = ?", child.Path, created.ID); err != nil {
		t.Fatal(err)
	}
	special, err = a.getSpecialFolder(created.ID, folder.TypeSent)
	if err != nil || special == nil || special.ID != child.ID {
		t.Fatalf("mapped special folder = %#v, %v", special, err)
	}
	if _, err := a.db.Exec("UPDATE accounts SET sent_folder_path = ? WHERE id = ?", parent.Path, created.ID); err != nil {
		t.Fatal(err)
	}
	special, err = a.getSpecialFolder(created.ID, folder.TypeSent)
	if err != nil || special == nil || special.ID != sent.ID {
		t.Fatalf("non-selectable mapping fallback = %#v, %v", special, err)
	}
	if _, err := a.getSpecialFolder("missing", folder.TypeSent); err == nil {
		t.Fatal("missing account must fail special-folder lookup")
	}

	if selected, err := a.requireSelectableFolder(child.ID); err != nil || selected.ID != child.ID {
		t.Fatalf("selectable folder = %#v, %v", selected, err)
	}
	if _, err := a.requireSelectableFolder(parent.ID); !errors.Is(err, folder.ErrNotSelectable) {
		t.Fatalf("non-selectable folder error = %v", err)
	}
	if _, err := a.requireSelectableFolder("missing"); err == nil {
		t.Fatal("missing folder must fail selection")
	}
}

func TestActivityLogBridgeAvailabilityQueriesAndClear(t *testing.T) {
	unavailable := &App{}
	if _, err := unavailable.ListActivityLogs(activitylog.Query{}); !errors.Is(err, errActivityLogStoreUnavailable) {
		t.Fatalf("ListActivityLogs unavailable error = %v", err)
	}
	if _, err := unavailable.GetLatestActivityLog(activitylog.TypeSync, ""); !errors.Is(err, errActivityLogStoreUnavailable) {
		t.Fatalf("GetLatestActivityLog unavailable error = %v", err)
	}
	if _, err := unavailable.ClearActivityLogs(activitylog.Query{}); !errors.Is(err, errActivityLogStoreUnavailable) {
		t.Fatalf("ClearActivityLogs unavailable error = %v", err)
	}

	a, _, _ := newContactOwnEmailsTestApp(t)
	a.activityLogStore = activitylog.NewStore(a.db)
	for _, entry := range []activitylog.Entry{
		{Type: activitylog.TypeSync, Status: activitylog.StatusSuccess, Title: "Sync", Summary: "Complete"},
		{Type: activitylog.TypeBackup, Status: activitylog.StatusFailed, Title: "Backup", Summary: "Failed", Payload: map[string]any{"directory": "/backup"}},
	} {
		entry := entry
		if err := a.activityLogStore.Append(&entry); err != nil {
			t.Fatal(err)
		}
	}

	page, err := a.ListActivityLogs(activitylog.Query{ProblemOnly: true})
	if err != nil || page.Total != 1 || len(page.Entries) != 1 || page.Entries[0].Type != activitylog.TypeBackup {
		t.Fatalf("problem activity page = %#v, %v", page, err)
	}
	latest, err := a.GetLatestActivityLog(activitylog.TypeBackup, "/backup")
	if err != nil || latest == nil || latest.Status != activitylog.StatusFailed {
		t.Fatalf("latest activity = %#v, %v", latest, err)
	}
	cleared, err := a.ClearActivityLogs(activitylog.Query{Type: activitylog.TypeBackup})
	if err != nil || cleared != 1 {
		t.Fatalf("cleared = %d, %v", cleared, err)
	}
	page, err = a.ListActivityLogs(activitylog.Query{})
	if err != nil || page.Total != 1 || page.Entries[0].Type != activitylog.TypeSync {
		t.Fatalf("remaining activities = %#v, %v", page, err)
	}
}
