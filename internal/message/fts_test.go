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
