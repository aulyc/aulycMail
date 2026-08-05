package sync

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/message"
	gomessage "github.com/emersion/go-message"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestParseMessageBodyFullComplexMultipart(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)
	raw := mimeMessage(
		"From: sender@example.com",
		"To: receiver@example.com",
		"Subject: MIME coverage",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="outer"`,
		"",
		"--outer",
		`Content-Type: multipart/alternative; boundary="alternative"`,
		"",
		"--alternative",
		`Content-Type: text/plain; charset="utf-8"`,
		"",
		"plain body",
		"--alternative",
		`Content-Type: text/html; charset="utf-8"`,
		"",
		`<p>html body<img src="cid:logo"></p>`,
		"--alternative--",
		"--outer",
		`Content-Type: image/png; name="logo.png"`,
		`Content-Disposition: inline; filename="logo.png"`,
		"Content-ID: <logo>",
		"Content-Transfer-Encoding: base64",
		"",
		"aW1hZ2UtYnl0ZXM=",
		"--outer",
		`Content-Type: application/pdf; name="report.pdf"`,
		`Content-Disposition: attachment; filename="report.pdf"`,
		"Content-Transfer-Encoding: base64",
		"",
		"cGRmLWJ5dGVz",
		"--outer",
		"Content-Type: application/pgp-signature",
		"",
		"signature bytes",
		"--outer--",
		"",
	)

	parsed := engine.parseMessageBodyFull(raw, "message-1", time.Second)
	if parsed.BodyText != "plain body" {
		t.Fatalf("BodyText = %q, want plain body", parsed.BodyText)
	}
	if parsed.BodyHTML != `<p>html body<img src="cid:logo"></p>` {
		t.Fatalf("BodyHTML = %q", parsed.BodyHTML)
	}
	if !parsed.HasAttachments || len(parsed.Attachments) != 2 {
		t.Fatalf("attachments = %d, hasAttachments = %v; want 2, true", len(parsed.Attachments), parsed.HasAttachments)
	}

	attachments := make(map[string]struct {
		contentType string
		contentID   string
		inline      bool
		content     string
		size        int
	})
	for _, attachment := range parsed.Attachments {
		attachments[attachment.Filename] = struct {
			contentType string
			contentID   string
			inline      bool
			content     string
			size        int
		}{attachment.ContentType, attachment.ContentID, attachment.IsInline, string(attachment.Content), attachment.Size}
		if attachment.MessageID != "message-1" || attachment.ID == "" {
			t.Errorf("attachment identity not populated: %#v", attachment)
		}
	}

	logo, ok := attachments["logo.png"]
	if !ok || logo.contentType != "image/png" || logo.contentID != "logo" || !logo.inline || logo.content != "image-bytes" || logo.size != len("image-bytes") {
		t.Errorf("inline attachment = %#v", logo)
	}
	report, ok := attachments["report.pdf"]
	if !ok || report.contentType != "application/pdf" || report.inline || report.content != "" || report.size != len("pdf-bytes") {
		t.Errorf("file attachment = %#v", report)
	}
	if _, ok := attachments["attachment.bin"]; ok {
		t.Fatal("signature part must not be exposed as an attachment")
	}
}

func TestParseMessageBodyFullSinglePartVariants(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)

	t.Run("html charset from meta", func(t *testing.T) {
		html := `<html><head><meta charset="gbk"></head><body>中文正文</body></html>`
		encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(html))
		if err != nil {
			t.Fatal(err)
		}
		raw := append([]byte("MIME-Version: 1.0\r\nContent-Type: text/html\r\n\r\n"), encoded...)

		parsed := engine.parseMessageBodyFull(raw, "message-html", time.Second)
		if parsed.BodyHTML != html || parsed.BodyText != "" || parsed.HasAttachments {
			t.Fatalf("parsed HTML = %#v", parsed)
		}
	})

	t.Run("whole message attachment", func(t *testing.T) {
		raw := mimeMessage(
			"MIME-Version: 1.0",
			`Content-Type: application/zip; name="report.zip"`,
			`Content-Disposition: attachment; filename="report.zip"`,
			"Content-Transfer-Encoding: base64",
			"",
			"emlwLWJ5dGVz",
		)

		parsed := engine.parseMessageBodyFull(raw, "message-zip", time.Second)
		if !parsed.HasAttachments || len(parsed.Attachments) != 1 {
			t.Fatalf("parsed attachment = %#v", parsed)
		}
		attachment := parsed.Attachments[0]
		if attachment.Filename != "report.zip" || attachment.ContentType != "application/zip" || attachment.Size != len("zip-bytes") || len(attachment.Content) != 0 {
			t.Fatalf("attachment = %#v", attachment)
		}
	})

	t.Run("unsafe transfer encoding", func(t *testing.T) {
		raw := mimeMessage(
			"MIME-Version: 1.0",
			"Content-Type: text/html; charset=utf-8",
			"Content-Transfer-Encoding: x-dangerous",
			"",
			"<script>unsafe()</script>",
		)

		parsed := engine.parseMessageBodyFull(raw, "message-unsafe", time.Second)
		if !parsed.UnsafeContent || parsed.BodyHTML != "" || !strings.Contains(parsed.BodyText, "non-standard encoding") {
			t.Fatalf("unsafe result = %#v", parsed)
		}
	})

	t.Run("plain text default", func(t *testing.T) {
		parsed := engine.parseMessageBodyFull(mimeMessage("Subject: plain", "", "hello"), "message-plain", time.Second)
		if parsed.BodyText != "hello" || parsed.BodyHTML != "" {
			t.Fatalf("plain result = %#v", parsed)
		}
	})
}

func TestParseMessageBodyLegacyMultipartAndMalformedInput(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)
	raw := mimeMessage(
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="outer"`,
		"",
		"--outer",
		`Content-Type: multipart/alternative; boundary="inner"`,
		"",
		"--inner",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"legacy text",
		"--inner",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>legacy html</p>",
		"--inner--",
		"--outer",
		"Content-Type: application/octet-stream",
		`Content-Disposition: attachment; filename="data.bin"`,
		"",
		"data",
		"--outer--",
		"",
	)

	textBody, htmlBody, hasAttachments := engine.parseMessageBody(raw)
	if textBody != "legacy text" || htmlBody != "<p>legacy html</p>" || !hasAttachments {
		t.Fatalf("legacy result = %q, %q, %v", textBody, htmlBody, hasAttachments)
	}

	malformed := []byte("Bad Header\r\n\r\nraw body")
	textBody, htmlBody, hasAttachments = engine.parseMessageBody(malformed)
	if textBody != string(malformed) || htmlBody != "" || hasAttachments {
		t.Fatalf("malformed fallback = %q, %q, %v", textBody, htmlBody, hasAttachments)
	}
}

