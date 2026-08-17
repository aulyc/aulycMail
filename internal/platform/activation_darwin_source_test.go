//go:build darwin

package platform

import (
	"os"
	"strings"
	"testing"
)

func TestBackgroundHideWaitsForFullScreenExit(t *testing.T) {
	sourceBytes, err := os.ReadFile("activation_darwin.m")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)

	for _, required := range []string{
		"NSWindowStyleMaskFullScreen",
		"NSWindowDidExitFullScreenNotification",
		"backgroundWindowDidExitFullScreen:",
		"gBackgroundWindow = window",
		"[window toggleFullScreen:nil]",
		"[window orderOut:nil]",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("native background hide is missing %q", required)
		}
	}

	showStart := strings.Index(source, "void showWindowFromBackground(void)")
	if showStart < 0 {
		t.Fatal("native background show helper is missing")
	}
	showSource := source[showStart:]
	cancel := strings.Index(showSource, "cancelPendingBackgroundHide")
	show := strings.Index(showSource, "makeKeyAndOrderFront")
	if cancel < 0 || show < 0 || cancel > show {
		t.Fatal("background show must cancel the delayed full-screen hide before showing the window")
	}
}
