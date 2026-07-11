// Package activitylog stores durable records of background application work.
package activitylog

import "time"

const (
	TypeSync   = "sync"
	TypeBackup = "backup"

	StatusSuccess   = "success"
	StatusPartial   = "partial"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Entry is one durable activity record. Payload contains type-specific fields
// such as accountEmail, mode, counts, and directory.
type Entry struct {
	ID        string         `json:"id"`
	CreatedAt string         `json:"createdAt"`
	Type      string         `json:"type"`
	Status    string         `json:"status"`
	Title     string         `json:"title"`
	Summary   string         `json:"summary"`
	Detail    string         `json:"detail,omitempty"`
	Payload   map[string]any `json:"payload"`
}

// Query selects activity records. ProblemOnly includes partial and failed
// records. Date is an ISO calendar date (YYYY-MM-DD).
type Query struct {
	Type                  string `json:"type,omitempty"`
	ProblemOnly           bool   `json:"problemOnly,omitempty"`
	Date                  string `json:"date,omitempty"`
	TimezoneOffsetMinutes int    `json:"timezoneOffsetMinutes,omitempty"`
	Directory             string `json:"directory,omitempty"`
	Limit                 int    `json:"limit,omitempty"`
	Offset                int    `json:"offset,omitempty"`
}

// Page is a paginated activity-log response.
type Page struct {
	Entries []Entry `json:"entries"`
	Total   int     `json:"total"`
}

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
	MaxEntries      = 1000
	RetentionPeriod = 30 * 24 * time.Hour
)
