package email

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aulyc/aulycmail/internal/message"
)

func TestSafeAttachmentFilenameStripsPathComponents(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "invoice.pdf", "invoice.pdf"},
		{"unix traversal", "../../evil.txt", "evil.txt"},
		{"windows traversal", `..\evil.txt`, "evil.txt"},
		{"absolute", "/tmp/evil.txt", "evil.txt"},
		{"control chars", "bad\x00\nname.txt", "badname.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeAttachmentFilename(tt.input)
			if err != nil {
				t.Fatalf("SafeAttachmentFilename() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("SafeAttachmentFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSafeAttachmentFilenameRejectsEmptyBase(t *testing.T) {
	for _, input := range []string{"", ".", "..", "../../"} {
		t.Run(input, func(t *testing.T) {
			if _, err := SafeAttachmentFilename(input); err == nil {
				t.Fatalf("SafeAttachmentFilename(%q) returned nil error", input)
			}
		})
	}
}

func TestUniqueAttachmentPathAvoidsCollisions(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "invoice.pdf")
	if err := os.WriteFile(existing, []byte("old"), 0600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	got, err := UniqueAttachmentPath(dir, "invoice.pdf")
	if err != nil {
		t.Fatalf("UniqueAttachmentPath() error = %v", err)
	}
	want := filepath.Join(dir, "invoice_1.pdf")
	if got != want {
		t.Fatalf("UniqueAttachmentPath() = %q, want %q", got, want)
	}
}

func TestSaveAttachmentDefaultPathSanitizesFilename(t *testing.T) {
	downloader := NewAttachmentDownloader(t.TempDir())
	att := &message.Attachment{
		MessageID: "msg",
		Filename:  "../../evil.txt",
	}

	path, err := downloader.SaveAttachment(att, []byte("content"), "")
	if err != nil {
		t.Fatalf("SaveAttachment() error = %v", err)
	}
	if filepath.Base(path) != "evil.txt" {
		t.Fatalf("saved basename = %q, want evil.txt", filepath.Base(path))
	}
	if filepath.Base(filepath.Dir(path)) != "msg" {
		t.Fatalf("message directory = %q, want msg", filepath.Base(filepath.Dir(path)))
	}
}

func TestExtractAttachmentContentToWriterDecodesAttachment(t *testing.T) {
	raw := strings.Join([]string{
		"From: sender@example.com",
		"To: user@example.com",
		"Subject: attachment",
		"Content-Type: multipart/mixed; boundary=abc",
		"",
		"--abc",
		"Content-Type: text/plain",
		"",
		"hello",
		"--abc",
		"Content-Type: text/plain; name=\"note.txt\"",
		"Content-Disposition: attachment; filename=\"note.txt\"",
		"Content-Transfer-Encoding: base64",
		"",
		"aGVsbG8gYXR0YWNobWVudA==",
		"--abc--",
		"",
	}, "\r\n")

	downloader := NewAttachmentDownloader(t.TempDir())
	var out bytes.Buffer
	n, err := downloader.ExtractAttachmentContentToWriter(strings.NewReader(raw), "note.txt", &out)
	if err != nil {
		t.Fatalf("ExtractAttachmentContentToWriter() error = %v", err)
	}
	if got := out.String(); got != "hello attachment" {
		t.Fatalf("decoded content = %q, want %q", got, "hello attachment")
	}
	if n != int64(out.Len()) {
		t.Fatalf("written = %d, want %d", n, out.Len())
	}
}

func TestSaveAttachmentFromRawReaderWritesDecodedFile(t *testing.T) {
	raw := strings.Join([]string{
		"From: sender@example.com",
		"To: user@example.com",
		"Subject: attachment",
		"Content-Type: multipart/mixed; boundary=abc",
		"",
		"--abc",
		"Content-Type: text/plain",
		"",
		"hello",
		"--abc",
		"Content-Type: text/plain; name=\"note.txt\"",
		"Content-Disposition: attachment; filename=\"note.txt\"",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"hello=20file",
		"--abc--",
		"",
	}, "\r\n")

	downloader := NewAttachmentDownloader(t.TempDir())
	att := &message.Attachment{
		MessageID: "msg-123456789",
		Filename:  "note.txt",
	}

	path, n, err := downloader.SaveAttachmentFromRawReader(strings.NewReader(raw), att, "")
	if err != nil {
		t.Fatalf("SaveAttachmentFromRawReader() error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if string(content) != "hello file" {
		t.Fatalf("saved content = %q, want %q", string(content), "hello file")
	}
	if n != int64(len(content)) {
		t.Fatalf("written = %d, want %d", n, len(content))
	}
	if filepath.Base(filepath.Dir(path)) != "msg-1234" {
		t.Fatalf("message directory = %q, want msg-1234", filepath.Base(filepath.Dir(path)))
	}
}

func TestSaveAttachmentsFromRawReaderWritesMultipleFilesInOnePass(t *testing.T) {
	raw := strings.Join([]string{
		"From: sender@example.com",
		"To: user@example.com",
		"Subject: attachments",
		"Content-Type: multipart/mixed; boundary=abc",
		"",
		"--abc",
		"Content-Type: text/plain",
		"",
		"hello",
		"--abc",
		"Content-Type: text/plain; name=\"one.txt\"",
		"Content-Disposition: attachment; filename=\"one.txt\"",
		"Content-Transfer-Encoding: base64",
		"",
		"b25l",
		"--abc",
		"Content-Type: text/plain; name=\"two.txt\"",
		"Content-Disposition: attachment; filename=\"two.txt\"",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"tw=6f",
		"--abc--",
		"",
	}, "\r\n")

	dir := t.TempDir()
	downloader := NewAttachmentDownloader(t.TempDir())
	results, err := downloader.SaveAttachmentsFromRawReader(strings.NewReader(raw), []AttachmentSaveTarget{
		{
			Attachment: &message.Attachment{MessageID: "msg", Filename: "one.txt"},
			CustomPath: filepath.Join(dir, "one.txt"),
		},
		{
			Attachment: &message.Attachment{MessageID: "msg", Filename: "two.txt"},
			CustomPath: filepath.Join(dir, "two.txt"),
		},
	})
	if err != nil {
		t.Fatalf("SaveAttachmentsFromRawReader() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2", len(results))
	}
	for _, result := range results {
		if result.Err != nil {
			t.Fatalf("result for %s error = %v", result.Attachment.Filename, result.Err)
		}
	}

	assertFileContent := func(name, want string) {
		t.Helper()
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s content = %q, want %q", name, string(got), want)
		}
	}

	assertFileContent("one.txt", "one")
	assertFileContent("two.txt", "two")
}

func TestExtractAttachmentsContentFromRawReaderReadsMultipleAttachmentsInOnePass(t *testing.T) {
	raw := strings.Join([]string{
		"From: sender@example.com",
		"To: user@example.com",
		"Subject: attachments",
		"Content-Type: multipart/mixed; boundary=abc",
		"",
		"--abc",
		"Content-Type: text/plain",
		"",
		"hello",
		"--abc",
		"Content-Type: text/plain; name=\"one.txt\"",
		"Content-Disposition: attachment; filename=\"one.txt\"",
		"Content-Transfer-Encoding: base64",
		"",
		"b25l",
		"--abc",
		"Content-Type: text/plain; name=\"two.txt\"",
		"Content-Disposition: attachment; filename=\"two.txt\"",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"tw=6f",
		"--abc--",
		"",
	}, "\r\n")

	downloader := NewAttachmentDownloader(t.TempDir())
	results, err := downloader.ExtractAttachmentsContentFromRawReader(strings.NewReader(raw), []string{"one.txt", "two.txt"})
	if err != nil {
		t.Fatalf("ExtractAttachmentsContentFromRawReader() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("result[0] error = %v", results[0].Err)
	}
	if results[1].Err != nil {
		t.Fatalf("result[1] error = %v", results[1].Err)
	}
	if string(results[0].Content) != "one" {
		t.Fatalf("first attachment = %q, want one", string(results[0].Content))
	}
	if string(results[1].Content) != "two" {
		t.Fatalf("second attachment = %q, want two", string(results[1].Content))
	}
}

func TestCopyAttachmentContentRejectsOverLimit(t *testing.T) {
	var buf bytes.Buffer
	written, err := copyAttachmentContent(&buf, strings.NewReader("abcdef"), 3)
	if err == nil {
		t.Fatal("expected over-limit attachment content to fail")
	}
	var tooLarge AttachmentContentTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("expected AttachmentContentTooLargeError, got %T %v", err, err)
	}
	if written != 3 {
		t.Fatalf("written = %d, want 3", written)
	}
	if buf.String() != "abc" {
		t.Fatalf("buffer = %q, want abc", buf.String())
	}
}
