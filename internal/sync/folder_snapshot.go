package sync

// RemoteFolderSnapshot contains only compact server-side state. Comparing two
// successful probes is safer than comparing against local folder counts, which
// are updated during both STATUS and message synchronization.
type RemoteFolderSnapshot struct {
	UIDValidity   uint32
	UIDNext       uint32
	Messages      uint32
	Unseen        uint32
	HighestModSeq uint64
	NoSelect      bool
	Subscribed    bool
}

type FolderSyncResult struct {
	Snapshots map[string]RemoteFolderSnapshot
	Complete  bool
}

// ChangedFolderPaths returns added, removed, or remotely changed folder paths.
func ChangedFolderPaths(previous, current map[string]RemoteFolderSnapshot) map[string]bool {
	changed := make(map[string]bool)
	for path, snapshot := range current {
		if prior, ok := previous[path]; !ok || prior != snapshot {
			changed[path] = true
		}
	}
	for path := range previous {
		if _, ok := current[path]; !ok {
			changed[path] = true
		}
	}
	return changed
}
