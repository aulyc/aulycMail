package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/email"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/platform"
	"github.com/rs/zerolog"
)

func TestValidateOpenPathAllowsOnlyApplicationAndDownloadRoots(t *testing.T) {
	dataDir := t.TempDir()
	a := &App{paths: &platform.Paths{Data: dataDir, Config: dataDir, Cache: filepath.Join(dataDir, "cache")}}

	for _, allowed := range []string{
		dataDir,
		filepath.Join(dataDir, "message-sources", "source.eml"),
		filepath.Join(a.paths.AttachmentsPath(), "message-1", "report.pdf"),
	} {
		if err := a.validateOpenPath(allowed); err != nil {
			t.Fatalf("validateOpenPath(%q) error = %v", allowed, err)
		}
	}
	outside := filepath.Join(filepath.Dir(dataDir), filepath.Base(dataDir)+"-outside", "secret.txt")
	if err := a.validateOpenPath(outside); err == nil {
		t.Fatalf("validateOpenPath(%q) error = nil, want outside-root rejection", outside)
	}
}

func TestCleanupOldMessageSourceFilesRemovesOnlyExpiredFiles(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.eml")
	freshPath := filepath.Join(dir, "fresh.eml")
	nestedDir := filepath.Join(dir, "nested")
	if err := os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatalf("write old file: %v", err)
	}
	if err := os.WriteFile(freshPath, []byte("fresh"), 0600); err != nil {
		t.Fatalf("write fresh file: %v", err)
	}
	if err := os.Mkdir(nestedDir, 0700); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("age old file: %v", err)
	}

	cleanupOldMessageSourceFiles(dir, 24*time.Hour)
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old source still exists or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("fresh source was removed: %v", err)
	}
	if info, err := os.Stat(nestedDir); err != nil || !info.IsDir() {
		t.Fatalf("nested directory = %v, %v; want preserved", info, err)
	}
	cleanupOldMessageSourceFiles(filepath.Join(dir, "missing"), time.Hour)
}

func TestSaveAttachmentTargetsFromRawFileReportsPartialSuccess(t *testing.T) {
	dir := t.TempDir()
	rawPath := filepath.Join(dir, "message.eml")
	raw := strings.Join([]string{
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="boundary"`,
		"",
		"--boundary",
		"Content-Type: text/plain",
		"",
		"body",
		"--boundary",
		`Content-Type: text/plain; name="report.txt"`,
		`Content-Disposition: attachment; filename="report.txt"`,
		"",
		"hello",
		"--boundary--",
		"",
	}, "\r\n")
	if err := os.WriteFile(rawPath, []byte(raw), 0600); err != nil {
		t.Fatalf("write raw message: %v", err)
	}
	reportPath := filepath.Join(dir, "saved-report.txt")
	missingPath := filepath.Join(dir, "missing.txt")
	targets := []email.AttachmentSaveTarget{
		{Attachment: &message.Attachment{Filename: "report.txt"}, CustomPath: reportPath},
		{Attachment: &message.Attachment{Filename: "not-present.txt"}, CustomPath: missingPath},
	}

	count, err := saveAttachmentTargetsFromRawFile(email.NewAttachmentDownloader(filepath.Join(dir, "downloads")), rawPath, targets, zerolog.Nop())
	if err != nil {
		t.Fatalf("saveAttachmentTargetsFromRawFile() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("saved count = %d, want 1", count)
	}
	content, err := os.ReadFile(reportPath)
	if err != nil || strings.TrimSpace(string(content)) != "hello" {
		t.Fatalf("saved report = %q, %v", content, err)
	}
	if _, err := os.Stat(missingPath); !os.IsNotExist(err) {
		t.Fatalf("missing attachment unexpectedly created: %v", err)
	}
	if _, err := saveAttachmentTargetsFromRawFile(email.NewAttachmentDownloader(dir), filepath.Join(dir, "absent.eml"), nil, zerolog.Nop()); err == nil {
		t.Fatal("missing raw message error = nil")
	}
}

func TestBackupViewerAttachmentFallbackName(t *testing.T) {
	for _, test := range []struct {
		contentType string
		want        string
	}{
		{contentType: "image/png", want: "attachment.png"},
		{contentType: "image/", want: "attachment.bin"},
		{contentType: "application/pdf", want: "attachment.bin"},
		{contentType: "", want: "attachment.bin"},
	} {
		if got := backupViewerAttachmentFallbackName(test.contentType); got != test.want {
			t.Fatalf("fallback name for %q = %q, want %q", test.contentType, got, test.want)
		}
	}
}
