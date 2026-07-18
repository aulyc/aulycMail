package app

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aulyc.local/aulycmail/internal/logging"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	externalFileOpenDebounce = 200 * time.Millisecond
	externalFileOpenEvent    = "files:openAsAttachments"
)

var errExternalOpenNotRegular = errors.New("external open target is not a regular file")

type externalFileOpenBatcher struct {
	mu       sync.Mutex
	debounce time.Duration
	emit     func([]string)
	pending  []string
	seen     map[string]struct{}
	ready    bool
	stopped  bool
	timer    *time.Timer
}

func newExternalFileOpenBatcher(debounce time.Duration, emit func([]string)) *externalFileOpenBatcher {
	return &externalFileOpenBatcher{
		debounce: debounce,
		emit:     emit,
		seen:     make(map[string]struct{}),
	}
}

func normalizeExternalOpenFile(filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errExternalOpenNotRegular
	}

	return filepath.Clean(absPath), nil
}

func (b *externalFileOpenBatcher) Add(filePath string) error {
	normalized, err := normalizeExternalOpenFile(filePath)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return nil
	}
	if _, exists := b.seen[normalized]; exists {
		return nil
	}

	b.seen[normalized] = struct{}{}
	b.pending = append(b.pending, normalized)
	b.scheduleLocked()
	return nil
}

func (b *externalFileOpenBatcher) SetReady() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return
	}
	b.ready = true
	b.scheduleLocked()
}

func (b *externalFileOpenBatcher) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = true
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	b.pending = nil
	b.seen = make(map[string]struct{})
}

func (b *externalFileOpenBatcher) scheduleLocked() {
	if !b.ready || len(b.pending) == 0 || b.stopped {
		return
	}
	if b.timer != nil {
		b.timer.Stop()
	}
	b.timer = time.AfterFunc(b.debounce, b.flush)
}

func (b *externalFileOpenBatcher) flush() {
	b.mu.Lock()
	if !b.ready || b.stopped || len(b.pending) == 0 {
		b.mu.Unlock()
		return
	}
	paths := append([]string(nil), b.pending...)
	b.pending = nil
	b.seen = make(map[string]struct{})
	b.timer = nil
	emit := b.emit
	b.mu.Unlock()

	if emit != nil {
		emit(paths)
	}
}

// HandleFileOpen is the native macOS file-open callback entrypoint. It is a
// package function rather than an App method so Wails does not expose it as a
// frontend bridge API.
func HandleFileOpen(application *App, filePath string) {
	if application == nil || application.externalFileOpen == nil {
		return
	}
	if err := application.externalFileOpen.Add(filePath); err != nil {
		// File paths can contain sensitive user information. Keep the log
		// intentionally path-free; the frontend reports attachment failures by
		// count when a previously valid file later becomes unreadable.
		log := logging.WithComponent("external-file-open")
		log.Warn().Msg("Ignored unsupported file-open request")
	}
}

func (a *App) emitExternalOpenFiles(paths []string) {
	if len(paths) == 0 {
		return
	}
	a.ShowWindow()
	wailsRuntime.EventsEmit(a.ctx, externalFileOpenEvent, map[string]interface{}{
		"paths": paths,
	})
}
