package imap

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	goImap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

const (
	testIMAPUsername = "user@example.com"
	testIMAPPassword = "secret"
)

func startMemoryIMAPServer(t *testing.T) (host string, port int) {
	t.Helper()
	return startMemoryIMAPServerWithCaps(t, goImap.CapSet{
		goImap.CapIMAP4rev1:    {},
		goImap.CapListExtended: {},
		goImap.CapUIDPlus:      {},
		goImap.CapIdle:         {},
	})
}

func startMemoryIMAPServerWithCaps(t *testing.T, caps goImap.CapSet) (host string, port int) {
	t.Helper()
	memory := imapmemserver.New()
	user := imapmemserver.NewUser(testIMAPUsername, testIMAPPassword)
	for _, mailbox := range []string{"INBOX", "Archive", "Sent", "Projects"} {
		if err := user.Create(mailbox, nil); err != nil {
			t.Fatalf("create mailbox %s: %v", mailbox, err)
		}
	}
	if err := user.Subscribe("INBOX"); err != nil {
		t.Fatalf("subscribe INBOX fixture: %v", err)
	}
	memory.AddUser(user)

	server := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memory.NewSession(), nil, nil
		},
		InsecureAuth: true,
		Caps:         caps,
		Logger:       log.New(io.Discard, "", 0),
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for IMAP server: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("close IMAP server: %v", err)
		}
		select {
		case err := <-serveDone:
			if err != nil {
				t.Errorf("serve IMAP server: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("IMAP server did not stop")
		}
	})

	tcpAddr := listener.Addr().(*net.TCPAddr)
	return tcpAddr.IP.String(), tcpAddr.Port
}

func newMemoryIMAPClient(t *testing.T, host string, port int, password string) *Client {
	t.Helper()
	config := DefaultConfig()
	config.Host = host
	config.Port = port
	config.Security = SecurityNone
	config.Username = testIMAPUsername
	config.Password = password
	config.ConnectTimeout = 2 * time.Second
	config.ReadTimeout = 2 * time.Second
	config.WriteTimeout = 2 * time.Second
	client := NewClient(config)
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	return client
}

