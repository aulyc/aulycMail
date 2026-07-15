package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"aulyc.local/aulycmail/internal/email"
	"aulyc.local/aulycmail/internal/logging"
	"aulyc.local/aulycmail/internal/message"
	"github.com/rs/zerolog"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Attachment API - Exposed to frontend via Wails bindings
// ============================================================================

// GetAttachments returns all attachments for a message
func (a *App) GetAttachments(messageID string) ([]*message.Attachment, error) {
	return a.attachmentStore.GetByMessage(messageID)
}

// GetInlineAttachments returns a map of content-id to data URL for all inline attachments
// This is used to resolve cid: references in HTML email bodies
// Content is read from the database (stored during sync) for fast offline access
func (a *App) GetInlineAttachments(messageID string) (map[string]string, error) {
	log := logging.WithComponent("app")

	log.Info().Str("messageID", messageID).Msg("GetInlineAttachments called")

	// Get inline attachments with content from database
	// This is fast and works offline since content is stored during sync
	result, err := a.attachmentStore.GetInlineByMessage(messageID)
	if err != nil {
		log.Error().Err(err).Str("messageID", messageID).Msg("Failed to get inline attachments from database")
		return nil, fmt.Errorf("failed to get inline attachments: %w", err)
	}

	// Log the content IDs we found
	contentIDs := make([]string, 0, len(result))
	for cid := range result {
		contentIDs = append(contentIDs, cid)
	}
	log.Info().Int("count", len(result)).Strs("contentIDs", contentIDs).Str("messageID", messageID).Msg("Returning inline attachments")

	return result, nil
}

// DownloadAttachment downloads an attachment and saves it to disk
// If savePath is empty, saves to the default attachments directory
// Returns the path where the file was saved
func (a *App) DownloadAttachment(attachmentID, savePath string) (string, error) {
	log := logging.WithComponent("app")

	log.Debug().Str("attachmentID", attachmentID).Str("savePath", savePath).Msg("DownloadAttachment called")

	// Get attachment metadata
	att, err := a.attachmentStore.Get(attachmentID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get attachment from store")
		return "", fmt.Errorf("failed to get attachment: %w", err)
	}
	if att == nil {
		log.Error().Str("attachmentID", attachmentID).Msg("Attachment not found")
		return "", fmt.Errorf("attachment not found: %s", attachmentID)
	}

	log.Debug().Str("filename", att.Filename).Int("size", att.Size).Msg("Got attachment metadata")

	// Check if already downloaded (only for default location, not custom paths)
	if savePath == "" && att.LocalPath != "" {
		if _, err := os.Stat(att.LocalPath); err == nil {
			log.Debug().Str("localPath", att.LocalPath).Msg("Attachment already downloaded")
			return att.LocalPath, nil
		}
	}

	// Get the message to find folder and UID
	msg, err := a.messageStore.Get(att.MessageID)
	if err != nil {
		log.Error().Err(err).Str("messageID", att.MessageID).Msg("Failed to get message")
		return "", fmt.Errorf("failed to get message: %w", err)
	}
	if msg == nil {
		log.Error().Str("messageID", att.MessageID).Msg("Message not found")
		return "", fmt.Errorf("message not found: %s", att.MessageID)
	}

	log.Debug().Uint32("uid", msg.UID).Str("folderID", msg.FolderID).Msg("Got message info")

	downloader := email.NewAttachmentDownloader(a.paths.AttachmentsPath())
	var localPath string
	var contentSize int64

	err = a.withStreamedRawMessage(msg, func(raw io.Reader) error {
		var err error
		localPath, contentSize, err = downloader.SaveAttachmentFromRawReader(raw, att, savePath)
		if err != nil {
			return err
		}
		log.Debug().Int64("contentSize", contentSize).Msg("Extracted and saved attachment content")
		return nil
	})
	if err != nil {
		log.Error().Err(err).Str("filename", att.Filename).Msg("Failed to download attachment")
		return "", err
	}

	// Update attachment record with local path (only for default location)
	if savePath == "" {
		if err := a.attachmentStore.UpdateLocalPath(attachmentID, localPath); err != nil {
			log.Warn().Err(err).Msg("Failed to update attachment local path")
		}
	}

	log.Info().Str("attachment", att.Filename).Str("path", localPath).Int64("size", contentSize).Msg("Attachment downloaded")
	return localPath, nil
}

