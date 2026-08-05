package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/contact"
	"aulyc.local/aulycmail/internal/credentials"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	imapPkg "aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/settings"
)

func TestAccountLifecycleAndConnectionChecksAgainstMemoryIMAP(t *testing.T) {
	harness := startActionIMAPServer(t)
	db, err := database.Open(filepath.Join(t.TempDir(), "account-lifecycle.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("database.Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	credentialStore, err := credentials.NewStore(db.DB, t.TempDir())
	if err != nil {
		t.Fatalf("credentials.NewStore: %v", err)
	}
	pool := imapPkg.NewPool(imapPkg.DefaultPoolConfig(), func(string) (*imapPkg.ClientConfig, error) {
		config := harness.config()
		return &config, nil
	})
	t.Cleanup(pool.CloseAll)
	a := &App{
		ctx:           context.Background(),
		db:            db,
		accountStore:  accountStore,
		contactStore:  contact.NewStore(db.DB),
		folderStore:   folder.NewStore(db),
		settingsStore: settings.NewStore(db),
		credStore:     credentialStore,
		imapPool:      pool,
	}
	a.composeOps = composeOps{
		accountStore: accountStore,
		folderStore:  a.folderStore,
		contactStore: a.contactStore,
		credStore:    credentialStore,
	}

	config := account.AccountConfig{
		Name: "Memory account", DisplayName: "Memory User", Email: actionIMAPUsername,
		Username: actionIMAPUsername, Password: actionIMAPPassword,
		IMAPHost: harness.host, IMAPPort: harness.port, IMAPSecurity: account.SecurityNone,
		SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPSecurity: account.SecurityStartTLS,
		SMTPUsername: "smtp-user", SMTPPassword: "smtp-secret", AuthType: account.AuthPassword,
	}
	created, err := a.AddAccount(config)
	if err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	t.Cleanup(func() { _ = credentialStore.DeleteAllCredentials(created.ID) })
	if password, err := credentialStore.GetPassword(created.ID); err != nil || password != actionIMAPPassword {
		t.Fatalf("stored IMAP password = (%q, %v)", password, err)
	}
	if password, err := credentialStore.GetSMTPPassword(created.ID); err != nil || password != "smtp-secret" {
		t.Fatalf("stored SMTP password = (%q, %v)", password, err)
	}

	if result := a.TestConnection(config); !result.Success || result.Error != "" || result.CertificateRequired {
		t.Fatalf("TestConnection success = %#v", result)
	}
	wrongPassword := config
	wrongPassword.Password = "wrong"
	if result := a.TestConnection(wrongPassword); result.Success || !strings.Contains(result.Error, "failed to login") {
		t.Fatalf("TestConnection wrong password = %#v", result)
	}
	if result := a.TestAccountConnection(created.ID); !result.Success || result.Error != "" {
		t.Fatalf("TestAccountConnection success = %#v", result)
	}
	if timestamp, err := a.GetAccountConnOK(created.ID); err != nil || timestamp == "" {
		t.Fatalf("connection timestamp = (%q, %v)", timestamp, err)
	}
	if result := a.TestAccountConnection("missing-account"); result.Success || !strings.Contains(result.Error, "failed to load account credentials") {
		t.Fatalf("TestAccountConnection missing = %#v", result)
	}

	invalidMapping := config
	invalidMapping.SentFolderPath = "Missing Sent"
	if _, err := a.UpdateAccount(created.ID, invalidMapping); err == nil || !strings.Contains(err.Error(), "sent folder not found") {
		t.Fatalf("UpdateAccount missing mapping error = %v", err)
	}
	if _, err := a.UpdateAccount("missing-account", config); err == nil || !strings.Contains(err.Error(), "failed to get existing account") {
		t.Fatalf("UpdateAccount missing account error = %v", err)
	}
	sent := &folder.Folder{AccountID: created.ID, Name: "Sent", Path: "Sent", Type: folder.TypeSent}
	if err := a.folderStore.Create(sent); err != nil {
		t.Fatalf("create mapped sent folder: %v", err)
	}
	updatedConfig := config
	updatedConfig.SentFolderPath = sent.Path
	updatedConfig.Password = "updated-password"
	updatedConfig.SMTPUsername = ""
	updatedConfig.SMTPPassword = ""
	updated, err := a.UpdateAccount(created.ID, updatedConfig)
	if err != nil || updated.SentFolderPath != sent.Path {
		t.Fatalf("UpdateAccount = (%#v, %v)", updated, err)
	}
	if password, err := credentialStore.GetPassword(created.ID); err != nil || password != "updated-password" {
		t.Fatalf("updated IMAP password = (%q, %v)", password, err)
	}
	if _, err := credentialStore.GetSMTPPassword(created.ID); !errors.Is(err, credentials.ErrCredentialNotFound) {
		t.Fatalf("cleared SMTP password error = %v", err)
	}

	child, err := a.AddAccount(account.AccountConfig{
		Name: "Shared child", DisplayName: "Shared", Email: "shared@example.com",
		Username: "shared@example.com", IMAPHost: harness.host, IMAPPort: harness.port,
		IMAPSecurity: account.SecurityNone, NoOutgoingServer: true, AuthType: account.AuthPassword,
		SharedMailboxParentID: created.ID,
	})
	if err != nil {
		t.Fatalf("AddAccount shared child: %v", err)
	}
	if err := a.RemoveAccount(created.ID); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	if _, err := a.GetAccount(created.ID); !errors.Is(err, account.ErrAccountNotFound) {
		t.Fatalf("removed parent error = %v", err)
	}
	if _, err := a.GetAccount(child.ID); !errors.Is(err, account.ErrAccountNotFound) {
		t.Fatalf("removed shared child error = %v", err)
	}
}