func TestClientMailboxLifecycleAgainstMemoryServer(t *testing.T) {
	host, port := startMemoryIMAPServer(t)
	client := newMemoryIMAPClient(t, host, port, testIMAPPassword)
	if err := client.Login(); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if !client.HasCap(goImap.CapUIDPlus) || client.SupportsCondStore() || client.SupportsQResync() || !client.SupportsIdle() {
		t.Fatalf("capabilities = %#v", client.Caps())
	}

	mailboxes, err := client.ListMailboxes()
	if err != nil {
		t.Fatalf("ListMailboxes() error = %v", err)
	}
	typesByName := make(map[string]FolderType, len(mailboxes))
	for _, mailbox := range mailboxes {
		typesByName[mailbox.Name] = mailbox.Type
	}
	if typesByName["INBOX"] != FolderTypeInbox || typesByName["Sent"] != FolderTypeSent || typesByName["Archive"] != FolderTypeArchive || typesByName["Projects"] != FolderTypeFolder {
		t.Fatalf("mailbox types = %#v", typesByName)
	}

	if err := client.Subscribe("Archive"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	subscribed, err := client.ListSubscribedMailboxes()
	if err != nil {
		t.Fatalf("ListSubscribedMailboxes() error = %v", err)
	}
	if !reflect.DeepEqual(subscribed, map[string]bool{"Archive": true, "INBOX": true}) {
		t.Fatalf("subscribed mailboxes = %#v", subscribed)
	}
	if err := client.Unsubscribe("Archive"); err != nil {
		t.Fatalf("Unsubscribe() error = %v", err)
	}
	subscribed, err = client.ListSubscribedMailboxes()
	if err != nil || !reflect.DeepEqual(subscribed, map[string]bool{"INBOX": true}) {
		t.Fatalf("subscribed after unsubscribe = %#v, %v", subscribed, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	initial, err := client.GetMailboxStatus(ctx, "INBOX")
	if err != nil {
		t.Fatalf("GetMailboxStatus(initial) error = %v", err)
	}
	if initial.Messages != 0 || initial.Unseen != 0 || initial.UIDNext != 1 || initial.UIDValidity == 0 {
		t.Fatalf("initial status = %+v", initial)
	}

	raw := []byte("From: sender@example.com\r\nTo: user@example.com\r\nSubject: first\r\n\r\nhello\r\n")
	wantDate := time.Date(2026, time.August, 2, 8, 30, 0, 0, time.UTC)
	uid, err := client.AppendMessage("INBOX", []goImap.Flag{goImap.FlagSeen, goImap.FlagFlagged}, wantDate, raw)
	if err != nil || uid != 1 {
		t.Fatalf("AppendMessage() = uid %d, %v", uid, err)
	}
	selected, err := client.SelectMailbox(ctx, "INBOX")
	if err != nil {
		t.Fatalf("SelectMailbox() error = %v", err)
	}
	if selected.Messages != 1 || selected.UIDNext != 2 || selected.UIDValidity == 0 {
		t.Fatalf("selected mailbox = %+v", selected)
	}
	status, err := client.GetMailboxStatus(ctx, "INBOX")
	if err != nil || status.Messages != 1 || status.Unseen != 0 {
		t.Fatalf("status after append = %+v, %v", status, err)
	}

	if err := client.AddMessageFlags([]goImap.UID{uid}, []goImap.Flag{goImap.FlagAnswered}); err != nil {
		t.Fatalf("AddMessageFlags() error = %v", err)
	}
	if err := client.AddMessageFlags(nil, []goImap.Flag{goImap.FlagDraft}); err != nil {
		t.Fatalf("AddMessageFlags(nil) error = %v", err)
	}
	if err := client.RemoveMessageFlags([]goImap.UID{uid}, []goImap.Flag{goImap.FlagSeen}); err != nil {
		t.Fatalf("RemoveMessageFlags() error = %v", err)
	}
	if err := client.RemoveMessageFlags(nil, []goImap.Flag{goImap.FlagSeen}); err != nil {
		t.Fatalf("RemoveMessageFlags(nil) error = %v", err)
	}
	status, err = client.GetMailboxStatus(ctx, "INBOX")
	if err != nil || status.Unseen != 1 {
		t.Fatalf("status after flag removal = %+v, %v", status, err)
	}

	if copied, err := client.CopyMessages([]goImap.UID{uid}, "Archive"); err != nil || len(copied) != 0 {
		t.Fatalf("CopyMessages() = %#v, %v", copied, err)
	}
	if copied, err := client.CopyMessages(nil, "Archive"); err != nil || copied != nil {
		t.Fatalf("CopyMessages(nil) = %#v, %v", copied, err)
	}
	archiveStatus, err := client.GetMailboxStatus(ctx, "Archive")
	if err != nil || archiveStatus.Messages != 1 {
		t.Fatalf("Archive status = %+v, %v", archiveStatus, err)
	}

	if err := client.DeleteMessageByUID(uid); err != nil {
		t.Fatalf("DeleteMessageByUID() error = %v", err)
	}
	status, err = client.GetMailboxStatus(ctx, "INBOX")
	if err != nil || status.Messages != 0 {
		t.Fatalf("status after single delete = %+v, %v", status, err)
	}

	second := []byte("Subject: second\r\n\r\nsecond\r\n")
	secondUID, err := client.AppendMessageFromReader("INBOX", nil, time.Time{}, int64(len(second)), bytes.NewReader(second))
	if err != nil || secondUID != 2 {
		t.Fatalf("AppendMessageFromReader() = uid %d, %v", secondUID, err)
	}
	thirdUID, err := client.AppendMessage("INBOX", nil, time.Time{}, []byte("Subject: third\r\n\r\nthird\r\n"))
	if err != nil || thirdUID != 3 {
		t.Fatalf("third AppendMessage() = uid %d, %v", thirdUID, err)
	}
	if _, err := client.AppendMessageFromReader("INBOX", nil, time.Time{}, -1, strings.NewReader("x")); err == nil || !strings.Contains(err.Error(), "invalid append size") {
		t.Fatalf("negative append error = %v", err)
	}
	if err := client.DeleteMessagesByUID([]goImap.UID{secondUID, thirdUID}); err != nil {
		t.Fatalf("DeleteMessagesByUID() error = %v", err)
	}
	if err := client.DeleteMessagesByUID(nil); err != nil {
		t.Fatalf("DeleteMessagesByUID(nil) error = %v", err)
	}
	status, err = client.GetMailboxStatus(ctx, "INBOX")
	if err != nil || status.Messages != 0 {
		t.Fatalf("final INBOX status = %+v, %v", status, err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestClientRejectsBadLoginAndMissingMailboxOperations(t *testing.T) {
	host, port := startMemoryIMAPServer(t)
	client := newMemoryIMAPClient(t, host, port, "wrong-password")
	if err := client.Login(); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("Login(wrong password) error = %v", err)
	}
	_ = client.ForceClose()

	client = newMemoryIMAPClient(t, host, port, testIMAPPassword)
	if err := client.Login(); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.GetMailboxStatus(ctx, "Missing"); err == nil || !strings.Contains(err.Error(), "failed to get mailbox status") {
		t.Fatalf("GetMailboxStatus(missing) error = %v", err)
	}
	if _, err := client.SelectMailbox(ctx, "Missing"); err == nil || !strings.Contains(err.Error(), "failed to select mailbox") {
		t.Fatalf("SelectMailbox(missing) error = %v", err)
	}
	if err := client.Subscribe("Missing"); err == nil || !strings.Contains(err.Error(), "failed to subscribe") {
		t.Fatalf("Subscribe(missing) error = %v", err)
	}
	if _, err := client.AppendMessage("Missing", nil, time.Time{}, []byte("message")); err == nil || !strings.Contains(err.Error(), "failed to append message") {
		t.Fatalf("AppendMessage(missing) error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	client = newMemoryIMAPClient(t, host, port, testIMAPPassword)
	if err := client.Login(); err != nil {
		t.Fatalf("Login() for short append error = %v", err)
	}
	if _, err := client.AppendMessageFromReader("INBOX", nil, time.Time{}, 2, strings.NewReader("x")); err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("short append error = %v", err)
	}
	_ = client.ForceClose()

	client = newMemoryIMAPClient(t, host, port, testIMAPPassword)
	if err := client.Login(); err != nil {
		t.Fatalf("Login() after aborted append = %v", err)
	}
	status, err := client.GetMailboxStatus(context.Background(), "INBOX")
	if err != nil || status.Messages != 0 {
		t.Fatalf("INBOX after aborted append = %+v, %v", status, err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() after aborted append = %v", err)
	}
}
