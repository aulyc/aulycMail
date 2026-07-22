package sync

import (
	"reflect"
	"testing"
)

func TestChangedFolderPathsUsesRemoteIdentityAndCounts(t *testing.T) {
	previous := map[string]RemoteFolderSnapshot{
		"INBOX": {UIDValidity: 10, UIDNext: 21, Messages: 20, Unseen: 2, HighestModSeq: 100},
		"Sent":  {UIDValidity: 20, UIDNext: 31, Messages: 30, Unseen: 0, HighestModSeq: 200},
	}
	current := map[string]RemoteFolderSnapshot{
		"INBOX":   {UIDValidity: 10, UIDNext: 22, Messages: 21, Unseen: 3, HighestModSeq: 101},
		"Sent":    previous["Sent"],
		"Archive": {UIDValidity: 30, UIDNext: 1},
	}

	got := ChangedFolderPaths(previous, current)
	want := map[string]bool{"INBOX": true, "Archive": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedFolderPaths() = %v, want %v", got, want)
	}
}

func TestChangedFolderPathsTreatsRemovedFolderAsChange(t *testing.T) {
	previous := map[string]RemoteFolderSnapshot{
		"INBOX": {UIDValidity: 10, UIDNext: 21, Messages: 20},
		"Old":   {UIDValidity: 40, UIDNext: 9, Messages: 8},
	}
	current := map[string]RemoteFolderSnapshot{
		"INBOX": previous["INBOX"],
	}

	got := ChangedFolderPaths(previous, current)
	if !got["Old"] {
		t.Fatalf("ChangedFolderPaths() = %v, want removed path marked changed", got)
	}
}
