package message

import (
	"context"
	"fmt"
	"reflect"
	"sort"
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

func TestFTSIndexerIndexesRebuildsAndRefreshesSelectableFolders(t *testing.T) {
	s, accountID, inboxID := newBodyFailedTestStore(t)
	const archiveID = "folder-archive"
	const groupID = "folder-group"
	const emptyID = "folder-empty"
	for _, folderFixture := range []struct {
		id         string
		name       string
		path       string
		selectable int
	}{
		{archiveID, "Archive", "Archive", 1},
		{groupID, "Group", "Group", 0},
		{emptyID, "Empty", "Empty", 1},
	} {
		if _, err := s.db.Exec(`
			INSERT INTO folders (id, account_id, name, path, folder_type, selectable)
			VALUES (?, ?, ?, ?, 'folder', ?)
		`, folderFixture.id, accountID, folderFixture.name, folderFixture.path, folderFixture.selectable); err != nil {
			t.Fatalf("seed folder %s: %v", folderFixture.id, err)
		}
	}

	now := time.Now().UTC()
	for _, fixture := range []*Message{
		{ID: "fts-inbox", AccountID: accountID, FolderID: inboxID, UID: 1, Subject: "Quarterly alpha", BodyText: "inbox needle", BodyFetched: true, Date: now},
		{ID: "fts-archive", AccountID: accountID, FolderID: archiveID, UID: 1, Subject: "Archived beta", BodyText: "archive needle", BodyFetched: true, Date: now.Add(time.Second)},
		{ID: "fts-group", AccountID: accountID, FolderID: groupID, UID: 1, Subject: "Hidden group", BodyText: "group needle", BodyFetched: true, Date: now.Add(2 * time.Second)},
	} {
		if err := s.Create(fixture); err != nil {
			t.Fatalf("create %s: %v", fixture.ID, err)
		}
	}

	ctx := context.Background()
	indexer := NewFTSIndexer(s.db.DB)
	if status, err := indexer.GetIndexStatus("missing"); err != nil || status != nil {
		t.Fatalf("missing status = (%#v, %v), want nil", status, err)
	}
	if indexer.IsIndexComplete("missing") || indexer.IsAnyIndexing() || len(indexer.GetIndexingFolders()) != 0 {
		t.Fatal("new indexer reported nonexistent or active indexing state")
	}

	var completed []string
	var progress map[string][2]int = make(map[string][2]int)
	indexer.SetProgressCallback(func(folderID string, indexed, total int) {
		progress[folderID] = [2]int{indexed, total}
	})
	indexer.SetCompleteCallback(func(folderID string) {
		completed = append(completed, folderID)
	})
	if err := indexer.IndexAllFolders(ctx); err != nil {
		t.Fatalf("IndexAllFolders: %v", err)
	}

	statuses, err := indexer.GetAllIndexStatuses()
	if err != nil {
		t.Fatalf("GetAllIndexStatuses: %v", err)
	}
	for _, folderID := range []string{inboxID, archiveID, emptyID} {
		status := statuses[folderID]
		if status == nil || !status.IsComplete || status.LastIndexedAt == "" {
			t.Fatalf("status for %s = %#v, want completed timestamp", folderID, status)
		}
	}
	if statuses[groupID] != nil {
		t.Fatalf("non-selectable group received index status: %#v", statuses[groupID])
	}
	if !indexer.IsIndexComplete(inboxID) || !indexer.IsIndexComplete(archiveID) || !indexer.IsIndexComplete(emptyID) {
		t.Fatalf("completed statuses = %#v", statuses)
	}
	if got := progress[inboxID]; got != [2]int{1, 1} {
		t.Fatalf("inbox progress = %v, want [1 1]", got)
	}
	if got := progress[archiveID]; got != [2]int{1, 1} {
		t.Fatalf("archive progress = %v, want [1 1]", got)
	}
	if _, exists := progress[emptyID]; exists {
		t.Fatalf("empty folder unexpectedly reported batch progress: %#v", progress)
	}

	completedBefore := len(completed)
	if err := indexer.IndexFolder(ctx, inboxID); err != nil {
		t.Fatalf("idempotent IndexFolder: %v", err)
	}
	if len(completed) != completedBefore {
		t.Fatalf("idempotent complete callbacks = %v", completed)
	}

	if err := indexer.RebuildIndex(ctx, inboxID); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}
	if !indexer.IsIndexComplete(inboxID) {
		t.Fatal("inbox not complete after rebuild")
	}
	if err := indexer.RebuildAllIndexes(ctx); err != nil {
		t.Fatalf("RebuildAllIndexes: %v", err)
	}
	for query, want := range map[string]int{"alpha": 1, "beta": 1, "group": 0} {
		var matches int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH ?`, query).Scan(&matches); err != nil {
			t.Fatalf("query %q after rebuild: %v", query, err)
		}
		if matches != want {
			t.Fatalf("query %q matches = %d, want %d", query, matches, want)
		}
	}

	if err := s.Delete("fts-archive"); err != nil {
		t.Fatalf("delete archive message: %v", err)
	}
	if err := indexer.IndexFolder(ctx, archiveID); err != nil {
		t.Fatalf("refresh reduced complete folder: %v", err)
	}
	archiveStatus, err := indexer.GetIndexStatus(archiveID)
	if err != nil || archiveStatus == nil || archiveStatus.IndexedCount != 0 || archiveStatus.TotalCount != 0 || !archiveStatus.IsComplete {
		t.Fatalf("archive status after delete = (%#v, %v)", archiveStatus, err)
	}
}

func TestFTSIndexerPublishesConcurrentStateAndHonorsCancellation(t *testing.T) {
	s, accountID, folderID := newBodyFailedTestStore(t)
	if err := s.Create(&Message{
		ID: "fts-concurrent", AccountID: accountID, FolderID: folderID, UID: 1,
		Subject: "Concurrent indexing", BodyText: "blocking needle", BodyFetched: true, Date: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create message: %v", err)
	}

	indexer := NewFTSIndexer(s.db.DB)
	reachedProgress := make(chan struct{})
	releaseProgress := make(chan struct{})
	indexer.SetProgressCallback(func(gotFolderID string, indexed, total int) {
		if gotFolderID == folderID && indexed == 1 && total == 1 {
			close(reachedProgress)
			<-releaseProgress
		}
	})
	done := make(chan error, 1)
	go func() {
		done <- indexer.IndexFolder(context.Background(), folderID)
	}()

	select {
	case <-reachedProgress:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for indexing progress")
	}
	if !indexer.IsAnyIndexing() {
		t.Fatal("IsAnyIndexing = false while progress callback is blocked")
	}
	gotFolders := indexer.GetIndexingFolders()
	sort.Strings(gotFolders)
	if !reflect.DeepEqual(gotFolders, []string{folderID}) {
		t.Fatalf("GetIndexingFolders = %v, want [%s]", gotFolders, folderID)
	}
	if err := indexer.IndexFolder(context.Background(), folderID); err != nil {
		t.Fatalf("duplicate IndexFolder while active: %v", err)
	}
	close(releaseProgress)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("background IndexFolder: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for indexing completion")
	}
	if indexer.IsAnyIndexing() || len(indexer.GetIndexingFolders()) != 0 {
		t.Fatal("indexing state was not cleared after completion")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := yieldIndexing(cancelled, time.Second); err != context.Canceled {
		t.Fatalf("yieldIndexing cancellation = %v, want context.Canceled", err)
	}
	if err := yieldIndexing(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("yieldIndexing timer completion: %v", err)
	}
}
