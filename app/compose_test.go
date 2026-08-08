package app

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/contact"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/smtp"
	goImap "github.com/emersion/go-imap/v2"
)

type fakeComposeCredentials struct {
	password          string
	smtpPassword      string
	passwordErr       error
	smtpPasswordErr   error
	passwordCalls     int
	smtpPasswordCalls int
}

func (f *fakeComposeCredentials) GetPassword(string) (string, error) {
	f.passwordCalls++
	return f.password, f.passwordErr
}

func (f *fakeComposeCredentials) GetSMTPPassword(string) (string, error) {
	f.smtpPasswordCalls++
	return f.smtpPassword, f.smtpPasswordErr
}

type fakeComposeSMTPClient struct {
	connectErr error
	loginErr   error
	sendErr    error

	connected  bool
	loggedIn   bool
	closed     bool
	from       string
	recipients []string
	rawMessage []byte
}

func (f *fakeComposeSMTPClient) Connect() error {
	f.connected = true
	return f.connectErr
}

func (f *fakeComposeSMTPClient) Login() error {
	f.loggedIn = true
	return f.loginErr
}

func (f *fakeComposeSMTPClient) SendMail(from string, to []string, msg []byte) error {
	f.from = from
	f.recipients = append([]string(nil), to...)
	f.rawMessage = append([]byte(nil), msg...)
	return f.sendErr
}

func (f *fakeComposeSMTPClient) Close() error {
	f.closed = true
	return nil
}

func newComposeTestOps(t *testing.T) (*composeOps, *account.Store, *contact.Store) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "compose.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	contactStore := contact.NewStore(db.DB)
	return &composeOps{
		accountStore: accountStore,
		folderStore:  folder.NewStore(db),
		contactStore: contactStore,
	}, accountStore, contactStore
}

func createComposeTestAccount(t *testing.T, store *account.Store, configure func(*account.AccountConfig)) *account.Account {
	t.Helper()
	config := &account.AccountConfig{
		Name:         "Sender",
		DisplayName:  "Sender",
		Email:        "sender@example.com",
		IMAPHost:     "imap.gmail.com",
		IMAPPort:     993,
		IMAPSecurity: account.SecurityTLS,
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     587,
		SMTPSecurity: account.SecurityStartTLS,
		Username:     "sender@example.com",
	}
	if configure != nil {
		configure(config)
	}
	acc, err := store.Create(config)
	if err != nil {
		t.Fatalf("Create account: %v", err)
	}
	return acc
}

func composeTestMessage() smtp.ComposeMessage {
	return smtp.ComposeMessage{
		From:     smtp.Address{Name: "Sender", Address: "sender@example.com"},
		To:       []smtp.Address{{Name: "Recipient", Address: "to@example.com"}},
		Cc:       []smtp.Address{{Name: "Copy", Address: "cc@example.com"}},
		Bcc:      []smtp.Address{{Name: "Blind", Address: "bcc@example.com"}},
		Subject:  "Compose integration",
		TextBody: "Synthetic message body",
	}
}

func TestComposeSendRejectsMissingAndReceiveOnlyAccountsBeforeSMTP(t *testing.T) {
	ops, accountStore, _ := newComposeTestOps(t)
	createdClients := 0
	ops.newSMTPClient = func(smtp.ClientConfig) composeSMTPClient {
		createdClients++
		return &fakeComposeSMTPClient{}
	}

	if _, err := ops.sendMessage(context.Background(), "missing", composeTestMessage(), nil); err == nil || !strings.Contains(err.Error(), "account not found") {
		t.Fatalf("missing account error = %v, want account not found", err)
	}

	receiveOnly := createComposeTestAccount(t, accountStore, func(config *account.AccountConfig) {
		config.Email = "receive-only@example.com"
		config.Username = config.Email
		config.NoOutgoingServer = true
		config.SMTPHost = ""
	})
	if _, err := ops.sendMessage(context.Background(), receiveOnly.ID, composeTestMessage(), nil); err == nil || !strings.Contains(err.Error(), "receive-only") {
		t.Fatalf("receive-only error = %v, want receive-only rejection", err)
	}
	if createdClients != 0 {
		t.Fatalf("created %d SMTP clients before account validation, want 0", createdClients)
	}
}

