package activitylog

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/database"
)

func openTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "activity-log.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func testEntry(id, createdAt, logType, status string) Entry {
	return Entry{
		ID:        id,
		CreatedAt: createdAt,
		Type:      logType,
		Status:    status,
		Title:     "Test activity",
		Summary:   "Test summary",
		Payload:   map[string]any{},
	}
}

func TestStoreAppendAssignsDefaultsAndRoundTripsPayload(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	now := time.Date(2026, 7, 11, 1, 21, 7, 123, time.UTC)
	store.now = func() time.Time { return now }

	entry := Entry{
		Type:    TypeBackup,
		Status:  StatusSuccess,
		Title:   "Backup email",
		Summary: "Incremental export complete",
		Detail:  "No errors",
		Payload: map[string]any{
			"directory": "/tmp/mail-backup",
			"mode":      "incremental",
			"added":     12,
		},
	}
	if err := store.Append(&entry); err != nil {
		t.Fatal(err)
	}
	if entry.ID == "" {
		t.Fatal("Append did not assign an ID")
	}
	if want := now.Format(time.RFC3339Nano); entry.CreatedAt != want {
		t.Fatalf("CreatedAt = %q, want %q", entry.CreatedAt, want)
	}

	got, err := store.Latest(TypeBackup, "/tmp/mail-backup")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("Latest returned nil")
	}
	if got.ID != entry.ID || got.Detail != entry.Detail {
		t.Fatalf("Latest = %#v, want appended entry %#v", got, entry)
	}
	if got.Payload["mode"] != "incremental" || got.Payload["directory"] != "/tmp/mail-backup" {
		t.Fatalf("payload did not round trip: %#v", got.Payload)
	}
}

func TestStoreListFiltersAndPaginates(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	store.now = func() time.Time { return time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC) }

	entries := []Entry{
		testEntry("sync-success", "2026-07-11T08:00:00Z", TypeSync, StatusSuccess),
		testEntry("sync-local-next-day", "2026-07-11T20:00:00Z", TypeSync, StatusSuccess),
		testEntry("backup-partial", "2026-07-11T09:00:00Z", TypeBackup, StatusPartial),
		testEntry("backup-failed", "2026-07-11T10:00:00Z", TypeBackup, StatusFailed),
		testEntry("backup-other-day", "2026-07-10T10:00:00Z", TypeBackup, StatusFailed),
	}
	for i := range entries {
		entries[i].Payload["directory"] = "/tmp/a"
		if err := store.Append(&entries[i]); err != nil {
			t.Fatal(err)
		}
	}

	page, err := store.List(Query{
		Type:        TypeBackup,
		ProblemOnly: true,
		Date:        "2026-07-11",
		Directory:   "/tmp/a",
		Limit:       1,
		Offset:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Entries) != 1 {
		t.Fatalf("page = %#v, want total 2 and one entry", page)
	}
	if page.Entries[0].ID != "backup-partial" {
		t.Fatalf("entry = %q, want backup-partial", page.Entries[0].ID)
	}

	localDay, err := store.List(Query{
		Type:                  TypeSync,
		Date:                  "2026-07-12",
		TimezoneOffsetMinutes: -8 * 60,
		Limit:                 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if localDay.Total != 1 || localDay.Entries[0].ID != "sync-local-next-day" {
		t.Fatalf("local-day page = %#v, want sync-local-next-day", localDay)
	}

	if _, err := store.List(Query{Date: "11/07/2026"}); err == nil {
		t.Fatal("List accepted an invalid ISO date")
	}
}

func TestStoreClearUsesCurrentFilters(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	store.now = func() time.Time { return time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC) }

	entries := []Entry{
		testEntry("sync-failed", "2026-07-11T08:00:00Z", TypeSync, StatusFailed),
		testEntry("backup-partial", "2026-07-11T09:00:00Z", TypeBackup, StatusPartial),
		testEntry("backup-success", "2026-07-11T10:00:00Z", TypeBackup, StatusSuccess),
	}
	for i := range entries {
		if err := store.Append(&entries[i]); err != nil {
			t.Fatal(err)
		}
	}

	cleared, err := store.Clear(Query{Type: TypeBackup, ProblemOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if cleared != 1 {
		t.Fatalf("cleared = %d, want 1", cleared)
	}
	page, err := store.List(Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 {
		t.Fatalf("remaining total = %d, want 2", page.Total)
	}
}

func TestStorePruneAppliesAgeAndPerTypeDayCountLimits(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	insert := func(id, logType string, createdAt time.Time) {
		t.Helper()
		_, err := db.Exec(`
			INSERT INTO activity_logs
				(id, created_at, type, status, title, summary, payload_json)
			VALUES (?, ?, ?, ?, 'Title', 'Summary', '{}')
		`, id, createdAt.UTC().Format(time.RFC3339Nano), logType, StatusSuccess)
		if err != nil {
			t.Fatal(err)
		}
	}

	insert("expired", TypeSync, now.Add(-RetentionPeriod-time.Second))
	for i := 0; i < MaxEntriesPerTypePerDay+5; i++ {
		insert(fmt.Sprintf("recent-%04d", i), TypeSync, now.Add(-time.Duration(i)*time.Second))
	}
	insert("previous-day-sync", TypeSync, now.Add(-24*time.Hour))
	insert("same-day-backup", TypeBackup, now.Add(-time.Minute))
	if err := store.Prune(); err != nil {
		t.Fatal(err)
	}

	var total, expired, previousDay, backup int
	if err := db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM activity_logs WHERE id = 'expired'").Scan(&expired); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM activity_logs WHERE id = 'previous-day-sync'").Scan(&previousDay); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM activity_logs WHERE id = 'same-day-backup'").Scan(&backup); err != nil {
		t.Fatal(err)
	}
	wantTotal := MaxEntriesPerTypePerDay + 2
	if total != wantTotal || expired != 0 || previousDay != 1 || backup != 1 {
		t.Fatalf("after prune: total=%d expired=%d previousDay=%d backup=%d, want total=%d expired=0 previousDay=1 backup=1", total, expired, previousDay, backup, wantTotal)
	}
}

func TestStoreAppendValidatesRequiredFields(t *testing.T) {
	store := NewStore(openTestDB(t))
	tests := []struct {
		name  string
		entry Entry
		want  error
	}{
		{name: "type", entry: Entry{Status: StatusSuccess, Title: "x", Summary: "x"}, want: ErrTypeRequired},
		{name: "status", entry: Entry{Type: TypeSync, Status: "unknown", Title: "x", Summary: "x"}, want: ErrInvalidStatus},
		{name: "title", entry: Entry{Type: TypeSync, Status: StatusSuccess, Summary: "x"}, want: ErrTitleRequired},
		{name: "summary", entry: Entry{Type: TypeSync, Status: StatusSuccess, Title: "x"}, want: ErrSummaryRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := tt.entry
			if err := store.Append(&entry); !errors.Is(err, tt.want) {
				t.Fatalf("Append error = %v, want %v", err, tt.want)
			}
		})
	}
}
