package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExternalFileOpenBatcherWaitsForReadyAndBatchesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.pdf")
	if err := os.WriteFile(first, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}

	batches := make(chan []string, 1)
	batcher := newExternalFileOpenBatcher(10*time.Millisecond, func(paths []string) {
		batches <- paths
	})
	t.Cleanup(batcher.Stop)

	if err := batcher.Add(first); err != nil {
		t.Fatal(err)
	}
	if err := batcher.Add(second); err != nil {
		t.Fatal(err)
	}
	if err := batcher.Add(first); err != nil {
		t.Fatal(err)
	}

	select {
	case paths := <-batches:
		t.Fatalf("emitted before frontend readiness: %v", paths)
	case <-time.After(30 * time.Millisecond):
	}

	batcher.SetReady()
	select {
	case paths := <-batches:
		want := []string{first, second}
		if len(paths) != len(want) {
			t.Fatalf("batch length = %d, want %d: %v", len(paths), len(want), paths)
		}
		for i := range want {
			if paths[i] != want[i] {
				t.Fatalf("paths[%d] = %q, want %q", i, paths[i], want[i])
			}
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for batched file-open event")
	}
}

func TestExternalFileOpenBatcherRejectsNonFiles(t *testing.T) {
	t.Parallel()

	batcher := newExternalFileOpenBatcher(10*time.Millisecond, nil)
	t.Cleanup(batcher.Stop)

	if err := batcher.Add(t.TempDir()); !errors.Is(err, errExternalOpenNotRegular) {
		t.Fatalf("directory error = %v, want %v", err, errExternalOpenNotRegular)
	}
	if err := batcher.Add(filepath.Join(t.TempDir(), "missing.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error = %v, want os.ErrNotExist", err)
	}
}
