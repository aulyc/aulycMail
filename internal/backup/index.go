package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const IndexVersion = 1

type Index struct {
	Version   int                     `json:"version"`
	CreatedAt string                  `json:"createdAt"`
	UpdatedAt string                  `json:"updatedAt"`
	Messages  map[string]IndexMessage `json:"messages"`
	LastRun   *IndexRun               `json:"lastRun,omitempty"`
}

type IndexMessage struct {
	AccountID      string `json:"accountId"`
	AccountEmail   string `json:"accountEmail"`
	FolderID       string `json:"folderId"`
	FolderPath     string `json:"folderPath"`
	UIDValidity    uint32 `json:"uidValidity"`
	UID            uint32 `json:"uid"`
	MessageID      string `json:"messageId,omitempty"`
	Subject        string `json:"subject,omitempty"`
	Date           string `json:"date,omitempty"`
	EMLPath        string `json:"emlPath"`
	Size           int    `json:"size"`
	HasAttachments *bool  `json:"hasAttachments,omitempty"`
	ExportedAt     string `json:"exportedAt"`
}

type IndexRun struct {
	StartedAt   string `json:"startedAt"`
	FinishedAt  string `json:"finishedAt"`
	Mode        string `json:"mode"`
	Total       int    `json:"total"`
	Exported    int    `json:"exported"`
	Skipped     int    `json:"skipped"`
	Missing     int    `json:"missing,omitempty"`
	Unavailable int    `json:"unavailable,omitempty"`
	Failed      int    `json:"failed"`
}

type Failure struct {
	AccountEmail string `json:"accountEmail"`
	FolderPath   string `json:"folderPath"`
	UID          uint32 `json:"uid"`
	Subject      string `json:"subject,omitempty"`
	Error        string `json:"error"`
}

type Report struct {
	IndexRun
	Directory           string    `json:"directory"`
	MissingMessages     []Failure `json:"missingMessages,omitempty"`
	UnavailableMessages []Failure `json:"unavailableMessages,omitempty"`
	Failures            []Failure `json:"failures,omitempty"`
}

type MessageRow struct {
	ID             string
	AccountID      string
	AccountEmail   string
	FolderID       string
	FolderPath     string
	FolderName     string
	UIDValidity    uint32
	UID            uint32
	MessageID      string
	Subject        string
	Date           time.Time
	DateRaw        string
	Size           int
	HasAttachments bool
	Selectable     bool
}

type MessageGroup struct {
	AccountID string
	FolderID  string
	Rows      []MessageRow
}

func LoadIndex(directory string) (*Index, bool, error) {
	path := IndexPath(directory)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Index{Version: IndexVersion, Messages: map[string]IndexMessage{}}, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to read backup index: %w", err)
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, false, fmt.Errorf("failed to parse backup index: %w", err)
	}
	if idx.Messages == nil {
		idx.Messages = map[string]IndexMessage{}
	}
	return &idx, true, nil
}

func SaveIndex(directory string, idx *Index) error {
	path := IndexPath(directory)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create backup metadata directory: %w", err)
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode backup index: %w", err)
	}
	return WriteFileAtomic(path, data, 0600)
}

func SaveReport(directory string, report Report) (string, error) {
	reportDir := filepath.Join(directory, ".aulycmail-backup", "reports")
	if err := os.MkdirAll(reportDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create backup report directory: %w", err)
	}
	name := time.Now().UTC().Format("20060102-150405") + ".json"
	path := filepath.Join(reportDir, name)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode backup report: %w", err)
	}
	if err := WriteFileAtomic(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func FormatRunResult(run IndexRun) string {
	if run.Unavailable > 0 {
		return fmt.Sprintf("%d exported, %d skipped, %d missing, %d unavailable, %d failed", run.Exported, run.Skipped, run.Missing, run.Unavailable, run.Failed)
	}
	if run.Missing > 0 {
		return fmt.Sprintf("%d exported, %d skipped, %d missing, %d failed", run.Exported, run.Skipped, run.Missing, run.Failed)
	}
	return fmt.Sprintf("%d exported, %d skipped, %d failed", run.Exported, run.Skipped, run.Failed)
}

func GroupMessageRows(rows []MessageRow) []MessageGroup {
	groups := make([]MessageGroup, 0)
	indexByKey := make(map[string]int)
	for _, row := range rows {
		key := row.AccountID + "\x00" + row.FolderID
		index, ok := indexByKey[key]
		if !ok {
			index = len(groups)
			indexByKey[key] = index
			groups = append(groups, MessageGroup{
				AccountID: row.AccountID,
				FolderID:  row.FolderID,
			})
		}
		groups[index].Rows = append(groups[index].Rows, row)
	}
	return groups
}

func RowUIDs(rows []MessageRow) []uint32 {
	uids := make([]uint32, 0, len(rows))
	for _, row := range rows {
		if row.UID == 0 {
			continue
		}
		uids = append(uids, row.UID)
	}
	return uids
}

func RowsByUID(rows []MessageRow) map[uint32]MessageRow {
	byUID := make(map[uint32]MessageRow, len(rows))
	for _, row := range rows {
		if row.UID == 0 {
			continue
		}
		byUID[row.UID] = row
	}
	return byUID
}

func MessageKey(row MessageRow) string {
	return fmt.Sprintf("%s:%s:%d:%d", row.AccountID, row.FolderID, row.UIDValidity, row.UID)
}

func MessageRelativePathForRow(row MessageRow) string {
	return MessageRelativePath(row.AccountEmail, row.FolderPath, row.Subject, row.Date, row.UIDValidity, row.UID)
}

func BoolPtr(value bool) *bool {
	return &value
}

func ParseMessageTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05",
		time.RFC1123Z,
		time.RFC1123,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func FailureFromRow(row MessageRow, err error) Failure {
	return Failure{
		AccountEmail: row.AccountEmail,
		FolderPath:   row.FolderPath,
		UID:          row.UID,
		Subject:      row.Subject,
		Error:        err.Error(),
	}
}
