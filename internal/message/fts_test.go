package message

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestFTSIndexerSupplementsCompleteFolderOnCountIncrease(t *testing.T) {
	s, accountID, folderID := newBodyFailedTestStore(t)
	now := time.Now().UTC()

	for i := 1; i <= 3; i++ {
		msg := &Message{
			ID:          fmt.Sprintf("msg-%d", i),
			AccountID:   accountID,
			FolderID:    folderID,
			UID:         uint32(i),
			Subject:     fmt.Sprintf("subject %d", i),
			BodyText:    fmt.Sprintf("needle%d", i),
			BodyFetched: true,
			Date:        now.Add(time.Duration(i) * time.Second),
		}
		if err := s.Create(msg); err != nil {
			t.Fatalf("Create message %d: %v", i, err)
		}
	}

	ctx := context.Background()
	indexer := NewFTSIndexer(s.db.DB)
	if err := indexer.updateIndexStatus(ctx, folderID, 2, 2, true); err != nil {
		t.Fatalf("seed index status: %v", err)
	}

	var progress []int
	indexer.SetProgressCallback(func(gotFolderID string, indexed, total int) {
		if gotFolderID != folderID {
			t.Fatalf("progress folder = %s, want %s", gotFolderID, folderID)
		}
		if total != 3 {
			t.Fatalf("progress total = %d, want 3", total)
		}
		progress = append(progress, indexed)
	})

	completed := false
	indexer.SetCompleteCallback(func(gotFolderID string) {
		if gotFolderID != folderID {
			t.Fatalf("complete folder = %s, want %s", gotFolderID, folderID)
		}
		completed = true
	})

	if err := indexer.IndexFolder(ctx, folderID); err != nil {
		t.Fatalf("IndexFolder: %v", err)
	}

	status, err := indexer.GetIndexStatus(folderID)
	if err != nil {
		t.Fatalf("GetIndexStatus: %v", err)
	}
	if status == nil {
		t.Fatal("GetIndexStatus returned nil")
	}
	if status.IndexedCount != 3 || status.TotalCount != 3 || !status.IsComplete {
		t.Fatalf("status = indexed:%d total:%d complete:%v, want indexed:3 total:3 complete:true",
			status.IndexedCount, status.TotalCount, status.IsComplete)
	}
	if len(progress) != 1 || progress[0] != 3 {
		t.Fatalf("progress = %v, want [3]", progress)
	}
	if !completed {
		t.Fatal("complete callback was not called")
	}
}

func TestFTSIndexerRepairsMissingRowsInCompleteFolder(t *testing.T) {
	s, accountID, folderID := newBodyFailedTestStore(t)
	now := time.Now().UTC()

	for i := 1; i <= 3; i++ {
		msg := &Message{
			ID:          fmt.Sprintf("hole-msg-%d", i),
			AccountID:   accountID,
			FolderID:    folderID,
			UID:         uint32(i),
			Subject:     fmt.Sprintf("hole subject %d", i),
			BodyText:    fmt.Sprintf("hole_needle_%d", i),
			BodyFetched: true,
			Date:        now.Add(time.Duration(i) * time.Second),
		}
		if err := s.Create(msg); err != nil {
			t.Fatalf("Create message %d: %v", i, err)
		}
	}

	if _, err := s.db.Exec(`
		INSERT INTO messages_fts(messages_fts, rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text)
		SELECT 'delete', rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text
		FROM messages
		WHERE id = 'hole-msg-2'
	`); err != nil {
		t.Fatalf("remove FTS row: %v", err)
	}

	ctx := context.Background()
	indexer := NewFTSIndexer(s.db.DB)
	if err := indexer.updateIndexStatus(ctx, folderID, 2, 2, true); err != nil {
		t.Fatalf("seed index status: %v", err)
	}

	if err := indexer.IndexFolder(ctx, folderID); err != nil {
		t.Fatalf("IndexFolder: %v", err)
	}

	var matches int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'hole_needle_2'`).Scan(&matches); err != nil {
		t.Fatalf("query repaired FTS row: %v", err)
	}
	if matches != 1 {
		t.Fatalf("FTS matches after repair = %d, want 1", matches)
	}
}

func TestFTSIndexerFinalizesInterruptedCompleteBatchWithoutRescan(t *testing.T) {
	s, accountID, folderID := newBodyFailedTestStore(t)
	now := time.Now().UTC()

	for i := 1; i <= 3; i++ {
		msg := &Message{
			ID:          fmt.Sprintf("interrupted-msg-%d", i),
			AccountID:   accountID,
			FolderID:    folderID,
			UID:         uint32(i),
			Subject:     fmt.Sprintf("interrupted subject %d", i),
			BodyText:    fmt.Sprintf("interrupted_needle_%d", i),
			BodyFetched: true,
			Date:        now.Add(time.Duration(i) * time.Second),
		}
		if err := s.Create(msg); err != nil {
			t.Fatalf("Create message %d: %v", i, err)
		}
	}

	ctx := context.Background()
	indexer := NewFTSIndexer(s.db.DB)
	if err := indexer.updateIndexStatus(ctx, folderID, 3, 3, false); err != nil {
		t.Fatalf("seed interrupted index status: %v", err)
	}

	progressCalls := 0
	indexer.SetProgressCallback(func(string, int, int) {
		progressCalls++
	})
	completed := false
	indexer.SetCompleteCallback(func(gotFolderID string) {
		if gotFolderID != folderID {
			t.Fatalf("complete folder = %s, want %s", gotFolderID, folderID)
		}
		completed = true
	})

	if err := indexer.IndexFolder(ctx, folderID); err != nil {
		t.Fatalf("IndexFolder: %v", err)
	}

	status, err := indexer.GetIndexStatus(folderID)
	if err != nil {
		t.Fatalf("GetIndexStatus: %v", err)
	}
	if status == nil || status.IndexedCount != 3 || status.TotalCount != 3 || !status.IsComplete {
		t.Fatalf("status = %#v, want indexed:3 total:3 complete:true", status)
	}
	if progressCalls != 0 {
		t.Fatalf("progress calls = %d, want 0 for metadata-only repair", progressCalls)
	}
	if !completed {
		t.Fatal("complete callback was not called")
	}
}
