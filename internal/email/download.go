// Package email provides email content processing utilities
package email

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/aulyc/aulycmail/internal/message"
	gomessage "github.com/emersion/go-message"
	msgcharset "github.com/emersion/go-message/charset"
	"golang.org/x/text/encoding/htmlindex"
)

// AttachmentDownloader handles downloading and saving attachments
type AttachmentDownloader struct {
	attachmentsDir string
}

// NewAttachmentDownloader creates a new attachment downloader
func NewAttachmentDownloader(attachmentsDir string) *AttachmentDownloader {
	return &AttachmentDownloader{
		attachmentsDir: attachmentsDir,
	}
}

// ExtractAttachmentContent extracts the content of a specific attachment from raw email bytes
func (d *AttachmentDownloader) ExtractAttachmentContent(raw []byte, targetFilename string) ([]byte, error) {
	return d.ExtractAttachmentContentFromReader(bytes.NewReader(raw), targetFilename)
}

// ExtractAttachmentContentFromReader extracts one attachment without requiring
// callers to keep the whole raw message in memory.
func (d *AttachmentDownloader) ExtractAttachmentContentFromReader(raw io.Reader, targetFilename string) ([]byte, error) {
	entity, err := gomessage.Read(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	// We need to find the attachment by matching properties
	if mr := entity.MultipartReader(); mr != nil {
		return d.findAttachmentInMultipart(mr, targetFilename)
	}

	// Single-part message: the whole entity may itself be the attachment.
	if getFilename(entity) == targetFilename {
		content, err := readAttachmentContent(entity.Body)
		if err != nil {
			return nil, err
		}
		transferEncoding := strings.ToLower(entity.Header.Get("Content-Transfer-Encoding"))
		return decodeAttachmentContent(content, transferEncoding)
	}

	return nil, fmt.Errorf("attachment not found: %s", targetFilename)
}

// findAttachmentInMultipart searches for an attachment by filename in a multipart message
func (d *AttachmentDownloader) findAttachmentInMultipart(mr gomessage.MultipartReader, targetFilename string) ([]byte, error) {
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Handle nested multipart
		if nestedMr := part.MultipartReader(); nestedMr != nil {
			if content, err := d.findAttachmentInMultipart(nestedMr, targetFilename); err == nil {
				return content, nil
			}
			continue
		}

		// Check filename
		filename := getFilename(part)
		if filename == targetFilename {
			content, err := readAttachmentContent(part.Body)
			if err != nil {
				return nil, err
			}

			// Decode content if transfer-encoded
			transferEncoding := strings.ToLower(part.Header.Get("Content-Transfer-Encoding"))
			return decodeAttachmentContent(content, transferEncoding)
		}
	}

	return nil, fmt.Errorf("attachment not found: %s", targetFilename)
}

// decodeMIMEFilename decodes a MIME-encoded filename with full charset support.
// Mirrors the sync code's decodeMIMEWord() to ensure filenames match between
// sync (when stored to DB) and download (when extracting from raw message).
func decodeMIMEFilename(s string) string {
	if s == "" {
		return s
	}
	dec := &mime.WordDecoder{
		CharsetReader: func(charsetName string, r io.Reader) (io.Reader, error) {
			if reader, err := msgcharset.Reader(charsetName, r); err == nil {
				return reader, nil
			}
			enc, err := htmlindex.Get(charsetName)
			if err != nil {
				return nil, fmt.Errorf("unknown charset: %s", charsetName)
			}
			return enc.NewDecoder().Reader(r), nil
		},
	}
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

// getFilename extracts the filename from a message part
func getFilename(part *gomessage.Entity) string {
	// Try Content-Disposition first
	if disp := part.Header.Get("Content-Disposition"); disp != "" {
		_, params, _ := mime.ParseMediaType(disp)
		if filename := params["filename"]; filename != "" {
			return decodeMIMEFilename(filename)
		}
	}

	// Try Content-Type name parameter
	if ct := part.Header.Get("Content-Type"); ct != "" {
		_, params, _ := mime.ParseMediaType(ct)
		if name := params["name"]; name != "" {
			return decodeMIMEFilename(name)
		}
	}

	// Synthetic fallback: match sync/parse.go extractAttachmentMetadata logic
	contentType := "application/octet-stream"
	if ct := part.Header.Get("Content-Type"); ct != "" {
		mt, _, _ := mime.ParseMediaType(ct)
		if mt != "" {
			contentType = mt
		}
	}

	ext := ".bin"
	if strings.HasPrefix(contentType, "image/") {
		parts := strings.SplitN(contentType, "/", 2)
		if len(parts) == 2 {
			ext = "." + parts[1]
		}
	}
	return "attachment" + ext
}

// SafeAttachmentFilename strips any path components from untrusted attachment
// metadata before using it as a filesystem name.
func SafeAttachmentFilename(filename string) (string, error) {
	name := strings.TrimSpace(strings.ReplaceAll(filename, "\x00", ""))
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)

	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("unsafe attachment filename %q", filename)
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("unsafe attachment filename %q", filename)
	}
	return name, nil
}

// DefaultAttachmentFilename returns a safe name suitable for save dialogs.
func DefaultAttachmentFilename(filename string) string {
	name, err := SafeAttachmentFilename(filename)
	if err != nil {
		return "attachment.bin"
	}
	return name
}

// UniqueAttachmentPath returns a non-existing path under dir for an attachment.
func UniqueAttachmentPath(dir, filename string) (string, error) {
	safeName, err := SafeAttachmentFilename(filename)
	if err != nil {
		return "", err
	}

	savePath := filepath.Join(dir, safeName)
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		return savePath, nil
	} else if err != nil {
		return "", err
	}

	ext := filepath.Ext(safeName)
	base := strings.TrimSuffix(safeName, ext)
	for i := 1; ; i++ {
		savePath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(savePath); os.IsNotExist(err) {
			return savePath, nil
		} else if err != nil {
			return "", err
		}
	}
}

// SaveAttachment saves attachment content to disk
func (d *AttachmentDownloader) SaveAttachment(att *message.Attachment, content []byte, customPath string) (string, error) {
	if att == nil {
		return "", fmt.Errorf("attachment is required")
	}

	var savePath string

	if customPath != "" {
		// Use custom path provided by user
		savePath = customPath
	} else {
		// Save to default attachments directory
		// Create subdirectory based on message ID for organization
		subDir := filepath.Join(d.attachmentsDir, attachmentMessagePrefix(att.MessageID))
		if err := os.MkdirAll(subDir, 0700); err != nil {
			return "", fmt.Errorf("failed to create attachment directory: %w", err)
		}

		// Generate unique filename to avoid conflicts
		var err error
		savePath, err = UniqueAttachmentPath(subDir, att.Filename)
		if err != nil {
			return "", err
		}
	}

	// Write content to file
	if err := os.WriteFile(savePath, content, 0600); err != nil {
		return "", fmt.Errorf("failed to write attachment: %w", err)
	}

	return savePath, nil
}

func readAttachmentContent(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

func decodeAttachmentContent(content []byte, encoding string) ([]byte, error) {
	return decodeContent(content, encoding), nil
}

func attachmentMessagePrefix(messageID string) string {
	const prefixLen = 8
	if len(messageID) >= prefixLen {
		return messageID[:prefixLen]
	}
	if messageID == "" {
		return "unknown"
	}
	return messageID
}
