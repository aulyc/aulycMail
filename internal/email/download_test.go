package email

import (
	"os"
	"path/filepath"
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
