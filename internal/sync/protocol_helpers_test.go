package sync

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/folder"
	imapPkg "aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/message"
	"github.com/emersion/go-imap/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestCharsetHelpersDecodeDeclaredAndQuotedPrintableContent(t *testing.T) {
	want := "中文正文"
	encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(want))
	if err != nil {
		t.Fatalf("encode GBK fixture: %v", err)
	}
	if got := DecodeTextCharset(encoded, ` "gbk" `); got != want {
		t.Fatalf("DecodeTextCharset = %q, want %q", got, want)
	}
	reader, err := CharsetReader("gbk", bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("CharsetReader: %v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil || string(decoded) != want {
		t.Fatalf("CharsetReader output = (%q, %v)", decoded, err)
	}
	plainReader := strings.NewReader("plain")
	returned, err := CharsetReader("", plainReader)
	if err != nil || returned != plainReader {
		t.Fatalf("empty CharsetReader = (%v, %v), want original reader", returned, err)
	}
	if _, err := CharsetReader("definitely-unknown-charset", strings.NewReader("x")); err == nil {
		t.Fatal("unknown charset should fail")
	}

	if got := string(decodeQuotedPrintableIfNeeded([]byte("name=3Dvalue"))); got != "name=value" {
		t.Fatalf("quoted-printable decode = %q", got)
	}
	plain := []byte("unchanged")
	if got := decodeQuotedPrintableIfNeeded(plain); !bytes.Equal(got, plain) {
		t.Fatalf("plain content changed to %q", got)
	}
}

func TestCharsetDetectionAndGibberishHeuristics(t *testing.T) {
	tests := []struct {
		html string
		want string
	}{
		{html: `<html><meta charset="gbk"><body></body></html>`, want: "gbk"},
		{html: `<meta http-equiv="Content-Type" content="text/html; charset=big5">`, want: "big5"},
		{html: `<html><head></head></html>`, want: ""},
	}
	for _, testCase := range tests {
		if got := extractCharsetFromHTML([]byte(testCase.html)); got != testCase.want {
			t.Errorf("extractCharsetFromHTML(%q) = %q, want %q", testCase.html, got, testCase.want)
		}
	}
	if looksLikeGibberish("ordinary readable text") {
		t.Fatal("ordinary text classified as gibberish")
	}
	if !looksLikeGibberish(strings.Repeat("�", 12)) {
		t.Fatal("replacement-heavy text should be gibberish")
	}
	if !looksLikeGibberish(strings.Repeat("𠀀", 22)) {
		t.Fatal("extension-B-heavy text should be gibberish")
	}
}

func TestApplyEnvelopeFlagsAndFolderMappings(t *testing.T) {
	envelope := &imap.Envelope{
		Date:      time.Date(2026, time.July, 31, 3, 0, 0, 0, time.FixedZone("offset", 8*60*60)),
		Subject:   "=?UTF-8?Q?Status_=E2=9C=93?=",
		From:      []imap.Address{{Name: `"Sender"`, Mailbox: "sender", Host: "example.com"}},
		To:        []imap.Address{{Name: "Receiver", Mailbox: "to", Host: "example.com"}},
		Cc:        []imap.Address{{Mailbox: "cc", Host: "example.com"}},
		ReplyTo:   []imap.Address{{Mailbox: "reply", Host: "example.com"}},
		MessageID: "message@example.com",
		InReplyTo: []string{"parent@example.com"},
	}
	got := &message.Message{}
	applyEnvelopeToMessage(got, envelope)
	applyFlagsToMessage(got, []imap.Flag{imap.FlagSeen, imap.FlagFlagged, imap.FlagAnswered, imap.FlagDraft, imap.FlagDeleted, "$Forwarded"})
	if got.Subject != "Status ✓" || got.FromName != "Sender" || got.FromEmail != "sender@example.com" || got.ReplyTo != "reply@example.com" {
		t.Fatalf("mapped envelope = %#v", got)
	}
	if !got.Date.Equal(envelope.Date.UTC()) || got.MessageID != envelope.MessageID || got.InReplyTo != envelope.InReplyTo[0] {
		t.Fatalf("mapped identity/date = %#v", got)
	}
	if !got.IsRead || !got.IsStarred || !got.IsAnswered || !got.IsDraft || !got.IsDeleted || !got.IsForwarded {
		t.Fatalf("mapped flags = %#v", got)
	}
	var to []map[string]string
	if err := json.Unmarshal([]byte(got.ToList), &to); err != nil || len(to) != 1 || to[0]["email"] != "to@example.com" {
		t.Fatalf("mapped To = (%#v, %v)", to, err)
	}
	untouched := &message.Message{Subject: "keep"}
	applyEnvelopeToMessage(untouched, nil)
	if untouched.Subject != "keep" {
		t.Fatal("nil envelope mutated message")
	}

	mappings := map[imapPkg.FolderType]folder.Type{
		imapPkg.FolderTypeInbox: folder.TypeInbox, imapPkg.FolderTypeSent: folder.TypeSent,
		imapPkg.FolderTypeDrafts: folder.TypeDrafts, imapPkg.FolderTypeTrash: folder.TypeTrash,
		imapPkg.FolderTypeSpam: folder.TypeSpam, imapPkg.FolderTypeArchive: folder.TypeArchive,
		imapPkg.FolderTypeAll: folder.TypeAll, imapPkg.FolderTypeStarred: folder.TypeStarred,
		imapPkg.FolderTypeFolder: folder.TypeFolder,
	}
	for input, wantType := range mappings {
		if actual := convertFolderType(input); actual != wantType {
			t.Errorf("convertFolderType(%q) = %q, want %q", input, actual, wantType)
		}
	}
	if got := extractFolderName("Parent/Child", "/"); got != "Child" {
		t.Fatalf("extractFolderName = %q", got)
	}
	if got := extractFolderName("INBOX", ""); got != "INBOX" {
		t.Fatalf("delimiter-free extractFolderName = %q", got)
	}
}

