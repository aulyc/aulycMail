package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func NormalizeExistingDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid directory: %w", err)
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("backup directory is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("backup path is not a directory: %s", abs)
	}
	return abs, nil
}

func NormalizeTargetDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("backup directory is not set")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid backup directory: %w", err)
	}
	abs = filepath.Clean(abs)
	if err := os.MkdirAll(abs, 0700); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("backup directory is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("backup path is not a directory: %s", abs)
	}
	return abs, nil
}

func IndexPath(directory string) string {
	return filepath.Join(directory, ".aulycmail-backup", "index.json")
}

func FileExists(baseDir, relPath string) bool {
	if relPath == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(baseDir, filepath.FromSlash(relPath)))
	return err == nil && !info.IsDir()
}

func MessageRelativePath(accountEmail, folderPath, subject string, date time.Time, uidValidity, uid uint32) string {
	datePrefix := "unknown-date"
	if !date.IsZero() {
		datePrefix = date.UTC().Format("20060102-150405")
	}
	cleanSubject := SanitizePathSegment(subject, 80)
	if cleanSubject == "" {
		cleanSubject = "no-subject"
	}
	filename := fmt.Sprintf("%s_uv%d_uid%d_%s.eml", datePrefix, uidValidity, uid, cleanSubject)

	parts := []string{"eml", SanitizePathSegment(accountEmail, 80)}
	for _, segment := range SplitFolderPath(folderPath) {
		parts = append(parts, SanitizePathSegment(segment, 80))
	}
	parts = append(parts, filename)
	return filepath.ToSlash(filepath.Join(parts...))
}

func SplitFolderPath(path string) []string {
	path = strings.ReplaceAll(path, "\\", "/")
	raw := strings.Split(path, "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return []string{"folder"}
	}
	return parts
}

func WriteFileFromReader(baseDir, relPath string, reader io.Reader) (int64, error) {
	path := filepath.Join(baseDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return 0, fmt.Errorf("failed to create backup folder: %w", err)
	}

	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to write temp file: %w", err)
	}

	written, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return written, fmt.Errorf("failed to stream backup file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return written, fmt.Errorf("failed to close temp file: %w", closeErr)
	}
	if written == 0 {
		_ = os.Remove(tmp)
		return written, errors.New("message body not found")
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return written, fmt.Errorf("failed to replace file: %w", err)
	}
	return written, nil
}

func FileSizeInt(size int64) int {
	if size <= 0 {
		return 0
	}
	maxInt := int64(^uint(0) >> 1)
	if size > maxInt {
		return int(maxInt)
	}
	return int(size)
}

func WriteFileAtomic(path string, content []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, perm); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to replace file: %w", err)
	}
	return nil
}

func SanitizePathSegment(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			b.WriteRune('_')
		case unicode.IsControl(r):
			continue
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.Trim(out, ". ")
	if out == "" {
		return ""
	}
	runes := []rune(out)
	if len(runes) > maxRunes {
		out = string(runes[:maxRunes])
	}
	return out
}
