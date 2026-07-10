package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aulyc/aulycmail/internal/email"
	"github.com/aulyc/aulycmail/internal/logging"
	"github.com/aulyc/aulycmail/internal/message"
	"github.com/aulyc/aulycmail/internal/platform"
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

	// In Flatpak, use portal save dialog (Wails GTK dialog doesn't route through portal)
	if platform.IsFlatpak() {
		savePath, err := platform.PortalSaveFile("Save Attachment", email.DefaultAttachmentFilename(att.Filename), defaultDir)
		if err != nil {
			log.Error().Err(err).Msg("Failed to show portal save dialog")
			return "", fmt.Errorf("failed to show save dialog: %w", err)
		}
		if savePath == "" {
			log.Debug().Msg("User cancelled save dialog")
			return "", nil
		}
		return a.DownloadAttachment(attachmentID, savePath)
	}

	// Native: use Wails dialog
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
	if runtime.GOOS == "linux" && platform.IsFlatpak() {
		if platform.IsDocPortalPath(path) {
			wailsRuntime.EventsEmit(a.ctx, "flatpak:filesystem-dialog")
			return nil // Don't open — portal FUSE path is broken for editing
		}
		return platform.PortalOpenFile(path)
	}

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		// ShellExecute, not `cmd /c start`: cmd re-parses `&`/`|`/`^` in the
		// path as command separators, so an attachment filename like
		// `x&calc.pdf` would inject commands (same class as issue #261).
		// ShellExecute passes the path as a single arg with no shell re-parse.
		return platform.OpenPathWindows(path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
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

	// In Flatpak, use the OpenURI portal to resolve sandboxed paths correctly
	if runtime.GOOS == "linux" && platform.IsFlatpak() {
		return platform.PortalOpenDirectory(path)
	}

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	case "darwin":
		// -R reveals the file in Finder
		cmd = exec.Command("open", "-R", path)
	case "windows":
		// /select highlights the file in Explorer
		cmd = exec.Command("explorer", "/select,", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
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

	// In Flatpak, use portal save dialog (Wails GTK dialog doesn't route through portal)
	if platform.IsFlatpak() {
		return a.saveAllAttachmentsViaPortal(messageID, attachments, defaultDir)
	}

	// Native: use Wails folder dialog
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
	savedCount := 0

	err = a.withStreamedRawMessagePath(msg, func(rawPath string) error {
		for _, att := range attachments {
			savePath, err := email.UniqueAttachmentPath(saveDir, att.Filename)
			if err != nil {
				log.Warn().Err(err).Str("filename", att.Filename).Msg("Skipping attachment with unsafe filename")
				continue
			}
			_, _, err = saveAttachmentFromRawFile(downloader, rawPath, att, savePath)
			if err != nil {
				log.Warn().Err(err).Str("filename", att.Filename).Msg("Failed to save attachment")
				continue
			}
			savedCount++
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	log.Info().Int("count", savedCount).Str("folder", saveDir).Msg("Saved all attachments")
	return saveDir, nil
}

// saveAllAttachmentsViaPortal saves all attachments using the XDG FileChooser portal.
func (a *App) saveAllAttachmentsViaPortal(messageID string, attachments []*message.Attachment, defaultDir string) (string, error) {
	log := logging.WithComponent("app")

	filenames := make([]string, len(attachments))
	for i, att := range attachments {
		filenames[i] = email.DefaultAttachmentFilename(att.Filename)
	}

	savePaths, err := platform.PortalSaveFiles("Save All Attachments", filenames, defaultDir)
	if err != nil {
		return "", fmt.Errorf("failed to show save dialog: %w", err)
	}
	if len(savePaths) == 0 {
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
	savedCount := 0

	err = a.withStreamedRawMessagePath(msg, func(rawPath string) error {
		for i, att := range attachments {
			if i >= len(savePaths) {
				break
			}

			_, _, err := saveAttachmentFromRawFile(downloader, rawPath, att, savePaths[i])
			if err != nil {
				log.Warn().Err(err).Str("filename", att.Filename).Msg("Failed to save attachment")
				continue
			}
			savedCount++
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	log.Info().Int("count", savedCount).Msg("Saved all attachments via portal")
	return filepath.Dir(savePaths[0]), nil
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

func extractAttachmentFromRawFile(downloader *email.AttachmentDownloader, rawPath, filename string) ([]byte, error) {
	file, err := os.Open(rawPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open streamed message: %w", err)
	}
	defer file.Close()
	return downloader.ExtractAttachmentContentFromReader(file, filename)
}

func saveAttachmentFromRawFile(downloader *email.AttachmentDownloader, rawPath string, att *message.Attachment, savePath string) (string, int64, error) {
	file, err := os.Open(rawPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open streamed message: %w", err)
	}
	defer file.Close()
	return downloader.SaveAttachmentFromRawReader(file, att, savePath)
}
