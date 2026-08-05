package smtp

import (
	"bytes"
	"net/mail"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/message"
)

func TestBuildMDNValidatesRequiredInputs(t *testing.T) {
	if _, err := BuildMDN(nil, "User", "user@example.com", MDNDisplayed); err == nil {
		t.Fatal("nil original message should fail")
	}
	if _, err := BuildMDN(&message.Message{}, "User", "user@example.com", MDNDisplayed); err == nil {
		t.Fatal("missing receipt request should fail")
	}
	if _, err := BuildMDN(&message.Message{ReadReceiptTo: "sender@example.com"}, "User", "", MDNDisplayed); err == nil {
		t.Fatal("missing from email should fail")
	}
}

func TestBuildMDNProducesMultipartDispositionNotification(t *testing.T) {
	original := &message.Message{
		Subject:       "Quarterly update",
		MessageID:     "<original@example.com>",
		ReadReceiptTo: "Requester <requester@example.com>",
		Date:          time.Date(2026, time.July, 31, 8, 0, 0, 0, time.UTC),
	}
	raw, err := BuildMDN(original, `User, "Desktop"`, "user@example.com", MDNDisplayed)
	if err != nil {
		t.Fatalf("BuildMDN: %v", err)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if parsed.Header.Get("To") != original.ReadReceiptTo {
		t.Fatalf("To = %q, want %q", parsed.Header.Get("To"), original.ReadReceiptTo)
	}
	if !strings.HasPrefix(parsed.Header.Get("Content-Type"), "multipart/report") {
		t.Fatalf("Content-Type = %q", parsed.Header.Get("Content-Type"))
	}
	body := string(raw)
	for _, want := range []string{
		"Content-Type: message/disposition-notification",
		"Original-Message-ID: <original@example.com>",
		"Original-Recipient: rfc822; requester@example.com",
		"Disposition: manual-action/MDN-sent-manually; displayed",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("MDN missing %q", want)
		}
	}
}

func TestMDNAddressHelpers(t *testing.T) {
	if got := domainFromEmail("user@example.com"); got != "example.com" {
		t.Fatalf("domainFromEmail = %q", got)
	}
	if got := domainFromEmail("invalid"); got != "localhost" {
		t.Fatalf("invalid domainFromEmail = %q", got)
	}
	if got := extractEmailAddress(" Name <person@example.com> "); got != "person@example.com" {
		t.Fatalf("extractEmailAddress = %q", got)
	}
	if got := extractEmailAddress("plain@example.com"); got != "plain@example.com" {
		t.Fatalf("plain extractEmailAddress = %q", got)
	}
	if got := formatAddress("", "user@example.com"); got != "user@example.com" {
		t.Fatalf("formatAddress without name = %q", got)
	}
	if got := formatAddress("Simple Name", "user@example.com"); got != "Simple Name <user@example.com>" {
		t.Fatalf("simple formatAddress = %q", got)
	}
	if got := formatAddress(`Last, "First"`, "user@example.com"); got != `"Last, \"First\"" <user@example.com>` {
		t.Fatalf("quoted formatAddress = %q", got)
	}
}
