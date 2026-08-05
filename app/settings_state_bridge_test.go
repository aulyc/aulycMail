package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/appstate"
	"aulyc.local/aulycmail/internal/credentials"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/settings"
	"aulyc.local/aulycmail/internal/smtp"
)

func TestSettingsBridgeRoundTripsPreferencesAndDependencies(t *testing.T) {
	a, _, _ := newContactOwnEmailsTestApp(t)
	a.settingsStore = settings.NewStore(a.db)
	a.imageAllowlistStore = settings.NewImageAllowlistStore(a.db)

	if err := a.SetReadReceiptResponsePolicy(settings.PolicyAlways); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetReadReceiptResponsePolicy(); err != nil || got != settings.PolicyAlways {
		t.Fatalf("read receipt policy = %q, %v", got, err)
	}
	if delay, err := a.GetMarkAsReadDelay(); err != nil || delay != 1000 {
		t.Fatalf("fixed mark-read delay = %d, %v", delay, err)
	}
	if err := a.SetMarkAsReadDelay(500); err != nil {
		t.Fatal(err)
	}
	if err := a.SetMessageListDensity(settings.DensityCompact); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetMessageListDensity(); err != nil || got != settings.DensityCompact {
		t.Fatalf("density = %q, %v", got, err)
	}

	if err := a.SetAccentBarUnread(true); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetAccentBarUnread(); err != nil || !got {
		t.Fatalf("accent unread = %v, %v", got, err)
	}
	if err := a.SetDeveloperMode(true); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetDeveloperMode(); err != nil || !got {
		t.Fatalf("developer mode = %v, %v", got, err)
	}
	if err := a.SetEnhancedKeyboardNavigation(true); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetEnhancedKeyboardNavigation(); err != nil || !got {
		t.Fatalf("keyboard navigation = %v, %v", got, err)
	}
	if got, err := a.GetAutomaticUpdateChecks(); err != nil || !got {
		t.Fatalf("automatic update checks default = %v, %v", got, err)
	}
	if err := a.SetAutomaticUpdateChecks(false); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetAutomaticUpdateChecks(); err != nil || got {
		t.Fatalf("automatic update checks = %v, %v", got, err)
	}

	if err := a.SetMessageListSortOrder(settings.SortOrderOldest); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetMessageListSortOrder(); err != nil || got != settings.SortOrderOldest {
		t.Fatalf("sort order = %q, %v", got, err)
	}
	if err := a.SetThemeMode(settings.ThemeModeNordDark); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetThemeMode(); err != nil || got != settings.ThemeModeNordDark {
		t.Fatalf("theme = %q, %v", got, err)
	}
	if err := a.SetTermsAccepted(true); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetTermsAccepted(); err != nil || !got {
		t.Fatalf("terms accepted = %v, %v", got, err)
	}

	if err := a.SetStartHidden(true); err != nil {
		t.Fatal(err)
	}
	if hidden, err := a.GetStartHidden(); err != nil || !hidden {
		t.Fatalf("start hidden = %v, %v", hidden, err)
	}
	if background, err := a.GetRunBackground(); err != nil || !background {
		t.Fatalf("start hidden must enable background = %v, %v", background, err)
	}
	if err := a.SetMenuBarIcon(true); err != nil {
		t.Fatal(err)
	}
	if menuBar, err := a.GetMenuBarIcon(); err != nil || !menuBar {
		t.Fatalf("menu bar icon = %v, %v", menuBar, err)
	}
	if err := a.SetRunBackground(false); err != nil {
		t.Fatal(err)
	}
	if background, err := a.GetRunBackground(); err != nil || background {
		t.Fatalf("background after disable = %v, %v", background, err)
	}
	if hidden, _ := a.GetStartHidden(); hidden {
		t.Fatal("disabling background must disable start-hidden")
	}
	if menuBar, _ := a.GetMenuBarIcon(); menuBar {
		t.Fatal("disabling background must disable menu bar icon")
	}

	if err := a.SetAutostart(true); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetAutostart(); err != nil || !got {
		t.Fatalf("autostart = %v, %v", got, err)
	}
	if err := a.SetLanguage("zh-CN"); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetLanguage(); err != nil || got != "zh-CN" {
		t.Fatalf("language = %q, %v", got, err)
	}
	if err := a.SetComposerFormat(settings.ComposerFormatRich); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetComposerFormat(); err != nil || got != settings.ComposerFormatRich {
		t.Fatalf("composer format = %q, %v", got, err)
	}
	if err := a.SetNativeTitleBar(true); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetNativeTitleBar(); err != nil || !got {
		t.Fatalf("native title bar = %v, %v", got, err)
	}
	if err := a.SetAlwaysLoadImages(true); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetAlwaysLoadImages(); err != nil || !got {
		t.Fatalf("always load images = %v, %v", got, err)
	}
	if err := a.SetDarkMailContent(true); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetDarkMailContent(); err != nil || !got {
		t.Fatalf("dark mail content = %v, %v", got, err)
	}

	if err := a.AddImageAllowlist(settings.AllowlistTypeDomain, " Example.COM "); err != nil {
		t.Fatal(err)
	}
	if allowed, err := a.IsImageAllowed("news@example.com"); err != nil || !allowed {
		t.Fatalf("domain allowlist = %v, %v", allowed, err)
	}
	entries, err := a.GetImageAllowlist()
	if err != nil || len(entries) != 1 || entries[0].Value != "example.com" {
		t.Fatalf("allowlist entries = %#v, %v", entries, err)
	}
	if err := a.RemoveImageAllowlist(entries[0].ID); err != nil {
		t.Fatal(err)
	}
	if allowed, err := a.IsImageAllowed("news@example.com"); err != nil || allowed {
		t.Fatalf("removed allowlist = %v, %v", allowed, err)
	}

	if got := extractEmailFromHeader(" Name <person@example.com> "); got != "person@example.com" {
		t.Fatalf("bracketed email = %q", got)
	}
	if got := extractEmailFromHeader("plain@example.com"); got != "plain@example.com" {
		t.Fatalf("plain email = %q", got)
	}
}

