//go:build darwin

package platform

import (
	"os"
	"strings"
	"testing"
)

func TestStatusItemNativeMenuDeclaresIconGroups(t *testing.T) {
	sourceBytes, err := os.ReadFile("statusitem_darwin.m")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	sequence := []string{
		`@"envelope.open"`,
		`[menu addItem:[NSMenuItem separatorItem]]`,
		`@"gearshape"`,
		`@"arrow.triangle.2.circlepath"`,
		`[menu addItem:[NSMenuItem separatorItem]]`,
		`@"power"`,
	}

	cursor := 0
	for _, item := range sequence {
		index := strings.Index(source[cursor:], item)
		if index < 0 {
			t.Fatalf("native status menu is missing ordered item %q", item)
		}
		cursor += index + len(item)
	}
	if !strings.Contains(source, "configurationWithPointSize:14") || !strings.Contains(source, "[image setTemplate:YES]") {
		t.Fatal("native status menu icons must use aligned 14-point template symbols")
	}
}
