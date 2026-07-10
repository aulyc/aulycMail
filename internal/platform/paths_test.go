package platform

import (
	"strings"
	"testing"
)

func TestGetPaths(t *testing.T) {
	paths, err := GetPaths()
	if err != nil {
		t.Fatalf("GetPaths() returned error: %v", err)
	}
	if paths == nil {
		t.Fatal("GetPaths() returned nil")
	}
	if paths.Config == "" {
		t.Error("GetPaths().Config is empty")
	}
	if paths.Data == "" {
		t.Error("GetPaths().Data is empty")
	}
	if paths.Cache == "" {
		t.Error("GetPaths().Cache is empty")
	}
}

func TestDatabasePath(t *testing.T) {
	paths, err := GetPaths()
	if err != nil {
		t.Fatalf("GetPaths() returned error: %v", err)
	}
	dbPath := paths.DatabasePath()
	if !strings.HasSuffix(dbPath, "aulycmail.db") {
		t.Errorf("DatabasePath() = %q, want suffix 'aulycmail.db'", dbPath)
	}
}
