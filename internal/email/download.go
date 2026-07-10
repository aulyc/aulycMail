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

// AttachmentSaveTarget describes one attachment extraction target.
type AttachmentSaveTarget struct {
	Attachment *message.Attachment
	CustomPath string
}

// AttachmentSaveResult is the per-attachment outcome from a batch extraction.
type AttachmentSaveResult struct {
	Attachment *message.Attachment
	Path       string
	Written    int64
	Err        error
}

// AttachmentContentResult is the per-attachment outcome from a batch content
// extraction.
type AttachmentContentResult struct {
	Filename string
	Content  []byte
	Err      error
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
	var buf bytes.Buffer
	if _, err := d.ExtractAttachmentContentToWriter(raw, targetFilename, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExtractAttachmentsContentFromRawReader extracts multiple attachments in a
// single MIME traversal. It buffers only the requested attachment bodies.
func (d *AttachmentDownloader) ExtractAttachmentsContentFromRawReader(raw io.Reader, filenames []string) ([]AttachmentContentResult, error) {
	results := make([]AttachmentContentResult, len(filenames))
	pending := make(map[string][]int, len(filenames))

	for i, filename := range filenames {
		results[i].Filename = filename
		if filename == "" {
			results[i].Err = fmt.Errorf("attachment filename is required")
			continue
		}
		pending[filename] = append(pending[filename], i)
	}

	entity, err := gomessage.Read(raw)
	if err != nil {
		return results, fmt.Errorf("failed to parse message: %w", err)
	}

	extractMatch := func(part *gomessage.Entity) error {
		filename := getFilename(part)
		indexes := pending[filename]
		if len(indexes) == 0 {
			return nil
		}

		resultIndex := indexes[0]
		pending[filename] = indexes[1:]
		var buf bytes.Buffer
		if _, err := writeAttachmentEntityContent(part, &buf); err != nil {
			results[resultIndex].Err = err
			return nil
		}
		results[resultIndex].Content = buf.Bytes()
		return nil
	}

	if mr := entity.MultipartReader(); mr != nil {
		if err := walkAttachmentParts(mr, extractMatch); err != nil {
			return results, err
		}
	} else if err := extractMatch(entity); err != nil {
		return results, err
	}

	for _, indexes := range pending {
		for _, resultIndex := range indexes {
			if results[resultIndex].Err == nil && results[resultIndex].Content == nil {
				results[resultIndex].Err = fmt.Errorf("attachment not found: %s", results[resultIndex].Filename)
			}
		}
	}

	return results, nil
}

// ExtractAttachmentContentToWriter extracts one attachment and streams decoded
// content directly to dst.
func (d *AttachmentDownloader) ExtractAttachmentContentToWriter(raw io.Reader, targetFilename string, dst io.Writer) (int64, error) {
	entity, err := gomessage.Read(raw)
	if err != nil {
		return 0, fmt.Errorf("failed to parse message: %w", err)
	}

	// We need to find the attachment by matching properties
	if mr := entity.MultipartReader(); mr != nil {
		return d.findAttachmentInMultipartToWriter(mr, targetFilename, dst)
	}

	// Single-part message: the whole entity may itself be the attachment.
	if getFilename(entity) == targetFilename {
		return writeAttachmentEntityContent(entity, dst)
	}

	return 0, fmt.Errorf("attachment not found: %s", targetFilename)
}

// findAttachmentInMultipart searches for an attachment by filename in a multipart message
func (d *AttachmentDownloader) findAttachmentInMultipart(mr gomessage.MultipartReader, targetFilename string) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := d.findAttachmentInMultipartToWriter(mr, targetFilename, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (d *AttachmentDownloader) findAttachmentInMultipartToWriter(mr gomessage.MultipartReader, targetFilename string, dst io.Writer) (int64, error) {
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
			if n, err := d.findAttachmentInMultipartToWriter(nestedMr, targetFilename, dst); err == nil {
				return n, nil
			}
			continue
		}

		// Check filename
		filename := getFilename(part)
		if filename == targetFilename {
			return writeAttachmentEntityContent(part, dst)
		}
	}

	return 0, fmt.Errorf("attachment not found: %s", targetFilename)
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
	savePath, err := d.attachmentSavePath(att, customPath)
	if err != nil {
		return "", err
	}

	// Write content to file
	if err := os.WriteFile(savePath, content, 0600); err != nil {
		return "", fmt.Errorf("failed to write attachment: %w", err)
	}

	return savePath, nil
}

// SaveAttachmentFromRawReader extracts att from raw and streams it directly to
// disk without buffering the attachment content in memory.
func (d *AttachmentDownloader) SaveAttachmentFromRawReader(raw io.Reader, att *message.Attachment, customPath string) (string, int64, error) {
	savePath, err := d.attachmentSavePath(att, customPath)
	if err != nil {
		return "", 0, err
	}

	written, err := writeAttachmentToPath(savePath, func(dst io.Writer) (int64, error) {
		return d.ExtractAttachmentContentToWriter(raw, att.Filename, dst)
	})
	if err != nil {
		return "", written, err
	}

	return savePath, written, nil
}

// SaveAttachmentsFromRawReader extracts multiple attachments in a single MIME
// traversal. This avoids reparsing the same raw message once per attachment
// when the user saves all attachments from a message.
func (d *AttachmentDownloader) SaveAttachmentsFromRawReader(raw io.Reader, targets []AttachmentSaveTarget) ([]AttachmentSaveResult, error) {
	results := make([]AttachmentSaveResult, len(targets))
	pending := make(map[string][]int, len(targets))

	for i, target := range targets {
		results[i].Attachment = target.Attachment
		if target.Attachment == nil {
			results[i].Err = fmt.Errorf("attachment is required")
			continue
		}

		savePath, err := d.attachmentSavePath(target.Attachment, target.CustomPath)
		if err != nil {
			results[i].Err = err
			continue
		}
		results[i].Path = savePath
		pending[target.Attachment.Filename] = append(pending[target.Attachment.Filename], i)
	}

	entity, err := gomessage.Read(raw)
	if err != nil {
		return results, fmt.Errorf("failed to parse message: %w", err)
	}

	saveMatch := func(part *gomessage.Entity) error {
		filename := getFilename(part)
		indexes := pending[filename]
		if len(indexes) == 0 {
			return nil
		}

		resultIndex := indexes[0]
		pending[filename] = indexes[1:]
		written, err := writeAttachmentToPath(results[resultIndex].Path, func(dst io.Writer) (int64, error) {
			return writeAttachmentEntityContent(part, dst)
		})
		results[resultIndex].Written = written
		if err != nil {
			results[resultIndex].Err = err
		}
		return nil
	}

	if mr := entity.MultipartReader(); mr != nil {
		if err := walkAttachmentParts(mr, saveMatch); err != nil {
			return results, err
		}
	} else if err := saveMatch(entity); err != nil {
		return results, err
	}

	for _, indexes := range pending {
		for _, resultIndex := range indexes {
			if results[resultIndex].Err == nil {
				results[resultIndex].Err = fmt.Errorf("attachment not found: %s", results[resultIndex].Attachment.Filename)
			}
		}
	}

	return results, nil
}

func (d *AttachmentDownloader) attachmentSavePath(att *message.Attachment, customPath string) (string, error) {
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

	return savePath, nil
}

func readAttachmentContent(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

func decodeAttachmentContent(content []byte, encoding string) ([]byte, error) {
	return decodeContent(content, encoding), nil
}

func writeAttachmentEntityContent(entity *gomessage.Entity, dst io.Writer) (int64, error) {
	return io.Copy(dst, entity.Body)
}

func writeAttachmentToPath(savePath string, write func(io.Writer) (int64, error)) (int64, error) {
	file, err := os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to create attachment file: %w", err)
	}

	written, writeErr := write(file)
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(savePath)
		return written, fmt.Errorf("failed to extract attachment: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(savePath)
		return written, fmt.Errorf("failed to close attachment file: %w", closeErr)
	}
	return written, nil
}

func walkAttachmentParts(mr gomessage.MultipartReader, handle func(*gomessage.Entity) error) error {
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			continue
		}

		if nestedMr := part.MultipartReader(); nestedMr != nil {
			if err := walkAttachmentParts(nestedMr, handle); err != nil {
				return err
			}
			continue
		}

		if err := handle(part); err != nil {
			return err
		}
	}
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
