package app

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratedBindingsExcludeInternalOnlyMethods(t *testing.T) {
	bindings, err := os.ReadFile("../frontend/wailsjs/go/app/App.js")
	if err != nil {
		t.Fatalf("read generated App bindings: %v", err)
	}

	forbidden := []string{
		"DownloadAttachment",
		"FindLocalMessageIDs",
		"GetShowTitleBar",
		"GetSpecialFolder",
		"MoveMessagesToFolder",
		"Preflight",
		"RunEmailBackup",
		"SetShowTitleBar",
		"ShowWindow",
		"SyncFolders",
		"SyncPendingDrafts",
	}
	for _, method := range forbidden {
		if strings.Contains(string(bindings), "export function "+method+"(") {
			t.Errorf("generated Wails surface still exports internal-only method %s", method)
		}
	}
}
