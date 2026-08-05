//go:build darwin

package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func formalProvenance(manifest Manifest) []byte {
	value := map[string]any{
		"application": "aulycMail", "releaseProfile": releaseProfile, "version": manifest.Version,
		"buildNumber": manifest.BuildNumber, "releaseChannel": releaseChannel, "tag": manifest.Tag,
		"commit": manifest.Commit, "dirty": false, "artifact": manifest.Artifact.File,
		"sha256": manifest.Artifact.SHA256, "architecture": architecture,
		"bundleIdentifier": bundleIdentifier, "teamIdentifier": teamIdentifier,
		"minimumSystemVersion": minimumSystemTarget, "signatureType": "developer-id",
		"hardenedRuntime": true, "notarized": true, "notarizationSubmissionId": "submission",
		"builtAt": "2026-08-04T00:00:00Z", "sourceRepository": sourceRepository,
		"sourceBranch": "main", "sourceRemoteCommit": manifest.Commit,
		"sourceRemoteTagCommit": manifest.Commit, "sourceRemoteVerifiedAt": "2026-08-04T00:00:00Z",
	}
	data, _ := json.Marshal(value)
	return data
}

func TestPlatformInstallerRunsVerifiedStagingPipeline(t *testing.T) {
	dmg := []byte("synthetic signed dmg")
	manifest := validManifest()
	manifest.Artifact.SHA256 = sha(dmg)
	provenance := formalProvenance(manifest)
	manifest.Provenance.SHA256 = sha(provenance)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/provenance":
			_, _ = w.Write(provenance)
		case "/dmg":
			_, _ = w.Write(dmg)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	manifest.Provenance.Downloads = []Download{{Source: "github", URL: server.URL + "/provenance"}}
	manifest.Artifact.Downloads = []Download{{Source: "github", URL: server.URL + "/dmg"}}

	root := t.TempDir()
	target := filepath.Join(root, "Applications", "aulycMail.app")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	launched := false
	runner := func(_ context.Context, name string, args ...string) (string, error) {
		if name == "/usr/bin/hdiutil" && len(args) > 0 && args[0] == "attach" {
			mountPoint := args[len(args)-2]
			if err := makeSyntheticApp(filepath.Join(mountPoint, "aulycMail.app")); err != nil {
				return "", err
			}
		}
		if name == "/usr/bin/ditto" {
			if err := makeSyntheticApp(args[1]); err != nil {
				return "", err
			}
		}
		if name == "/usr/bin/plutil" {
			values := map[string]string{
				"CFBundleIdentifier": bundleIdentifier, "CFBundleShortVersionString": manifest.Version,
				"CFBundleVersion": fmt.Sprint(manifest.BuildNumber), "AULYCSemanticVersion": manifest.Version,
				"AULYCCommitSHA": manifest.Commit, "LSMinimumSystemVersion": minimumSystemTarget,
			}
			return values[args[1]], nil
		}
		if name == "/usr/bin/codesign" && len(args) > 0 && args[0] == "-dv" {
			return "TeamIdentifier=" + teamIdentifier + "\nAuthority=Developer ID Application: Test\nRuntime Version=14.0\n", nil
		}
		if name == "/usr/bin/lipo" {
			return "arm64\n", nil
		}
		return "", nil
	}
	installer := &platformInstaller{
		run:           runner,
		installedPath: func() (string, error) { return target, nil },
		launchHelper: func(actualTarget, staged string) error {
			launched = true
			if actualTarget != target || !strings.Contains(filepath.Base(staged), ".aulycMail-update-") {
				t.Fatalf("unexpected launch paths: %s %s", actualTarget, staged)
			}
			return nil
		},
	}
	var states []string
	if err := installer.PrepareAndLaunch(context.Background(), manifest, server.Client(), func(state string, _ int) { states = append(states, state) }); err != nil {
		t.Fatalf("PrepareAndLaunch() error = %v", err)
	}
	if !launched {
		t.Fatal("replacement helper was not launched")
	}
	if len(states) == 0 || states[len(states)-1] != StateInstalling {
		t.Fatalf("unexpected progress states: %v", states)
	}
}

func makeSyntheticApp(path string) error {
	if err := os.MkdirAll(filepath.Join(path, "Contents", "MacOS"), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(path, "Contents", "Info.plist"), []byte("plist"), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(path, "Contents", "MacOS", "aulycMail"), []byte("binary"), 0o700)
}

func TestDownloadVerifiedFallsBackAndRejectsChecksumMismatch(t *testing.T) {
	good := []byte("good")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/good" {
			_, _ = w.Write(good)
			return
		}
		_, _ = w.Write([]byte("bad"))
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "artifact")
	source, err := downloadVerified(context.Background(), server.Client(), Downloadable{
		SHA256: sha(good), Downloads: []Download{{Source: "github", URL: server.URL + "/bad"}, {Source: "gitee", URL: server.URL + "/good"}},
	}, destination, func(int) {})
	if err != nil || source != "gitee" {
		t.Fatalf("downloadVerified() = %q, %v", source, err)
	}
	if data, _ := os.ReadFile(destination); string(data) != "good" {
		t.Fatalf("downloaded %q", data)
	}

	if _, err := downloadVerified(context.Background(), server.Client(), Downloadable{SHA256: repeatedHex("0"), Downloads: []Download{{Source: "github", URL: server.URL + "/bad"}}}, filepath.Join(t.TempDir(), "bad"), func(int) {}); err == nil {
		t.Fatal("checksum mismatch was accepted")
	}
}

func TestProvenanceAndAppValidationRejectMismatches(t *testing.T) {
	manifest := validManifest()
	provenance := formalProvenance(manifest)
	if err := validateProvenance(provenance, manifest); err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	_ = json.Unmarshal(provenance, &value)
	value["commit"] = strings.Repeat("f", 40)
	changed, _ := json.Marshal(value)
	if err := validateProvenance(changed, manifest); err == nil {
		t.Fatal("mismatched provenance was accepted")
	}
	if err := verifyApp(context.Background(), filepath.Join(t.TempDir(), "missing.app"), manifest, runCommand); err == nil {
		t.Fatal("missing app was accepted")
	}
	if _, err := installedApplicationPath(); err == nil {
		t.Fatal("test binary was mistaken for /Applications/aulycMail.app")
	}
}

func TestCommandAndExecutableHelpers(t *testing.T) {
	if _, err := runCommand(context.Background(), "/usr/bin/true"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCommand(context.Background(), "/usr/bin/false"); err == nil {
		t.Fatal("failing command returned nil")
	}
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	if err := os.WriteFile(source, []byte("helper"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := copyExecutable(source, destination); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(destination); string(data) != "helper" {
		t.Fatalf("copied %q", data)
	}
	if NewInstaller() == nil {
		t.Fatal("NewInstaller returned nil")
	}
}