func TestMessageBodyParsingCancellationAndLimits(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := engine.parseMessageBodyInternal(canceled, mimeMessage("", "body"), "message-canceled"); !errors.Is(err, context.Canceled) {
		t.Fatalf("parse canceled error = %v", err)
	}
	if _, err := readLimitedPart(canceled, strings.NewReader("body"), 4); !errors.Is(err, context.Canceled) {
		t.Fatalf("limited read canceled error = %v", err)
	}
	got, err := readLimitedPart(context.Background(), strings.NewReader("abcdef"), 3)
	if err != nil || string(got) != "abc" {
		t.Fatalf("limited read = %q, %v", got, err)
	}

	reader := strings.NewReader("unchanged")
	if newContextReader(nil, reader) != reader { //nolint:staticcheck // nil intentionally exercises the helper's compatibility branch.
		t.Fatal("nil context should preserve the original reader")
	}

	ctx, cancelDeadline := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancelDeadline()
	time.Sleep(time.Millisecond)
	buf := make([]byte, 1)
	if _, err := newContextReader(ctx, bytes.NewReader([]byte("x"))).Read(buf); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("context reader error = %v", err)
	}
}

func TestSignatureClassificationAndFallbackExtraction(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)
	for _, contentType := range []string{"application/pkcs7-signature", "application/x-pkcs7-signature", "application/pgp-signature"} {
		if !isSignaturePart(contentType) {
			t.Errorf("%q should be classified as a signature", contentType)
		}
	}
	if isSignaturePart("application/pdf") {
		t.Fatal("application/pdf must not be classified as a signature")
	}

	if got := engine.extractPlainTextFallback([]byte("Header: value\r\n\r\nhello\x00 world\n")); got != "hello world" {
		t.Fatalf("CRLF fallback = %q", got)
	}
	if got := engine.extractPlainTextFallback([]byte("Header: value\n\nLF body")); got != "LF body" {
		t.Fatalf("LF fallback = %q", got)
	}
	if got := engine.extractPlainTextFallback([]byte("no separator")); got != "" {
		t.Fatalf("missing separator fallback = %q", got)
	}

	large := []byte("Header: value\r\n\r\n" + strings.Repeat("a", 11*1024))
	got := engine.extractPlainTextFallback(large)
	if !strings.HasSuffix(got, "... [truncated - parsing timed out]") || !strings.HasPrefix(got, strings.Repeat("a", 100)) {
		t.Fatalf("large fallback was not predictably truncated: len=%d suffix=%q", len(got), got[len(got)-40:])
	}
}