func TestThreadingHeaderExtraction(t *testing.T) {
	engine := &Engine{}
	raw := []byte("References: <root@example.com> invalid <parent@example.com>\r\n" +
		"Disposition-Notification-To: Reader <reader@example.com>\r\n" +
		"Subject: test\r\n\r\nbody")
	if got := engine.extractReferences(raw); !reflect.DeepEqual(got, []string{"<root@example.com>", "<parent@example.com>"}) {
		t.Fatalf("extractReferences = %#v", got)
	}
	if got := engine.extractDispositionNotificationTo(raw); got != "Reader <reader@example.com>" {
		t.Fatalf("extractDispositionNotificationTo = %q", got)
	}
	if engine.extractReferences([]byte("not a message")) != nil {
		t.Fatal("malformed message should not return references")
	}
	if engine.extractDispositionNotificationTo([]byte("Subject: no receipt\r\n\r\nbody")) != "" {
		t.Fatal("missing disposition header should return empty")
	}
}

func TestSyncOptionDefaultsAndNormalization(t *testing.T) {
	if got := LegacyMessageSyncOptions(-1); got.RetentionDays != 30 || got.Strategy != account.SyncStrategyFull {
		t.Fatalf("LegacyMessageSyncOptions(-1) = %#v", got)
	}
	if got := MessageSyncOptionsFromAccount(nil); got.RetentionDays != 30 || got.Strategy != account.SyncStrategyFull {
		t.Fatalf("nil MessageSyncOptionsFromAccount = %#v", got)
	}
	acc := &account.Account{LocalRetentionDays: -1, SyncStrategy: "unexpected", FullCheckIntervalDays: -1}
	got := MessageSyncOptionsFromAccount(acc)
	if got.RetentionDays != account.DefaultLocalRetentionDays || got.Strategy != account.SyncStrategyIncremental || got.FullCheckIntervalDays != account.DefaultFullCheckIntervalDays {
		t.Fatalf("normalized options = %#v", got)
	}
	if got := BodyFetchOptionsFromAccount(nil); !got.Enabled || got.Days != 30 {
		t.Fatalf("nil BodyFetchOptionsFromAccount = %#v", got)
	}
	if got := BodyFetchOptionsFromAccount(&account.Account{BodyDownloadPolicy: account.BodyDownloadRecent}); !got.Enabled || got.Days != account.DefaultBodyDownloadRecentDays {
		t.Fatalf("recent default body options = %#v", got)
	}
	if got := BodyFetchOptionsFromAccount(&account.Account{BodyDownloadPolicy: "unexpected"}); got.Enabled {
		t.Fatalf("unknown body policy = %#v, want disabled", got)
	}
}
