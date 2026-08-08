package backup

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/message"
	mailSync "aulyc.local/aulycmail/internal/sync"
)

const (
	ProgressStageChecking  = "checking"
	ProgressStageExporting = "exporting"
)

type Progress struct {
	Phase        string
	Stage        string
	StageCurrent int
	StageTotal   int
	AccountEmail string
	FolderPath   string
	Current      int
	Total        int
	Exported     int
	Skipped      int
	Missing      int
	Unavailable  int
	Failed       int
	Message      string
}

type RunOptions struct {
	Directory         string
	StartedAt         string
	AccountIDs        []string
	Mode              string
	RawFetchBatchSize int
	StreamRawMessages func(
		ctx context.Context,
		accountID string,
		folderID string,
		uids []uint32,
		handle mailSync.RawMessageStreamHandler,
	) (map[uint32]*mailSync.RawMessageStreamResult, map[uint32]error, error)
	EmitProgress func(Progress)
}

type RunResult struct {
	Directory   string
	Mode        string
	Total       int
	Exported    int
	Skipped     int
	Missing     int
	Unavailable int
	Failed      int
	ReportPath  string
}

func Run(ctx context.Context, db *database.DB, options RunOptions) (*RunResult, error) {
	if options.RawFetchBatchSize <= 0 {
		options.RawFetchBatchSize = 10
	}
	if options.StartedAt == "" {
		options.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if options.StreamRawMessages == nil {
		return nil, fmt.Errorf("backup raw message streamer is not configured")
	}

	idx, found, err := LoadIndex(options.Directory)
	if err != nil {
		return nil, err
	}
	mode := options.Mode
	if mode == "" {
		mode = "full"
		if found {
			mode = "incremental"
		}
	}
	if idx.Messages == nil {
		idx.Messages = make(map[string]IndexMessage)
	}
	if idx.Version == 0 {
		idx.Version = IndexVersion
	}
	if idx.CreatedAt == "" {
		idx.CreatedAt = options.StartedAt
	}
	viewerIndex, err := OpenViewerIndex(options.Directory)
	if err != nil {
		return nil, err
	}
	defer viewerIndex.Close()
	viewerMessageKeys, err := viewerIndex.MessageKeys()
	if err != nil {
		return nil, err
	}

	rows, err := ListMessageRows(db, options.AccountIDs)
	if err != nil {
		return nil, err
	}

	result := &RunResult{
		Directory: options.Directory,
		Mode:      mode,
		Total:     len(rows),
	}
	failures := make([]Failure, 0)
	missing := make([]Failure, 0)
	unavailable := make([]Failure, 0)
	progressEmitter := newProgressEmitter(options.EmitProgress, 250*time.Millisecond, time.Now)
	progressEmitter.Emit(Progress{Phase: "running", Stage: ProgressStageChecking, Total: len(rows), Message: "开始核对已有备份"})

	pendingRows := make([]MessageRow, 0, len(rows))
	recoveryMessageStore := message.NewStore(db)
	recoveryFolderStore := folder.NewStore(db)
	processed := 0
	for _, row := range rows {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		key := MessageKey(row)
		existing, indexed := idx.Messages[key]
		if indexed && FileExists(options.Directory, existing.EMLPath) {
			existing.HasAttachments = BoolPtr(row.HasAttachments)
			idx.Messages[key] = existing
			if !viewerMessageKeys[key] {
				if upsertErr := viewerIndex.UpsertMessage(key, existing, ViewerIndexedMessage{}); upsertErr == nil {
					viewerMessageKeys[key] = true
				}
			}
			result.Skipped++
			processed++
			progressEmitter.Emit(progressForRow(row, result, processed, len(rows), 0, 0, ProgressStageChecking, ""))
			continue
		}
		if !row.Selectable {
			_, recoverable, lookupErr := findIndexedMessageFile(options.Directory, idx, row)
			if lookupErr == nil && !recoverable {
				equivalent, equivalentErr := recoveryMessageStore.FindUniqueSelectableEquivalent(row.ID)
				if equivalentErr != nil {
					lookupErr = equivalentErr
				} else if equivalent != nil {
					equivalentFolder, folderErr := recoveryFolderStore.Get(equivalent.FolderID)
					if folderErr != nil {
						lookupErr = folderErr
					} else if equivalentFolder != nil && equivalentFolder.IsSelectable() {
						_, recoverable, lookupErr = findIndexedMessageFile(options.Directory, idx, MessageRow{
							ID:          equivalent.ID,
							AccountID:   equivalent.AccountID,
							FolderID:    equivalent.FolderID,
							UIDValidity: equivalentFolder.UIDValidity,
							UID:         equivalent.UID,
							MessageID:   equivalent.MessageID,
						})
					}
				}
			}
			if lookupErr != nil {
				result.Failed++
				failures = append(failures, FailureFromRow(row, lookupErr))
				processed++
				progressEmitter.Emit(progressForRow(row, result, processed, len(rows), 0, 0, ProgressStageChecking, lookupErr.Error()))
				continue
			}
			if recoverable {
				result.Skipped++
				processed++
				progressEmitter.Emit(progressForRow(row, result, processed, len(rows), 0, 0, ProgressStageChecking, ""))
				continue
			}
			reason := fmt.Errorf("raw message unavailable: hierarchy-only folder has no indexed backup file")
			result.Unavailable++
			unavailable = append(unavailable, FailureFromRow(row, reason))
			processed++
			progressEmitter.Emit(progressForRow(row, result, processed, len(rows), 0, 0, ProgressStageChecking, reason.Error()))
			continue
		}

		pendingRows = append(pendingRows, row)
	}

	progressEmitter.Emit(Progress{
		Phase:        "running",
		Stage:        ProgressStageExporting,
		StageCurrent: 0,
		StageTotal:   len(pendingRows),
		Current:      processed,
		Total:        len(rows),
		Exported:     result.Exported,
		Skipped:      result.Skipped,
		Missing:      result.Missing,
		Unavailable:  result.Unavailable,
		Failed:       result.Failed,
		Message:      "开始导出待备份邮件",
	})

	exportProcessed := 0
	for _, group := range GroupMessageRows(pendingRows) {
		for offset := 0; offset < len(group.Rows); offset += options.RawFetchBatchSize {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			end := offset + options.RawFetchBatchSize
			if end > len(group.Rows) {
				end = len(group.Rows)
			}
			chunk := group.Rows[offset:end]
			rowsByUID := RowsByUID(chunk)
			streamResults, streamFailures, err := options.StreamRawMessages(ctx, group.AccountID, group.FolderID, RowUIDs(chunk), func(uid uint32, body io.Reader) (int64, error) {
				row, ok := rowsByUID[uid]
				if !ok {
					return 0, fmt.Errorf("unexpected backup UID: %d", uid)
				}
				return WriteFileFromReader(options.Directory, MessageRelativePathForRow(row), body)
			})
			if err != nil {
				for _, row := range chunk {
					result.Failed++
					failures = append(failures, FailureFromRow(row, err))
					processed++
					exportProcessed++
					progressEmitter.Emit(progressForRow(row, result, processed, len(rows), exportProcessed, len(pendingRows), ProgressStageExporting, err.Error()))
				}
				continue
			}

			for _, row := range chunk {
				key := MessageKey(row)
				relPath := MessageRelativePathForRow(row)
				streamResult, ok := streamResults[row.UID]
				err := streamFailures[row.UID]
				if err == nil && !ok {
					err = mailSync.RawMessageNotFoundError{UID: row.UID}
				}
				if err != nil {
					if mailSync.IsRawMessageNotFoundError(err) {
						result.Missing++
						missing = append(missing, FailureFromRow(row, err))
					} else {
						result.Failed++
						failures = append(failures, FailureFromRow(row, err))
					}
					processed++
					exportProcessed++
					progressEmitter.Emit(progressForRow(row, result, processed, len(rows), exportProcessed, len(pendingRows), ProgressStageExporting, err.Error()))
					continue
				}

				idx.Messages[key] = IndexMessage{
					AccountID:      row.AccountID,
					AccountEmail:   row.AccountEmail,
					FolderID:       row.FolderID,
					FolderPath:     row.FolderPath,
					UIDValidity:    row.UIDValidity,
					UID:            row.UID,
					MessageID:      row.MessageID,
					Subject:        row.Subject,
					Date:           row.DateRaw,
					EMLPath:        relPath,
					Size:           FileSizeInt(streamResult.BytesWritten),
					HasAttachments: BoolPtr(row.HasAttachments),
					ExportedAt:     time.Now().UTC().Format(time.RFC3339),
				}
				if upsertErr := viewerIndex.UpsertMessageFromFile(options.Directory, key, idx.Messages[key]); upsertErr == nil {
					viewerMessageKeys[key] = true
				}
				result.Exported++
				processed++
				exportProcessed++
				progressEmitter.Emit(progressForRow(row, result, processed, len(rows), exportProcessed, len(pendingRows), ProgressStageExporting, ""))
			}
		}
	}

	run := IndexRun{
		StartedAt:   options.StartedAt,
		FinishedAt:  time.Now().UTC().Format(time.RFC3339),
		Mode:        mode,
		Total:       result.Total,
		Exported:    result.Exported,
		Skipped:     result.Skipped,
		Missing:     result.Missing,
		Unavailable: result.Unavailable,
		Failed:      result.Failed,
	}
	idx.Version = IndexVersion
	idx.UpdatedAt = run.FinishedAt
	idx.LastRun = &run

	if err := SaveIndex(options.Directory, idx); err != nil {
		return nil, err
	}
	reportPath, err := SaveReport(options.Directory, Report{
		IndexRun:            run,
		Directory:           options.Directory,
		MissingMessages:     missing,
		UnavailableMessages: unavailable,
		Failures:            failures,
	})
	if err != nil {
		return nil, err
	}
	result.ReportPath = reportPath
	return result, nil
}

func ListMessageRows(db *database.DB, accountIDs []string) ([]MessageRow, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(accountIDs)), ",")
	args := make([]interface{}, 0, len(accountIDs))
	for _, id := range accountIDs {
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		SELECT
			m.id,
			m.account_id,
			a.email,
			m.folder_id,
			f.path,
			f.name,
			COALESCE(f.uid_validity, 0),
			m.uid,
			COALESCE(m.message_id, ''),
			COALESCE(m.subject, ''),
			COALESCE(m.date, ''),
			COALESCE(m.size, 0),
			COALESCE(m.has_attachments, 0),
			COALESCE(f.selectable, 1)
		FROM messages m
		INNER JOIN accounts a ON a.id = m.account_id
		INNER JOIN folders f ON f.id = m.folder_id
		WHERE m.account_id IN (%s)
		ORDER BY a.order_index ASC, f.path COLLATE NOCASE ASC, m.date ASC, m.uid ASC
	`, placeholders)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages for backup: %w", err)
	}
	defer rows.Close()

	var messages []MessageRow
	for rows.Next() {
		var row MessageRow
		var uid, uidValidity int64
		var size int64
		var hasAttachments bool
		var selectable bool
		var dateRaw sql.NullString
		if err := rows.Scan(
			&row.ID,
			&row.AccountID,
			&row.AccountEmail,
			&row.FolderID,
			&row.FolderPath,
			&row.FolderName,
			&uidValidity,
			&uid,
			&row.MessageID,
			&row.Subject,
			&dateRaw,
			&size,
			&hasAttachments,
			&selectable,
		); err != nil {
			return nil, fmt.Errorf("failed to scan backup message: %w", err)
		}
		row.UID = uint32(uid)
		row.UIDValidity = uint32(uidValidity)
		row.Size = int(size)
		row.HasAttachments = hasAttachments
		row.Selectable = selectable
		if dateRaw.Valid {
			row.DateRaw = dateRaw.String
			row.Date = ParseMessageTime(dateRaw.String)
		}
		if row.FolderPath == "" {
			row.FolderPath = row.FolderName
		}
		messages = append(messages, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate backup messages: %w", err)
	}
	return messages, nil
}

func progressForRow(row MessageRow, result *RunResult, current, total, stageCurrent, stageTotal int, stage, message string) Progress {
	return Progress{
		Phase:        "running",
		Stage:        stage,
		StageCurrent: stageCurrent,
		StageTotal:   stageTotal,
		AccountEmail: row.AccountEmail,
		FolderPath:   row.FolderPath,
		Current:      current,
		Total:        total,
		Exported:     result.Exported,
		Skipped:      result.Skipped,
		Missing:      result.Missing,
		Unavailable:  result.Unavailable,
		Failed:       result.Failed,
		Message:      message,
	}
}

type progressEmitter struct {
	emit        func(Progress)
	minInterval time.Duration
	now         func() time.Time
	lastEmit    time.Time
	lastStage   string
	emitted     bool
}

func newProgressEmitter(emit func(Progress), minInterval time.Duration, now func() time.Time) *progressEmitter {
	return &progressEmitter{emit: emit, minInterval: minInterval, now: now}
}

func (e *progressEmitter) Emit(progress Progress) {
	if e == nil || e.emit == nil {
		return
	}
	now := e.now()
	isFinal := progress.Total >= 0 && progress.Current >= progress.Total
	stageChanged := e.emitted && progress.Stage != e.lastStage
	if e.emitted && !isFinal && !stageChanged && now.Sub(e.lastEmit) < e.minInterval {
		return
	}
	e.emit(progress)
	e.lastEmit = now
	e.lastStage = progress.Stage
	e.emitted = true
}
