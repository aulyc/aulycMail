package smtp

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
)

type parsedMIMELeaf struct {
	contentType string
	disposition string
	contentID   string
	body        []byte
}

func parseMIMELeaves(t *testing.T, contentType, transferEncoding, disposition, contentID string, body io.Reader) []parsedMIMELeaf {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("ParseMediaType(%q): %v", contentType, err)
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		reader := multipart.NewReader(body, params["boundary"])
		var leaves []parsedMIMELeaf
		for {
			part, nextErr := reader.NextPart()
			if nextErr == io.EOF {
				break
			}
			if nextErr != nil {
				t.Fatalf("NextPart: %v", nextErr)
			}
			leaves = append(leaves, parseMIMELeaves(
				t,
				part.Header.Get("Content-Type"),
				part.Header.Get("Content-Transfer-Encoding"),
				part.Header.Get("Content-Disposition"),
				part.Header.Get("Content-ID"),
				part,
			)...)
		}
		return leaves
	}

	decoded := body
	switch strings.ToLower(transferEncoding) {
	case "base64":
		decoded = base64.NewDecoder(base64.StdEncoding, body)
	case "quoted-printable":
		decoded = quotedprintable.NewReader(body)
	}
	content, err := io.ReadAll(decoded)
	if err != nil {
		t.Fatalf("read %s MIME leaf: %v", mediaType, err)
	}
	return []parsedMIMELeaf{{contentType: mediaType, disposition: disposition, contentID: contentID, body: content}}
}

