package sync

import "testing"

func TestDiffMessageUIDs(t *testing.T) {
	t.Parallel()

	result := diffMessageUIDs(
		[]uint32{1, 2, 2, 4},
		[]uint32{2, 3, 3, 5},
	)

	if result.Added != 2 {
		t.Fatalf("Added = %d, want 2", result.Added)
	}
	if result.Removed != 2 {
		t.Fatalf("Removed = %d, want 2", result.Removed)
	}
}

func TestDiffMessageUIDsNoChanges(t *testing.T) {
	t.Parallel()

	result := diffMessageUIDs([]uint32{1, 2}, []uint32{2, 1})
	if result != (MessageSyncResult{}) {
		t.Fatalf("result = %+v, want zero value", result)
	}
}
