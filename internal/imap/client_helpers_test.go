package imap

import (
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
)

type helperTestAddr string

func (a helperTestAddr) Network() string { return "test" }
func (a helperTestAddr) String() string  { return string(a) }

type recordingConn struct {
	readData         []byte
	written          []byte
	readDeadline     time.Time
	writeDeadline    time.Time
	readDeadlineErr  error
	writeDeadlineErr error
}

func (c *recordingConn) Read(p []byte) (int, error) {
	if len(c.readData) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.readData)
	c.readData = c.readData[n:]
	return n, nil
}

func (c *recordingConn) Write(p []byte) (int, error) {
	c.written = append(c.written, p...)
	return len(p), nil
}

func (c *recordingConn) Close() error                { return nil }
func (c *recordingConn) LocalAddr() net.Addr         { return helperTestAddr("local") }
func (c *recordingConn) RemoteAddr() net.Addr        { return helperTestAddr("remote") }
func (c *recordingConn) SetDeadline(time.Time) error { return nil }
func (c *recordingConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return c.readDeadlineErr
}
func (c *recordingConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return c.writeDeadlineErr
}

func TestDeadlineConnAppliesTimeoutsAndDelegatesIO(t *testing.T) {
	underlying := &recordingConn{readData: []byte("mail")}
	conn := &deadlineConn{Conn: underlying, readTimeout: time.Minute, writeTimeout: 30 * time.Second}
	readBuffer := make([]byte, 4)
	if n, err := conn.Read(readBuffer); err != nil || n != 4 || string(readBuffer) != "mail" {
		t.Fatalf("Read = (%d, %v, %q)", n, err, readBuffer)
	}
	if underlying.readDeadline.IsZero() {
		t.Fatal("read deadline was not set")
	}
	if n, err := conn.Write([]byte("command")); err != nil || n != 7 || string(underlying.written) != "command" {
		t.Fatalf("Write = (%d, %v, %q)", n, err, underlying.written)
	}
	if underlying.writeDeadline.IsZero() {
		t.Fatal("write deadline was not set")
	}

	readErr := errors.New("read deadline failed")
	underlying.readDeadlineErr = readErr
	if _, err := conn.Read(readBuffer); !errors.Is(err, readErr) {
		t.Fatalf("read deadline error = %v", err)
	}
	writeErr := errors.New("write deadline failed")
	underlying.writeDeadlineErr = writeErr
	if _, err := conn.Write([]byte("ignored")); !errors.Is(err, writeErr) {
		t.Fatalf("write deadline error = %v", err)
	}
}

func TestClientCapabilityAndDisconnectedHelpers(t *testing.T) {
	client := NewClient(ClientConfig{Host: "imap.example.com", Port: 993})
	client.caps = imap.CapSet{imap.CapIdle: {}, imap.CapCondStore: {}, imap.CapQResync: {}}
	if !client.HasCap(imap.CapIdle) || !client.SupportsIdle() || !client.SupportsCondStore() || !client.SupportsQResync() {
		t.Fatalf("capability helpers failed for %#v", client.Caps())
	}
	if client.RawClient() != nil {
		t.Fatal("new disconnected client unexpectedly has a raw client")
	}
	if err := client.Login(); err == nil || err.Error() != "not connected" {
		t.Fatalf("disconnected Login error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("disconnected Close: %v", err)
	}
	if err := client.ForceClose(); err != nil {
		t.Fatalf("disconnected ForceClose: %v", err)
	}
	if _, err := client.AppendMessage("INBOX", nil, time.Time{}, []byte("message")); err == nil {
		t.Fatal("disconnected AppendMessage should fail")
	}
	if _, err := client.AppendMessageFromReader("INBOX", nil, time.Time{}, -1, nil); err == nil {
		t.Fatal("disconnected AppendMessageFromReader should fail")
	}
}

func TestMailboxClassificationAndLoggingConversions(t *testing.T) {
	attributeCases := map[imap.MailboxAttr]FolderType{
		imap.MailboxAttrAll: FolderTypeAll, imap.MailboxAttrArchive: FolderTypeArchive,
		imap.MailboxAttrDrafts: FolderTypeDrafts, imap.MailboxAttrJunk: FolderTypeSpam,
		imap.MailboxAttrSent: FolderTypeSent, imap.MailboxAttrTrash: FolderTypeTrash,
		imap.MailboxAttrFlagged: FolderTypeStarred,
	}
	for attr, want := range attributeCases {
		if got := determineFolderType("unrelated", []imap.MailboxAttr{attr}); got != want {
			t.Errorf("determineFolderType attribute %q = %q, want %q", attr, got, want)
		}
	}
	nameCases := map[string]FolderType{
		"INBOX": FolderTypeInbox, "Sent Items": FolderTypeSent, "Draft Box": FolderTypeDrafts,
		"Deleted Items": FolderTypeTrash, "Junk Mail": FolderTypeSpam, "Archive": FolderTypeArchive,
		"All Mail": FolderTypeAll, "Flagged": FolderTypeStarred, "Projects": FolderTypeFolder,
	}
	for name, want := range nameCases {
		if got := determineFolderType(name, nil); got != want {
			t.Errorf("determineFolderType(%q) = %q, want %q", name, got, want)
		}
	}
	if !containsIgnoreCase("Mixed CASE Folder", "case") || containsIgnoreCase("short", "longer") {
		t.Fatal("containsIgnoreCase returned the wrong result")
	}
	if !hasSpecialUseAttr([]string{string(imap.MailboxAttrTrash)}) || hasSpecialUseAttr([]string{"\\NoSelect"}) {
		t.Fatal("hasSpecialUseAttr returned the wrong result")
	}

	flags := []imap.Flag{imap.FlagSeen, imap.FlagFlagged}
	if got := flagsToStrings(flags); !reflect.DeepEqual(got, []string{string(imap.FlagSeen), string(imap.FlagFlagged)}) {
		t.Fatalf("flagsToStrings = %#v", got)
	}
	uids := []imap.UID{1, 42}
	if got := uidsToUint32s(uids); !reflect.DeepEqual(got, []uint32{1, 42}) {
		t.Fatalf("uidsToUint32s = %#v", got)
	}
	caps := capsToStrings(imap.CapSet{imap.CapIdle: {}})
	if !reflect.DeepEqual(caps, []string{string(imap.CapIdle)}) {
		t.Fatalf("capsToStrings = %#v", caps)
	}
	if EventNewMail.String() != "NewMail" || EventExpunge.String() != "Expunge" || EventFlagsChanged.String() != "FlagsChanged" || EventType(99).String() != "Unknown" {
		t.Fatal("event type strings are inconsistent")
	}
}