// OpenAttachment downloads (if needed) and opens an attachment with the default application
func (a *App) OpenAttachment(attachmentID string) error {
	// Download if not already downloaded
	localPath, err := a.DownloadAttachment(attachmentID, "")
	if err != nil {
		return err
	}

	// Open with default application using runtime
	return a.openFile(localPath)
}

// SaveAttachmentAs shows a Save As dialog and saves the attachment to the user-selected location
// Returns the path where the file was saved, or empty string if cancelled
func (a *App) SaveAttachmentAs(attachmentID string) (string, error) {
	log := logging.WithComponent("app")

	log.Debug().Str("attachmentID", attachmentID).Msg("SaveAttachmentAs called")

	// Get attachment metadata for the filename
	att, err := a.attachmentStore.Get(attachmentID)
	if err != nil {
		log.Error().Err(err).Str("attachmentID", attachmentID).Msg("Failed to get attachment metadata")
		return "", fmt.Errorf("failed to get attachment: %w", err)
	}
	if att == nil {
		log.Error().Str("attachmentID", attachmentID).Msg("Attachment not found in database")
		return "", fmt.Errorf("attachment not found: %s", attachmentID)
	}

	log.Debug().Str("filename", att.Filename).Str("messageID", att.MessageID).Msg("Found attachment metadata")

	// Get user's home directory for default save location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	defaultDir := filepath.Join(homeDir, "Downloads")

	savePath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultDirectory: defaultDir,
		DefaultFilename:  email.DefaultAttachmentFilename(att.Filename),
		Title:            "Save Attachment",
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to show save dialog")
		return "", fmt.Errorf("failed to show save dialog: %w", err)
	}

	log.Debug().Str("savePath", savePath).Msg("User selected save path")

	// User cancelled the dialog
	if savePath == "" {
		log.Debug().Msg("User cancelled save dialog")
		return "", nil
	}

	// Download and save to the selected path
	resultPath, err := a.DownloadAttachment(attachmentID, savePath)
	if err != nil {
		log.Error().Err(err).Str("savePath", savePath).Msg("Failed to download attachment")
		return "", err
	}

	log.Info().Str("attachment", att.Filename).Str("path", resultPath).Msg("Attachment saved")
	return resultPath, nil
}

// openFile opens a file with the system default application
func (a *App) openFile(path string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command("open", path).Start()
}

// validateOpenPath checks that the path is under an allowed root directory
func (a *App) validateOpenPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	allowedRoots := []string{
		a.paths.AttachmentsPath(),
		filepath.Join(homeDir, "Downloads"),
		a.paths.Data,
	}

	for _, root := range allowedRoots {
		cleanRoot := filepath.Clean(root) + string(filepath.Separator)
		if strings.HasPrefix(absPath, cleanRoot) || absPath == filepath.Clean(root) {
			return nil
		}
	}

	return fmt.Errorf("path %q is outside allowed directories", path)
}

// OpenFile opens a file with the system default application (exposed to frontend)
func (a *App) OpenFile(path string) error {
	if err := a.validateOpenPath(path); err != nil {
		return err
	}
	return a.openFile(path)
}

// OpenFolder opens the folder containing a file in the system file manager
func (a *App) OpenFolder(path string) error {
	if err := a.validateOpenPath(path); err != nil {
		return err
	}

	if runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command("open", "-R", path).Start()
}