func TestStateBridgePersistsUIAndConsumesPendingMailto(t *testing.T) {
	a, _, _ := newContactOwnEmailsTestApp(t)
	a.appStateStore = appstate.NewStore(a.db.DB)

	initial, err := a.GetUIState()
	if err != nil || initial.SidebarWidth != 240 || initial.ListWidth != 420 {
		t.Fatalf("default UI state = %#v, %v", initial, err)
	}
	want := &appstate.UIState{
		SelectedAccountID: "unified",
		SelectedFolderID:  "inbox",
		SidebarWidth:      300,
		ListWidth:         500,
		ExpandedAccounts:  map[string]bool{"account": true},
		CollapsedFolders:  map[string]bool{"folder": true},
		ActiveExtension:   "contacts",
	}
	if err := a.SaveUIState(want); err != nil {
		t.Fatal(err)
	}
	got, err := a.GetUIState()
	if err != nil || got.SidebarWidth != 300 || got.ActiveExtension != "contacts" || !got.ExpandedAccounts["account"] {
		t.Fatalf("saved UI state = %#v, %v", got, err)
	}

	info := a.GetAppInfo()
	if info.Name != "aulycMail" || info.Version != Version || info.BuildNumber != BuildNumber || info.DisplayVersion != VersionLabel() {
		t.Fatalf("app info = %#v", info)
	}
	pending := &MailtoData{To: []string{"receiver@example.com"}, Subject: "Hello"}
	a.PendingMailto = pending
	if got := a.GetPendingMailto(); got != pending {
		t.Fatalf("pending mailto = %#v", got)
	}
	if got := a.GetPendingMailto(); got != nil {
		t.Fatalf("pending mailto should be consumed, got %#v", got)
	}
}

