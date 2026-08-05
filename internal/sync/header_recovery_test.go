package sync

import (
	"encoding/json"
	"net/mail"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/message"
)

func TestParseHeadersIntoMessage(t *testing.T) {
	header := mimeMessage(
		"Subject: =?UTF-8?B?5Lit5paH5Li76aKY?=",
		"Message-ID: <message@example.com>",
		"In-Reply-To: <parent@example.com>",
		"Date: Tue, 29 Jul 2025 12:34:56 +0800",
		"From: =?UTF-8?B?5byg5LiJ?= <sender@example.com>",
		"To: Receiver <receiver@example.com>, second@example.com",
		"Cc: Copy <copy@example.com>",
		"Reply-To: replies@example.com",
		"",
	)
	m := &message.Message{}
	if err := parseHeadersIntoMessage(m, header); err != nil {
		t.Fatal(err)
	}

	wantDate := time.Date(2025, time.July, 29, 4, 34, 56, 0, time.UTC)
	if m.Subject != "中文主题" || m.MessageID != "message@example.com" || m.InReplyTo != "parent@example.com" || !m.Date.Equal(wantDate) {
		t.Fatalf("basic headers = %#v", m)
	}
	if m.FromName != "张三" || m.FromEmail != "sender@example.com" || m.ReplyTo != "replies@example.com" {
		t.Fatalf("address headers = %#v", m)
	}
	assertAddressJSON(t, m.ToList, []addressJSON{{Name: "Receiver", Email: "receiver@example.com"}, {Email: "second@example.com"}})
	assertAddressJSON(t, m.CcList, []addressJSON{{Name: "Copy", Email: "copy@example.com"}})
}

func TestParseHeadersIntoMessageToleratesEmptyAndInvalidFields(t *testing.T) {
	m := &message.Message{Subject: "unchanged"}
	if err := parseHeadersIntoMessage(m, nil); err != nil || m.Subject != "unchanged" {
		t.Fatalf("empty header changed message: %#v, %v", m, err)
	}

	header := mimeMessage(
		"Subject: valid",
		"Date: not-a-date",
		"From: not an address <>",
		"To: invalid <>",
		"Cc: invalid <>",
		"Reply-To: invalid <>",
		"",
	)
	m = &message.Message{}
	if err := parseHeadersIntoMessage(m, header); err != nil {
		t.Fatal(err)
	}
	if m.Subject != "valid" || !m.Date.IsZero() || m.FromEmail != "" || m.ToList != "" || m.CcList != "" || m.ReplyTo != "" {
		t.Fatalf("invalid optional fields should be ignored: %#v", m)
	}

	if err := parseHeadersIntoMessage(&message.Message{}, []byte("Bad Header\r\n\r\n")); err == nil {
		t.Fatal("malformed RFC 5322 header should return an error")
	}
}

func TestMailAddressListToJSON(t *testing.T) {
	addresses := []*mail.Address{
		{Name: "=?UTF-8?B?5byg5LiJ?=", Address: "one@example.com"},
		{Name: "Second", Address: "two@example.com"},
	}
	assertAddressJSON(t, mailAddressListToJSON(addresses), []addressJSON{
		{Name: "张三", Email: "one@example.com"},
		{Name: "Second", Email: "two@example.com"},
	})
	if got := mailAddressListToJSON(nil); got != "[]" {
		t.Fatalf("nil address list = %q, want []", got)
	}
}

type addressJSON struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func assertAddressJSON(t *testing.T, raw string, want []addressJSON) {
	t.Helper()
	var got []addressJSON
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("invalid address JSON %q: %v", raw, err)
	}
	if len(got) != len(want) {
		t.Fatalf("addresses = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("address %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}