// SaveAllAttachments shows a folder picker and saves all attachments from a message to that folder
// Returns the folder path where files were saved, or empty string if cancelled
func (a *App) SaveAllAttachments(messageID string) (string, error) {
	log := logging.WithComponent("app")

	// Get all attachments for the message
	attachments, err := a.attachmentStore.GetByMessage(messageID)
	if err != nil {
		return "", fmt.Errorf("failed to get attachments: %w", err)
	}
	if len(attachments) == 0 {
		return "", fmt.Errorf("no attachments found for message")
	}

	// Get user's home directory for default save location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	defaultDir := filepath.Join(homeDir, "Downloads")

	saveDir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		DefaultDirectory: defaultDir,
		Title:            "Save All Attachments",
	})
	if err != nil {
		return "", fmt.Errorf("failed to show folder dialog: %w", err)
	}

	// User cancelled the dialog
	if saveDir == "" {
		return "", nil
	}

	// Get the message to find folder and UID
	msg, err := a.messageStore.Get(messageID)
	if err != nil {
		return "", fmt.Errorf("failed to get message: %w", err)
	}
	if msg == nil {
		return "", fmt.Errorf("message not found: %s", messageID)
	}

	downloader := email.NewAttachmentDownloader(a.paths.AttachmentsPath())
	targets := make([]email.AttachmentSaveTarget, 0, len(attachments))
	savedCount := 0

	err = a.withStreamedRawMessagePath(msg, func(rawPath string) error {
		for _, att := range attachments {
			savePath, err := email.UniqueAttachmentPath(saveDir, att.Filename)
			if err != nil {
				log.Warn().Err(err).Str("filename", att.Filename).Msg("Skipping attachment with unsafe filename")
				continue
			}
			targets = append(targets, email.AttachmentSaveTarget{Attachment: att, CustomPath: savePath})
		}
		var err error
		savedCount, err = saveAttachmentTargetsFromRawFile(downloader, rawPath, targets, log)
		return err
	})
	if err != nil {
		return "", err
	}

	log.Info().Int("count", savedCount).Str("folder", saveDir).Msg("Saved all attachments")
	return saveDir, nil
}

func (a *App) withStreamedRawMessage(msg *message.Message, use func(io.Reader) error) error {
	return a.withStreamedRawMessagePath(msg, func(rawPath string) error {
		file, err := os.Open(rawPath)
		if err != nil {
			return fmt.Errorf("failed to open streamed message: %w", err)
		}
		defer file.Close()
		return use(file)
	})
}

func (a *App) withStreamedRawMessagePath(msg *message.Message, use func(string) error) error {
	backupPath, found, err := a.indexedBackupMessagePath(msg)
	if err != nil {
		return err
	}
	if found {
		file, err := os.Open(backupPath)
		if err != nil {
			return fmt.Errorf("failed to open indexed backup message: %w", err)
		}
		validateErr := a.syncEngine.ValidateMessageIdentityFromReader(a.ctx, msg.ID, file)
		closeErr := file.Close()
		if validateErr != nil {
			return fmt.Errorf("indexed backup message failed identity validation: %w", validateErr)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close indexed backup message: %w", closeErr)
		}
		return use(backupPath)
	}
	f, err := a.folderStore.Get(msg.FolderID)
	if err != nil {
		return fmt.Errorf("failed to get message folder: %w", err)
	}
	if f == nil {
		return fmt.Errorf("folder not found: %s", msg.FolderID)
	}
	if !f.IsSelectable() {
		return unavailableLocalMessageSourceError()
	}

	tmp, err := os.CreateTemp("", "aulycmail-raw-*.eml")
	if err != nil {
		return fmt.Errorf("failed to create temp message file: %w", err)
	}
	rawPath := tmp.Name()
	defer os.Remove(rawPath)
	defer tmp.Close()

	if _, err := a.syncEngine.StreamRawMessage(a.ctx, msg.AccountID, msg.FolderID, msg.UID, tmp); err != nil {
		return fmt.Errorf("failed to fetch message: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind temp message file: %w", err)
	}
	return use(rawPath)
}

func saveAttachmentTargetsFromRawFile(downloader *email.AttachmentDownloader, rawPath string, targets []email.AttachmentSaveTarget, log zerolog.Logger) (int, error) {
	file, err := os.Open(rawPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open streamed message: %w", err)
	}
	defer file.Close()

	results, err := downloader.SaveAttachmentsFromRawReader(file, targets)
	if err != nil {
		return 0, err
	}
	savedCount := 0
	for _, result := range results {
		if result.Err == nil && result.Path != "" {
			savedCount++
			continue
		}
		filename := ""
		if result.Attachment != nil {
			filename = result.Attachment.Filename
		}
		log.Warn().Err(result.Err).Str("filename", filename).Msg("Failed to save attachment")
	}
	return savedCount, nil
}