func TestMessageBodyParsersCoverImplicitInlineAndGeneratedAttachments(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)
	raw := mimeMessage(
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="edge"`,
		"",
		"--edge",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"first plain body",
		"--edge",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"ignored second plain body",
		"--edge",
		"Content-Type: image/jpeg",
		"Content-ID: <generated-image>",
		"Content-Transfer-Encoding: base64",
		"",
		"aW1hZ2U=",
		"--edge",
		"Content-Type: application/json",
		"Content-Disposition: inline",
		"",
		`{"ok":true}`,
		"--edge",
		"Content-Type: application/octet-stream",
		"Content-Disposition: attachment",
		"Content-ID: <download-inline>",
		"",
		"binary",
		"--edge--",
		"",
	)
	parsed := engine.parseMessageBodyFull(raw, "edge-message", time.Second)
	if parsed.BodyText != "first plain body" || !parsed.HasAttachments || len(parsed.Attachments) != 3 {
		t.Fatalf("implicit attachments = %#v", parsed)
	}
	byName := make(map[string]*message.Attachment, len(parsed.Attachments))
	for _, attachment := range parsed.Attachments {
		byName[attachment.Filename] = attachment
	}
	if got := byName["attachment.jpeg"]; got == nil || !got.IsInline || string(got.Content) != "image" {
		t.Fatalf("generated inline image = %#v", got)
	}
	if got := byName["attachment.bin"]; got == nil {
		t.Fatalf("generated binary attachments = %#v", parsed.Attachments)
	}

	unsafeMultipart := mimeMessage(
		"MIME-Version: 1.0",
		`Content-Type: multipart/alternative; boundary="unsafe"`,
		"",
		"--unsafe",
		"Content-Type: text/plain",
		"Content-Transfer-Encoding: x-spam",
		"",
		"unsafe body",
		"--unsafe--",
	)
	unsafe := engine.parseMessageBodyFull(unsafeMultipart, "unsafe-message", time.Second)
	if !unsafe.UnsafeContent || !strings.Contains(unsafe.BodyText, "non-standard encoding") {
		t.Fatalf("unsafe multipart = %#v", unsafe)
	}

	malformed := engine.parseMessageBodyFull([]byte("Bad Header\r\n\r\nfallback body"), "malformed", time.Second)
	if !strings.Contains(malformed.BodyText, "fallback body") || malformed.HasAttachments {
		t.Fatalf("malformed full parser fallback = %#v", malformed)
	}
}

func TestLegacyParserCoversLongHTMLImplicitPartsAndNestedNil(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil, nil, nil)
	longHTML := `<html><head><meta charset="utf-8"></head><body>` + strings.Repeat("x", 240) + `</body></html>`
	raw := mimeMessage(
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="legacy-edge"`,
		"",
		"--legacy-edge",
		"Content-Type: text/html",
		"",
		longHTML,
		"--legacy-edge",
		"Content-Type: image/png",
		"Content-Disposition: inline",
		"",
		"image",
		"--legacy-edge",
		"Content-Type: application/json",
		"",
		`{"legacy":true}`,
		"--legacy-edge--",
		"",
	)
	textBody, htmlBody, hasAttachments := engine.parseMessageBody(raw)
	if textBody != "" || htmlBody != longHTML || !hasAttachments {
		t.Fatalf("legacy implicit parser = %q, len(html)=%d, attachments=%v", textBody, len(htmlBody), hasAttachments)
	}

	single, err := gomessage.Read(strings.NewReader("Content-Type: text/plain\r\n\r\nbody"))
	if err != nil {
		t.Fatal(err)
	}
	if textBody, htmlBody, hasAttachments := engine.parseNestedMultipart(single); textBody != "" || htmlBody != "" || hasAttachments {
		t.Fatalf("non-multipart nested parser = %q, %q, %v", textBody, htmlBody, hasAttachments)
	}

	legacySingleHTML := mimeMessage("Content-Type: text/html", "", longHTML)
	textBody, htmlBody, hasAttachments = engine.parseMessageBody(legacySingleHTML)
	if textBody != "" || htmlBody != longHTML || hasAttachments {
		t.Fatalf("legacy single HTML = %q, len(html)=%d, %v", textBody, len(htmlBody), hasAttachments)
	}
}

func mimeMessage(lines ...string) []byte {
	return []byte(strings.Join(lines, "\r\n"))
}
