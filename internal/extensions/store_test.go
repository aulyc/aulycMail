package extensions

import (
	"testing"
)

func TestOpenStore_AppliesMigrations(t *testing.T) {
	dir := t.TempDir()
	migs := []Migration{
		{Version: 1, SQL: `CREATE TABLE foo (id INTEGER PRIMARY KEY, name TEXT)`},
		{Version: 2, SQL: `CREATE TABLE bar (id INTEGER PRIMARY KEY, value TEXT)`},
	}

	s, err := OpenStore(dir, "testext", migs)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}

	// Verify both tables exist by inserting and querying
	if _, err := s.DB().Exec(`INSERT INTO foo (name) VALUES (?)`, "hello"); err != nil {
		t.Fatalf("insert into foo: %v", err)
	}
	if _, err := s.DB().Exec(`INSERT INTO bar (value) VALUES (?)`, "world"); err != nil {
		t.Fatalf("insert into bar: %v", err)
	}

	var migCount int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM migrations`).Scan(&migCount); err != nil {
		t.Fatalf("query migrations: %v", err)
	}
	if migCount != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", migCount)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopen with same migrations — should be idempotent
	s2, err := OpenStore(dir, "testext", migs)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer s2.Close()

	var foundName string
	if err := s2.DB().QueryRow(`SELECT name FROM foo LIMIT 1`).Scan(&foundName); err != nil {
		t.Fatalf("query foo after reopen: %v", err)
	}
	if foundName != "hello" {
		t.Fatalf("expected name 'hello', got %q", foundName)
	}
}

func TestOpenStore_EmptyNameRejected(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenStore(dir, "", nil); err == nil {
		t.Fatal("expected error for empty extension name")
	}
}