func TestComposeSpecialFolderFallsBackFromNonSelectableMapping(t *testing.T) {
	ops, accountStore, _ := newComposeTestOps(t)
	acc := createComposeTestAccount(t, accountStore, func(config *account.AccountConfig) {
		config.SentFolderPath = "Sent Group"
	})

	mapped := &folder.Folder{
		AccountID: acc.ID,
		Name:      "Sent Group",
		Path:      "Sent Group",
		Type:      folder.TypeFolder,
		NoSelect:  true,
	}
	detected := &folder.Folder{
		AccountID: acc.ID,
		Name:      "Sent",
		Path:      "Sent",
		Type:      folder.TypeSent,
	}
	for _, candidate := range []*folder.Folder{mapped, detected} {
		if err := ops.folderStore.Create(candidate); err != nil {
			t.Fatalf("create folder %q: %v", candidate.Path, err)
		}
	}

	got, err := ops.getSpecialFolder(acc.ID, folder.TypeSent)
	if err != nil {
		t.Fatalf("getSpecialFolder: %v", err)
	}
	if got == nil || got.ID != detected.ID {
		t.Fatalf("getSpecialFolder = %#v, want selectable detected folder %#v", got, detected)
	}
}

func TestComposeSpecialFolderFallsBackFromStaleMapping(t *testing.T) {
	ops, accountStore, _ := newComposeTestOps(t)
	acc := createComposeTestAccount(t, accountStore, func(config *account.AccountConfig) {
		config.SentFolderPath = "Deleted Sent Folder"
	})

	detected := &folder.Folder{
		AccountID: acc.ID,
		Name:      "Sent",
		Path:      "Sent",
		Type:      folder.TypeSent,
	}
	if err := ops.folderStore.Create(detected); err != nil {
		t.Fatalf("create detected folder: %v", err)
	}

	got, err := ops.getSpecialFolder(acc.ID, folder.TypeSent)
	if err != nil {
		t.Fatalf("getSpecialFolder: %v", err)
	}
	if got == nil || got.ID != detected.ID {
		t.Fatalf("getSpecialFolder = %#v, want selectable detected folder %#v", got, detected)
	}
}

func TestComposeSpecialFolderPropagatesMappedFolderStoreFailure(t *testing.T) {
	ops, accountStore, _ := newComposeTestOps(t)
	acc := createComposeTestAccount(t, accountStore, func(config *account.AccountConfig) {
		config.SentFolderPath = "Mapped Sent"
	})

	failedDB, err := database.Open(filepath.Join(t.TempDir(), "closed-folder-store.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	if err := failedDB.Close(); err != nil {
		t.Fatalf("close folder database: %v", err)
	}
	ops.folderStore = folder.NewStore(failedDB)

	got, err := ops.getSpecialFolder(acc.ID, folder.TypeSent)
	if err == nil || !strings.Contains(err.Error(), "failed to get folder") {
		t.Fatalf("getSpecialFolder = (%#v, %v), want mapped-folder storage error", got, err)
	}
}

func TestComposeSendReportsCredentialAndMessageBuildErrorsBeforeSMTP(t *testing.T) {
	ops, accountStore, _ := newComposeTestOps(t)
	acc := createComposeTestAccount(t, accountStore, nil)
	createdClients := 0
	ops.newSMTPClient = func(smtp.ClientConfig) composeSMTPClient {
		createdClients++
		return &fakeComposeSMTPClient{}
	}

	invalidAttachment := composeTestMessage()
	invalidAttachment.Attachments = []smtp.Attachment{{
		Filename:      "broken.txt",
		ContentType:   "text/plain",
		ContentBase64: "not-base64",
	}}
	if _, err := ops.sendMessage(context.Background(), acc.ID, invalidAttachment, nil); err == nil || !strings.Contains(err.Error(), "failed to build message") {
		t.Fatalf("invalid attachment error = %v, want message build failure", err)
	}

	credentialErr := errors.New("credential unavailable")
	ops.credStore = &fakeComposeCredentials{passwordErr: credentialErr}
	if _, err := ops.sendMessage(context.Background(), acc.ID, composeTestMessage(), nil); err == nil || !strings.Contains(err.Error(), "failed to get password") {
		t.Fatalf("credential error = %v, want password failure", err)
	}
	if createdClients != 0 {
		t.Fatalf("created %d SMTP clients before message and credential validation, want 0", createdClients)
	}
}

func TestComposeSendPropagatesSMTPFailuresAndClosesConnectedClients(t *testing.T) {
	testCases := []struct {
		name       string
		client     *fakeComposeSMTPClient
		wantError  string
		wantClosed bool
	}{
		{name: "connect", client: &fakeComposeSMTPClient{connectErr: errors.New("dial failed")}, wantError: "failed to connect", wantClosed: false},
		{name: "login", client: &fakeComposeSMTPClient{loginErr: errors.New("auth failed")}, wantError: "failed to login", wantClosed: true},
		{name: "send", client: &fakeComposeSMTPClient{sendErr: errors.New("server rejected data")}, wantError: "failed to send", wantClosed: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ops, accountStore, _ := newComposeTestOps(t)
			acc := createComposeTestAccount(t, accountStore, nil)
			ops.credStore = &fakeComposeCredentials{password: "synthetic-password"}
			ops.newSMTPClient = func(smtp.ClientConfig) composeSMTPClient { return testCase.client }

			if _, err := ops.sendMessage(context.Background(), acc.ID, composeTestMessage(), nil); err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("send error = %v, want %q", err, testCase.wantError)
			}
			if testCase.client.closed != testCase.wantClosed {
				t.Fatalf("closed = %v, want %v", testCase.client.closed, testCase.wantClosed)
			}
		})
	}
}