func TestToRFC822BuildsNestedMixedAlternativeAndRelatedMessage(t *testing.T) {
	regularContent := bytes.Repeat([]byte("attachment-data-"), 8)
	inlineContent := []byte("inline-image")
	replyTo := Address{Name: "Reply Desk", Address: "reply@example.com"}
	msg := &ComposeMessage{
		From:               Address{Name: "发件人", Address: "sender@example.com"},
		To:                 []Address{{Name: "Recipient", Address: "to@example.com"}},
		Cc:                 []Address{{Address: "cc@example.com"}},
		Bcc:                []Address{{Address: "hidden@example.com"}},
		ReplyTo:            &replyTo,
		Subject:            "测试主题",
		TextBody:           "Plain body",
		HTMLBody:           `<p>HTML body<img src="cid:logo"></p>`,
		InReplyTo:          "<parent@example.com>",
		References:         []string{"<root@example.com>", "<parent@example.com>"},
		RequestReadReceipt: true,
		Attachments: []Attachment{
			{Filename: "report.bin", ContentBase64: base64.StdEncoding.EncodeToString(regularContent)},
			{Filename: "logo.png", ContentID: "logo", Inline: true, Content: inlineContent},
		},
	}

	raw, err := msg.ToRFC822()
	if err != nil {
		t.Fatalf("ToRFC822: %v", err)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if parsed.Header.Get("Bcc") != "" {
		t.Fatalf("Bcc leaked into message headers: %q", parsed.Header.Get("Bcc"))
	}
	if parsed.Header.Get("In-Reply-To") != msg.InReplyTo || parsed.Header.Get("References") != strings.Join(msg.References, " ") {
		t.Fatalf("thread headers = (%q, %q)", parsed.Header.Get("In-Reply-To"), parsed.Header.Get("References"))
	}
	if parsed.Header.Get("Disposition-Notification-To") == "" || parsed.Header.Get("Reply-To") == "" {
		t.Fatalf("receipt/reply headers missing: %#v", parsed.Header)
	}

	leaves := parseMIMELeaves(t, parsed.Header.Get("Content-Type"), parsed.Header.Get("Content-Transfer-Encoding"), "", "", parsed.Body)
	if len(leaves) != 4 {
		t.Fatalf("MIME leaf count = %d, want text/html/inline/regular", len(leaves))
	}
	var sawText, sawHTML, sawInline, sawRegular bool
	for _, leaf := range leaves {
		switch {
		case leaf.contentType == "text/plain" && string(leaf.body) == msg.TextBody:
			sawText = true
		case leaf.contentType == "text/html" && string(leaf.body) == msg.HTMLBody:
			sawHTML = true
		case leaf.contentType == "image/png" && strings.HasPrefix(leaf.disposition, "inline") && leaf.contentID == "<logo>" && bytes.Equal(leaf.body, inlineContent):
			sawInline = true
		case leaf.contentType == "application/octet-stream" && strings.HasPrefix(leaf.disposition, "attachment") && bytes.Equal(leaf.body, regularContent):
			sawRegular = true
		}
	}
	if !sawText || !sawHTML || !sawInline || !sawRegular {
		t.Fatalf("decoded MIME leaves missing content: text=%v html=%v inline=%v regular=%v", sawText, sawHTML, sawInline, sawRegular)
	}
}

func TestToRFC822PreservesAttachmentWhenBodyIsEmpty(t *testing.T) {
	msg := &ComposeMessage{
		From:    Address{Address: "sender@example.com"},
		To:      []Address{{Address: "recipient@example.com"}},
		Subject: "Attachment only",
		Attachments: []Attachment{{
			Filename:    "only.txt",
			ContentType: "text/plain",
			Content:     []byte("attachment without a body"),
		}, {
			Filename:  "inline.png",
			ContentID: "inline-only",
			Inline:    true,
			Content:   []byte("inline attachment without HTML"),
		}},
	}
	raw, err := msg.ToRFC822()
	if err != nil {
		t.Fatalf("ToRFC822: %v", err)
	}
	if !strings.Contains(string(raw), "multipart/mixed") ||
		!strings.Contains(string(raw), `filename="only.txt"`) ||
		!strings.Contains(string(raw), `filename="inline.png"`) {
		t.Fatalf("attachment-only message dropped its MIME attachment:\n%s", raw)
	}
}

func TestAttachmentResolveContentAndInvalidBase64(t *testing.T) {
	direct := &Attachment{Content: []byte("direct"), ContentBase64: base64.StdEncoding.EncodeToString([]byte("ignored"))}
	if got, err := direct.ResolveContent(); err != nil || string(got) != "direct" {
		t.Fatalf("direct ResolveContent = (%q, %v)", got, err)
	}
	encoded := &Attachment{ContentBase64: base64.StdEncoding.EncodeToString([]byte("decoded"))}
	if got, err := encoded.ResolveContent(); err != nil || string(got) != "decoded" {
		t.Fatalf("base64 ResolveContent = (%q, %v)", got, err)
	}
	if got, err := (&Attachment{}).ResolveContent(); err != nil || got != nil {
		t.Fatalf("empty ResolveContent = (%q, %v), want nil/nil", got, err)
	}

	invalid := &ComposeMessage{
		From:     Address{Address: "sender@example.com"},
		To:       []Address{{Address: "recipient@example.com"}},
		TextBody: "body",
		Attachments: []Attachment{{
			Filename:      "broken.bin",
			ContentBase64: "not-base64",
		}},
	}
	if _, err := invalid.ToRFC822(); err == nil || !strings.Contains(err.Error(), "failed to resolve attachment content") {
		t.Fatalf("invalid attachment error = %v", err)
	}
}

func TestToRFC822SanitizesHeaderNewlines(t *testing.T) {
	msg := &ComposeMessage{
		From:     Address{Address: "sender@example.com"},
		To:       []Address{{Address: "recipient@example.com"}},
		Subject:  "safe\r\nX-Injected: yes",
		HTMLBody: "<p>body</p>",
	}
	raw, err := msg.ToRFC822()
	if err != nil {
		t.Fatalf("ToRFC822: %v", err)
	}
	if strings.Contains(string(raw), "\r\nX-Injected:") {
		t.Fatalf("header injection survived sanitization:\n%s", raw)
	}
}
