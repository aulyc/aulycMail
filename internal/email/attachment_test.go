package email

import (
	"bytes"
	"strings"
	"testing"
)

func TestAttachmentExtractorExtractsNestedAndInlineParts(t *testing.T) {
	raw := strings.Join([]string{
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="outer"`,
		"",
		"--outer",
		"Content-Type: text/plain",
		"",
		"message body",
		"--outer",
		`Content-Type: application/octet-stream; name="=?UTF-8?Q?report=5Ffinal.txt?="`,
		`Content-Disposition: attachment; filename="=?UTF-8?Q?report=5Ffinal.txt?="`,
		"Content-Transfer-Encoding: base64",
		"",
		"aGVsbG8=",
		"--outer",
		`Content-Type: multipart/related; boundary="inner"`,
		"",
		"--inner",
		"Content-Type: image/png",
		"Content-Disposition: inline",
		"Content-ID: <logo-1>",
		"",
		"PNG",
		"--inner--",
		"--outer--",
		"",
	}, "\r\n")

	attachments, err := NewAttachmentExtractor().ExtractAttachments("message-1", []byte(raw))
	if err != nil {
		t.Fatalf("ExtractAttachments() error = %v", err)
	}
	if len(attachments) != 2 {
		t.Fatalf("ExtractAttachments() returned %d attachments, want 2: %#v", len(attachments), attachments)
	}

	file := attachments[0]
	if file.Attachment.ID == "" || file.Attachment.MessageID != "message-1" {
		t.Fatalf("file identity = %+v", file.Attachment)
	}
	if file.Attachment.Filename != "report_final.txt" {
		t.Fatalf("file name = %q, want report_final.txt", file.Attachment.Filename)
	}
	if string(file.Content) != "hello" || file.Attachment.Size != 5 || file.Attachment.IsInline {
		t.Fatalf("file payload = %q metadata = %+v", file.Content, file.Attachment)
	}

	inline := attachments[1]
	if inline.Attachment.Filename != "attachment.png" || inline.Attachment.ContentID != "logo-1" || !inline.Attachment.IsInline {
		t.Fatalf("inline metadata = %+v", inline.Attachment)
	}
	if string(inline.Content) != "PNG" {
		t.Fatalf("inline content = %q, want PNG", inline.Content)
	}
}

func TestAttachmentExtractorHandlesNonMultipartAndMalformedInput(t *testing.T) {
	attachments, err := NewAttachmentExtractor().ExtractAttachments("message-1", []byte("Subject: plain\r\n\r\nbody"))
	if err != nil {
		t.Fatalf("plain message error = %v", err)
	}
	if len(attachments) != 0 {
		t.Fatalf("plain message attachments = %#v, want none", attachments)
	}

	if _, err := NewAttachmentExtractor().ExtractAttachments("message-1", []byte("malformed header\r\n\r\nbody")); err == nil {
		t.Fatal("malformed message error = nil, want parse error")
	}
}

func TestAttachmentDecodersCoverSupportedAndFallbackEncodings(t *testing.T) {
	if got := string(decodeContent([]byte("aGVsbG8="), "base64")); got != "hello" {
		t.Fatalf("base64 decode = %q, want hello", got)
	}
	invalid := []byte("not base64")
	if got := decodeContent(invalid, "base64"); !bytes.Equal(got, invalid) {
		t.Fatalf("invalid base64 decode = %q, want original", got)
	}
	if got := string(decodeContent([]byte("hello=20world"), "quoted-printable")); got != "hello world" {
		t.Fatalf("quoted-printable decode = %q, want hello world", got)
	}
	plain := []byte("unchanged")
	if got := decodeContent(plain, "binary"); !bytes.Equal(got, plain) {
		t.Fatalf("binary decode = %q, want original", got)
	}

	decoded, err := decodeRFC2047("=?UTF-8?B?5rWL6K+VLnR4dA==?=")
	if err != nil || decoded != "测试.txt" {
		t.Fatalf("decodeRFC2047() = %q, %v; want 测试.txt", decoded, err)
	}

	if got := NewAttachmentExtractor().extractFromTNEF("message-1", strings.NewReader("not tnef")); len(got) != 0 {
		t.Fatalf("invalid TNEF attachments = %#v, want none", got)
	}
}