func TestSendReadReceiptUsesDefaultIdentityAndHandlesSMTPFailures(t *testing.T) {
	a, created, _ := newContactOwnEmailsTestApp(t)
	a.messageStore = message.NewStore(a.db)
	a.folderStore = folder.NewStore(a.db)
	credentialStore, err := credentials.NewStore(a.db.DB, t.TempDir())
	if err != nil {
		t.Fatalf("credentials.NewStore: %v", err)
	}
	a.credStore = credentialStore
	t.Cleanup(func() { _ = credentialStore.DeleteAllCredentials(created.ID) })
	if err := credentialStore.SetPassword(created.ID, "synthetic-password"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	a.composeOps = composeOps{accountStore: a.accountStore, credStore: credentialStore}
	inbox := &folder.Folder{ID: "receipt-inbox", AccountID: created.ID, Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox}
	if err := a.folderStore.Create(inbox); err != nil {
		t.Fatalf("create inbox: %v", err)
	}
	createMessage := func(id, receipt string, handled bool) {
		t.Helper()
		if err := a.messageStore.Create(&message.Message{
			ID: id, AccountID: created.ID, FolderID: inbox.ID, UID: uint32(time.Now().UnixNano()),
			MessageID: id + "@example.com", Subject: "Receipt " + id,
			FromName: "Original Sender", FromEmail: "sender@example.com",
			ToList:        `[{"name":"Owner","email":"owner@example.com"}]`,
			ReadReceiptTo: receipt, ReadReceiptHandled: handled, Date: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("create receipt message %s: %v", id, err)
		}
	}

	createMessage("receipt-success", "Receipt User <receipt@example.com>", false)
	successClient := &fakeComposeSMTPClient{}
	a.composeOps.newSMTPClient = func(config smtp.ClientConfig) composeSMTPClient {
		if config.Host != "smtp.example.com" || config.Password != "synthetic-password" {
			t.Fatalf("SMTP config = %#v", config)
		}
		return successClient
	}
	if err := a.SendReadReceipt(created.ID, "receipt-success"); err != nil {
		t.Fatalf("SendReadReceipt: %v", err)
	}
	if !successClient.connected || !successClient.loggedIn || !successClient.closed || successClient.from != created.Email || len(successClient.recipients) != 1 || successClient.recipients[0] != "receipt@example.com" || !strings.Contains(string(successClient.rawMessage), "message/disposition-notification") {
		t.Fatalf("successful MDN client = %#v", successClient)
	}
	stored, err := a.messageStore.Get("receipt-success")
	if err != nil || stored == nil || !stored.ReadReceiptHandled {
		t.Fatalf("handled receipt message = (%#v, %v)", stored, err)
	}

	if err := a.SendReadReceipt(created.ID, "missing-message"); err == nil || !strings.Contains(err.Error(), "message not found") {
		t.Fatalf("missing receipt message error = %v", err)
	}
	createMessage("receipt-not-requested", "", false)
	if err := a.SendReadReceipt(created.ID, "receipt-not-requested"); err == nil || !strings.Contains(err.Error(), "does not request") {
		t.Fatalf("not-requested receipt error = %v", err)
	}
	createMessage("receipt-handled", "receipt@example.com", true)
	if err := a.SendReadReceipt(created.ID, "receipt-handled"); err == nil || !strings.Contains(err.Error(), "already handled") {
		t.Fatalf("already-handled receipt error = %v", err)
	}
	createMessage("receipt-missing-account", "receipt@example.com", false)
	if err := a.SendReadReceipt("missing-account", "receipt-missing-account"); err == nil || !strings.Contains(err.Error(), "failed to get account") {
		t.Fatalf("missing account receipt error = %v", err)
	}

	for index, failure := range []struct {
		name   string
		client *fakeComposeSMTPClient
		want   string
	}{
		{name: "connect", client: &fakeComposeSMTPClient{connectErr: errors.New("connect unavailable")}, want: "failed to connect"},
		{name: "login", client: &fakeComposeSMTPClient{loginErr: errors.New("login rejected")}, want: "failed to authenticate"},
		{name: "send", client: &fakeComposeSMTPClient{sendErr: errors.New("send rejected")}, want: "failed to send read receipt"},
	} {
		id := "receipt-" + failure.name
		createMessage(id, "receipt@example.com", false)
		a.composeOps.newSMTPClient = func(smtp.ClientConfig) composeSMTPClient { return failure.client }
		if err := a.SendReadReceipt(created.ID, id); err == nil || !strings.Contains(err.Error(), failure.want) {
			t.Fatalf("failure %d (%s) error = %v", index, failure.name, err)
		}
	}

	passwordless, err := a.accountStore.Create(&account.AccountConfig{
		Name: "Passwordless", DisplayName: "Passwordless", Email: "passwordless@example.com",
		Username: "passwordless@example.com", IMAPHost: "imap.example.com",
		SMTPHost: "smtp.example.com", AuthType: account.AuthPassword,
	})
	if err != nil {
		t.Fatalf("create passwordless account: %v", err)
	}
	createMessage("receipt-passwordless", "receipt@example.com", false)
	if err := a.SendReadReceipt(passwordless.ID, "receipt-passwordless"); err == nil || !strings.Contains(err.Error(), "failed to get password") {
		t.Fatalf("passwordless receipt error = %v", err)
	}
}
