package app

import "testing"

func TestVersionLabel(t *testing.T) {
	oldVersion, oldBuild := Version, BuildNumber
	t.Cleanup(func() {
		Version, BuildNumber = oldVersion, oldBuild
	})

	Version = "0.3.92-dev+abcdef1"
	BuildNumber = "0"
	if got := VersionLabel(); got != Version {
		t.Fatalf("VersionLabel() = %q, want %q", got, Version)
	}

	Version = "0.3.92-rc.1"
	BuildNumber = "128"
	if got, want := VersionLabel(), "0.3.92-rc.1 (build 128)"; got != want {
		t.Fatalf("VersionLabel() = %q, want %q", got, want)
	}
}