func TestComposeSendUsesSeparateCredentialsAndCollectsRecipients(t *testing.T) {
	ops, accountStore, contactStore := newComposeTestOps(t)
	acc := createComposeTestAccount(t, accountStore, func(config *account.AccountConfig) {
		config.SMTPUsername = "smtp-user@example.com"
	})
	credentials := &fakeComposeCredentials{
		password:     "imap-password",
		smtpPassword: "smtp-password",
	}
	ops.credStore = credentials
	client := &fakeComposeSMTPClient{}
	var capturedConfig smtp.ClientConfig
	ops.newSMTPClient = func(config smtp.ClientConfig) composeSMTPClient {
		capturedConfig = config
		return client
	}

	returnedAccount, err := ops.sendMessage(context.Background(), acc.ID, composeTestMessage(), nil)
	if err != nil {
		t.Fatalf("sendMessage: %v", err)
	}
	if returnedAccount.ID != acc.ID {
		t.Fatalf("returned account = %q, want %q", returnedAccount.ID, acc.ID)
	}
	if capturedConfig.Username != "smtp-user@example.com" || capturedConfig.Password != "smtp-password" {
		t.Fatalf("SMTP credentials = (%q, %q), want separate credentials", capturedConfig.Username, capturedConfig.Password)
	}
	if credentials.passwordCalls != 0 || credentials.smtpPasswordCalls != 1 {
		t.Fatalf("credential calls = password:%d smtp:%d, want 0/1", credentials.passwordCalls, credentials.smtpPasswordCalls)
	}
	if !client.connected || !client.loggedIn || !client.closed {
		t.Fatalf("SMTP lifecycle = connected:%v loggedIn:%v closed:%v", client.connected, client.loggedIn, client.closed)
	}
	wantRecipients := []string{"to@example.com", "cc@example.com", "bcc@example.com"}
	if !reflect.DeepEqual(client.recipients, wantRecipients) {
		t.Fatalf("recipients = %#v, want %#v", client.recipients, wantRecipients)
	}
	if client.from != "sender@example.com" {
		t.Fatalf("from = %q, want sender@example.com", client.from)
	}
	messageText := string(client.rawMessage)
	if !strings.Contains(messageText, "Subject: Compose integration") || strings.Contains(messageText, "Bcc:") {
		t.Fatalf("unexpected RFC822 message:\n%s", messageText)
	}

	for _, address := range []string{"to@example.com", "cc@example.com", "bcc@example.com"} {
		stored, getErr := contactStore.Get(address)
		if getErr != nil {
			t.Fatalf("Get contact %s: %v", address, getErr)
		}
		if stored == nil || stored.SendCount != 1 {
			t.Fatalf("contact %s = %#v, want one collected send", address, stored)
		}
	}
}

