package backup

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"aulyc.local/aulycmail/internal/database"
	mailSync "aulyc.local/aulycmail/internal/sync"
)

type Progress struct {
	Phase        string
	AccountEmail string
	FolderPath   string
	Current      int
	Total        int
	Exported     int
	Skipped      int
	Missing      int
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
	Directory  string
	Mode       string
	Total      int
	Exported   int
	Skipped    int
	Missing    int
	Failed     int
	ReportPath string
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
	emitBackupProgress(options.EmitProgress, Progress{Phase: "running", Total: len(rows), Message: "开始备份"})

	pendingRows := make([]MessageRow, 0, len(rows))
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
			if !viewerIndex.HasMessage(key) {
				_ = viewerIndex.UpsertMessage(key, existing, ViewerIndexedMessage{})
			}
			result.Skipped++
			processed++
			emitBackupProgress(options.EmitProgress, progressForRow(row, result, processed, len(rows), ""))
			continue
		}

		pendingRows = append(pendingRows, row)
	}

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
					emitBackupProgress(options.EmitProgress, progressForRow(row, result, processed, len(rows), err.Error()))
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
					emitBackupProgress(options.EmitProgress, progressForRow(row, result, processed, len(rows), err.Error()))
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
				_ = viewerIndex.UpsertMessageFromFile(options.Directory, key, idx.Messages[key])
				result.Exported++
				processed++
				emitBackupProgress(options.EmitProgress, progressForRow(row, result, processed, len(rows), ""))
			}
		}
	}

	run := IndexRun{
		StartedAt:  options.StartedAt,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Mode:       mode,
		Total:      result.Total,
		Exported:   result.Exported,
		Skipped:    result.Skipped,
		Missing:    result.Missing,
		Failed:     result.Failed,
	}
	idx.Version = IndexVersion
	idx.UpdatedAt = run.FinishedAt
	idx.LastRun = &run

	if err := SaveIndex(options.Directory, idx); err != nil {
		return nil, err
	}
	reportPath, err := SaveReport(options.Directory, Report{
		IndexRun:        run,
		Directory:       options.Directory,
		MissingMessages: missing,
		Failures:        failures,
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
			COALESCE(m.has_attachments, 0)
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
		); err != nil {
			return nil, fmt.Errorf("failed to scan backup message: %w", err)
		}
		row.UID = uint32(uid)
		row.UIDValidity = uint32(uidValidity)
		row.Size = int(size)
		row.HasAttachments = hasAttachments
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

func progressForRow(row MessageRow, result *RunResult, current, total int, message string) Progress {
	return Progress{
		Phase:        "running",
		AccountEmail: row.AccountEmail,
		FolderPath:   row.FolderPath,
		Current:      current,
		Total:        total,
		Exported:     result.Exported,
		Skipped:      result.Skipped,
		Missing:      result.Missing,
		Failed:       result.Failed,
		Message:      message,
	}
}

func emitBackupProgress(emit func(Progress), progress Progress) {
	if emit != nil {
		emit(progress)
	}
}