func TestComposeAttachmentAndFormattingHelpers(t *testing.T) {
	attachmentPath := filepath.Join(t.TempDir(), "Quarterly Report.PDF")
	wantContent := []byte("synthetic attachment")
	if err := os.WriteFile(attachmentPath, wantContent, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	attachment, err := readFileAsAttachment(attachmentPath)
	if err != nil {
		t.Fatalf("readFileAsAttachment: %v", err)
	}
	if attachment.Filename != "Quarterly Report.PDF" || attachment.ContentType != "application/pdf" || attachment.Size != len(wantContent) {
		t.Fatalf("attachment metadata = %#v", attachment)
	}
	if attachment.Data != base64.StdEncoding.EncodeToString(wantContent) {
		t.Fatalf("attachment data = %q, want base64 content", attachment.Data)
	}
	if _, err := readFileAsAttachment(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("missing attachment should fail")
	}

	if got := escapeHTML(`<tag attr="x">&`); got != "&lt;tag attr=&quot;x&quot;&gt;&amp;" {
		t.Fatalf("escapeHTML = %q", got)
	}
	if got := quoteText("one\ntwo"); got != "> one\n> two" {
		t.Fatalf("quoteText = %q", got)
	}
	for input, want := range map[string]string{"": "", "abc@example.com": "<abc@example.com>", " <abc@example.com> ": "<abc@example.com>"} {
		if got := ensureAngleBrackets(input); got != want {
			t.Errorf("ensureAngleBrackets(%q) = %q, want %q", input, got, want)
		}
	}
	for filename, want := range map[string]string{
		"photo.JPG": "image/jpeg", "photo.JPEG": "image/jpeg", "image.png": "image/png",
		"image.gif": "image/gif", "image.webp": "image/webp", "image.svg": "image/svg+xml",
		"image.ico": "image/x-icon", "image.bmp": "image/bmp", "report.pdf": "application/pdf",
		"document.doc": "application/msword", "document.docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"sheet.xls": "application/vnd.ms-excel", "sheet.xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"slides.ppt": "application/vnd.ms-powerpoint", "slides.pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"document.odt": "application/vnd.oasis.opendocument.text", "sheet.ods": "application/vnd.oasis.opendocument.spreadsheet",
		"slides.odp": "application/vnd.oasis.opendocument.presentation", "notes.txt": "text/plain",
		"page.html": "text/html", "page.htm": "text/html", "style.css": "text/css", "script.js": "text/javascript",
		"data.json": "application/json", "data.xml": "application/xml", "data.csv": "text/csv", "readme.md": "text/markdown",
		"archive.zip": "application/zip", "archive.tar": "application/x-tar", "archive.gz": "application/gzip",
		"archive.gzip": "application/gzip", "archive.7z": "application/x-7z-compressed", "archive.rar": "application/vnd.rar",
		"audio.mp3": "audio/mpeg", "audio.wav": "audio/wav", "audio.ogg": "audio/ogg", "audio.flac": "audio/flac",
		"video.mp4": "video/mp4", "video.webm": "video/webm", "video.avi": "video/x-msvideo",
		"video.mov": "video/quicktime", "video.mkv": "video/x-matroska", "archive.unknown": "application/octet-stream",
	} {
		if got := detectContentType(filename); got != want {
			t.Errorf("detectContentType(%q) = %q, want %q", filename, got, want)
		}
	}
}

func TestAppAttachmentPickerCoversSelectionCancellationAndErrors(t *testing.T) {
	selectedPath := filepath.Join(t.TempDir(), "selected.txt")
	selectedContent := []byte("selected content")
	if err := os.WriteFile(selectedPath, selectedContent, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	missingPath := filepath.Join(t.TempDir(), "missing.txt")
	a := &App{ctx: context.Background()}

	var title string
	a.openMultipleFileDialog = func(gotTitle string) ([]string, error) {
		title = gotTitle
		return []string{selectedPath, missingPath}, nil
	}
	attachments, err := a.PickAttachmentFiles()
	if err != nil {
		t.Fatalf("PickAttachmentFiles() error = %v", err)
	}
	if title != "Select Attachments" || len(attachments) != 1 || attachments[0].Filename != "selected.txt" || attachments[0].Data != base64.StdEncoding.EncodeToString(selectedContent) {
		t.Fatalf("picked attachments = %#v, title %q", attachments, title)
	}

	a.openMultipleFileDialog = func(string) ([]string, error) { return nil, nil }
	if attachments, err := a.PickAttachmentFiles(); err != nil || attachments != nil {
		t.Fatalf("cancelled picker = %#v, %v", attachments, err)
	}

	a.openMultipleFileDialog = func(string) ([]string, error) {
		return nil, errors.New("synthetic picker failure")
	}
	if attachments, err := a.PickAttachmentFiles(); err == nil || attachments != nil || !strings.Contains(err.Error(), "failed to show file picker") {
		t.Fatalf("failed picker = %#v, %v", attachments, err)
	}

	if attachment, err := a.ReadFileAsAttachment(selectedPath); err != nil || attachment == nil || attachment.Size != len(selectedContent) {
		t.Fatalf("ReadFileAsAttachment() = %#v, %v", attachment, err)
	}
}

func TestFilterClipboardFilePathsKeepsUniqueRegularFiles(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.pdf")
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(first, []byte("pdf"), 0o600); err != nil {
		t.Fatalf("write first clipboard file: %v", err)
	}
	if err := os.WriteFile(second, []byte("text"), 0o600); err != nil {
		t.Fatalf("write second clipboard file: %v", err)
	}

	got := filterClipboardFilePaths([]string{
		"",
		first,
		filepath.Join(dir, ".", "first.pdf"),
		dir,
		filepath.Join(dir, "missing.pdf"),
		second,
	})
	if !reflect.DeepEqual(got, []string{first, second}) {
		t.Fatalf("filterClipboardFilePaths() = %#v, want %#v", got, []string{first, second})
	}
}

func TestComposeAddressAndReferenceCompatibilityHelpers(t *testing.T) {
	jsonAddresses := addressListToJSON([]smtp.Address{{Name: "Alice", Address: "alice@example.com"}})
	if got := parseAddressList(jsonAddresses); len(got) != 1 || got[0].Name != "Alice" || got[0].Address != "alice@example.com" {
		t.Fatalf("parseAddressList(address JSON) = %#v", got)
	}
	legacy := parseAddressList("Bob <bob@example.com>, plain@example.com")
	if len(legacy) != 2 || legacy[0].Name != "Bob" || legacy[1].Address != "plain@example.com" {
		t.Fatalf("parseAddressList(legacy) = %#v", legacy)
	}
	filtered := filterSelfAddresses(legacy, map[string]bool{"bob@example.com": true})
	if len(filtered) != 1 || filtered[0].Address != "plain@example.com" {
		t.Fatalf("filterSelfAddresses = %#v", filtered)
	}
	if got := parseAddressList(""); got != nil {
		t.Fatalf("parseAddressList(empty) = %#v, want nil", got)
	}

	refs := []string{"<one@example.com>", "<two@example.com>"}
	if got := parseReferencesList(referencesToJSON(refs)); !reflect.DeepEqual(got, refs) {
		t.Fatalf("JSON references = %#v, want %#v", got, refs)
	}
	if got := parseReferencesList("<one@example.com> <two@example.com>"); !reflect.DeepEqual(got, refs) {
		t.Fatalf("legacy references = %#v, want %#v", got, refs)
	}
	if parseReferencesList("") != nil || referencesToJSON(nil) != "" || addressListToJSON(nil) != "" {
		t.Fatal("empty compatibility values should remain empty")
	}

	if !providerAutoSavesSentMail("IMAP.GMAIL.COM") || providerAutoSavesSentMail("imap.example.com") {
		t.Fatal("provider auto-save detection returned the wrong result")
	}
	contentType, data := parseDataURL("data:image/png;base64,AAAA")
	if contentType != "image/png" || data != "AAAA" {
		t.Fatalf("parseDataURL = (%q, %q)", contentType, data)
	}
}

func TestPrepareReplyAndForwardBuildObservableComposerMessages(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "reply.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	acc := createComposeTestAccount(t, accountStore, nil)
	folderStore := folder.NewStore(db)
	inbox := &folder.Folder{
		ID:        "reply-inbox",
		AccountID: acc.ID,
		Name:      "Inbox",
		Path:      "INBOX",
		Type:      folder.TypeInbox,
	}
	if err := folderStore.Create(inbox); err != nil {
		t.Fatalf("Create folder: %v", err)
	}

	messageStore := message.NewStore(db)
	attachmentStore := message.NewAttachmentStore(db)
	original := &message.Message{
		ID:          "original-message",
		AccountID:   acc.ID,
		FolderID:    inbox.ID,
		UID:         42,
		MessageID:   "original@example.com",
		References:  `["<root@example.com>"]`,
		FromName:    "Original Sender",
		FromEmail:   "sender-of-message@example.com",
		ReplyTo:     "reply-here@example.com",
		ToList:      addressListToJSON([]smtp.Address{{Name: "Me", Address: acc.Email}, {Name: "Team", Address: "team@example.com"}}),
		CcList:      addressListToJSON([]smtp.Address{{Name: "Team duplicate", Address: "team@example.com"}, {Name: "Observer", Address: "observer@example.com"}}),
		Subject:     "Status update",
		BodyHTML:    `<p>Hello</p><img src="https://tracker.example/pixel.png">`,
		BodyText:    "Hello\nWorld",
		BodyFetched: true,
		Date:        time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC),
	}
	if err := messageStore.Create(original); err != nil {
		t.Fatalf("Create message: %v", err)
	}

	testApp := &App{
		ctx:             context.Background(),
		accountStore:    accountStore,
		folderStore:     folderStore,
		messageStore:    messageStore,
		attachmentStore: attachmentStore,
	}

	reply, err := testApp.PrepareReply(original.ID, "reply-all")
	if err != nil {
		t.Fatalf("PrepareReply(reply-all): %v", err)
	}
	if reply.Subject != "Re: Status update" || reply.InReplyTo != "<original@example.com>" {
		t.Fatalf("reply identity = subject:%q inReplyTo:%q", reply.Subject, reply.InReplyTo)
	}
	if len(reply.To) != 2 || reply.To[0].Address != "reply-here@example.com" || reply.To[1].Address != "team@example.com" {
		t.Fatalf("reply To = %#v", reply.To)
	}
	if len(reply.Cc) != 1 || reply.Cc[0].Address != "observer@example.com" {
		t.Fatalf("reply Cc = %#v", reply.Cc)
	}
	if reply.From.Address != acc.Email || reply.SourceMessageID != original.ID || reply.ReplyType != "reply-all" {
		t.Fatalf("reply compose context = %#v", reply)
	}
	if !reflect.DeepEqual(reply.References, []string{"<root@example.com>", "<original@example.com>"}) {
		t.Fatalf("reply references = %#v", reply.References)
	}
	if !strings.Contains(reply.HTMLBody, "blockquote") || strings.Contains(reply.HTMLBody, `<img src="https://tracker.example`) || !strings.Contains(reply.HTMLBody, `data-original-src="https://tracker.example`) {
		t.Fatalf("reply HTML did not quote and block remote image: %s", reply.HTMLBody)
	}
	if !strings.Contains(reply.TextBody, "> Hello\n> World") {
		t.Fatalf("reply text did not quote original: %q", reply.TextBody)
	}

	forward, err := testApp.PrepareReply(original.ID, "forward")
	if err != nil {
		t.Fatalf("PrepareReply(forward): %v", err)
	}
	if forward.Subject != "Fwd: Status update" || len(forward.To) != 0 || forward.ReplyType != "forward" {
		t.Fatalf("forward envelope = %#v", forward)
	}
	if !strings.Contains(forward.TextBody, "Forwarded message") || !strings.Contains(forward.HTMLBody, "Forwarded message") {
		t.Fatalf("forward body omitted forwarded header: %#v", forward)
	}

	if _, err := testApp.PrepareReply("missing-message", "reply"); err == nil || !strings.Contains(err.Error(), "message not found") {
		t.Fatalf("missing message error = %v", err)
	}
}

func TestComposeSentAppendAndCredentialResolutionAgainstMemoryIMAP(t *testing.T) {
	harness := startActionIMAPServer(t)
	fixtureClient := harness.client(t)
	if err := fixtureClient.RawClient().Create("Sent", nil).Wait(); err != nil {
		_ = fixtureClient.ForceClose()
		t.Fatalf("create Sent mailbox: %v", err)
	}
	if err := fixtureClient.Close(); err != nil {
		t.Fatalf("close fixture client: %v", err)
	}

	db, err := database.Open(filepath.Join(t.TempDir(), "compose-sent.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("database.Migrate: %v", err)
	}
	accountStore := account.NewStore(db)
	acc, err := accountStore.Create(&account.AccountConfig{
		Name: "Memory IMAP", DisplayName: "Memory Sender", Email: actionIMAPUsername,
		Username: actionIMAPUsername, IMAPHost: harness.host, IMAPPort: harness.port,
		IMAPSecurity: account.SecurityNone, SMTPHost: "smtp.example.com", SMTPPort: 587,
		SMTPSecurity: account.SecurityStartTLS, AuthType: account.AuthPassword,
	})
	if err != nil {
		t.Fatalf("create memory account: %v", err)
	}
	folderStore := folder.NewStore(db)
	sent := &folder.Folder{ID: "compose-sent", AccountID: acc.ID, Name: "Sent", Path: "Sent", Type: folder.TypeSent}
	archive := &folder.Folder{ID: "compose-mapped-sent", AccountID: acc.ID, Name: "Mapped Sent", Path: "Archive", Type: folder.TypeArchive}
	for _, item := range []*folder.Folder{sent, archive} {
		if err := folderStore.Create(item); err != nil {
			t.Fatalf("create folder %s: %v", item.ID, err)
		}
	}

	credentials := &fakeComposeCredentials{password: actionIMAPPassword}
	ops := &composeOps{
		accountStore: accountStore,
		folderStore:  folderStore,
		contactStore: contact.NewStore(db.DB),
		credStore:    credentials,
	}
	config, err := ops.getIMAPCredentials(acc.ID)
	if err != nil {
		t.Fatalf("getIMAPCredentials: %v", err)
	}
	if config.Host != harness.host || config.Port != harness.port || config.Password != actionIMAPPassword {
		t.Fatalf("resolved IMAP config = %#v", config)
	}
	if _, err := ops.getIMAPCredentials("missing-account"); err == nil || !strings.Contains(err.Error(), "account not found") {
		t.Fatalf("missing IMAP account error = %v", err)
	}

	raw := []byte("From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: sent append\r\nMessage-ID: <compose-sent@example.com>\r\n\r\nsent body\r\n")
	if err := ops.saveToSentFolder(acc.ID, acc, raw); err != nil {
		t.Fatalf("saveToSentFolder: %v", err)
	}
	if status := harness.status(t, "Sent"); status.Messages != 1 {
		t.Fatalf("Sent status after append = %#v", status)
	}
	if flags := harness.flags(t, "Sent", 1); !containsActionFlag(flags, goImap.FlagSeen) {
		t.Fatalf("sent message flags = %v", flags)
	}

	if _, err := db.Exec(`UPDATE accounts SET sent_folder_path = 'Archive' WHERE id = ?`, acc.ID); err != nil {
		t.Fatalf("set sent-folder mapping: %v", err)
	}
	mapped, err := ops.getSpecialFolder(acc.ID, folder.TypeSent)
	if err != nil || mapped == nil || mapped.ID != archive.ID {
		t.Fatalf("mapped sent folder = (%#v, %v)", mapped, err)
	}
	if _, err := db.Exec(`UPDATE accounts SET sent_folder_path = '' WHERE id = ?`, acc.ID); err != nil {
		t.Fatalf("clear sent-folder mapping: %v", err)
	}

	ops.credStore = &fakeComposeCredentials{passwordErr: errors.New("Keychain unavailable")}
	if err := ops.saveToSentFolder(acc.ID, acc, raw); err == nil || !strings.Contains(err.Error(), "failed to get IMAP credentials") {
		t.Fatalf("sent append credential error = %v", err)
	}
	ops.credStore = &fakeComposeCredentials{password: "wrong-password"}
	if err := ops.saveToSentFolder(acc.ID, acc, raw); err == nil || !strings.Contains(err.Error(), "failed to login") {
		t.Fatalf("sent append login error = %v", err)
	}
	if err := ops.saveToSentFolder("missing-account", nil, raw); err == nil || !strings.Contains(err.Error(), "failed to get account") {
		t.Fatalf("missing account error = %v", err)
	}
}

func TestAppSendMessagePropagatesComposeValidationErrors(t *testing.T) {
	ops, accountStore, _ := newComposeTestOps(t)
	testApp := &App{
		ctx:          context.Background(),
		accountStore: accountStore,
		folderStore:  ops.folderStore,
		composeOps:   *ops,
	}
	if err := testApp.SendMessage("missing-account", composeTestMessage()); err == nil || !strings.Contains(err.Error(), "account not found") {
		t.Fatalf("SendMessage missing account error = %v", err)
	}
}
