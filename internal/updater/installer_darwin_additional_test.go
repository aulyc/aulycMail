//go:build darwin

package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadOneCoversTransportAndFilesystemFailures(t *testing.T) {
	good := []byte("verified payload")
	tests := []struct {
		name        string
		url         string
		client      *http.Client
		destination func(*testing.T) string
		want        string
	}{
		{
			name: "invalid request",
			url:  "://bad",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("transport must not run")
				return nil, nil
			})},
			destination: func(t *testing.T) string { return filepath.Join(t.TempDir(), "download") },
			want:        "create request",
		},
		{
			name: "connection failure",
			url:  "https://github.com/file",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("offline")
			})},
			destination: func(t *testing.T) string { return filepath.Join(t.TempDir(), "download") },
			want:        "connection failed",
		},
		{
			name: "HTTP failure",
			url:  "https://github.com/file",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusBadGateway, Body: http.NoBody, Header: make(http.Header)}, nil
			})},
			destination: func(t *testing.T) string { return filepath.Join(t.TempDir(), "download") },
			want:        "HTTP 502",
		},
		{
			name: "declared oversized",
			url:  "https://github.com/file",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, ContentLength: maxUpdateArtifactSize + 1, Header: make(http.Header)}, nil
			})},
			destination: func(t *testing.T) string { return filepath.Join(t.TempDir(), "download") },
			want:        "file is too large",
		},
		{
			name: "existing destination",
			url:  "https://github.com/file",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(good))), ContentLength: int64(len(good)), Header: make(http.Header)}, nil
			})},
			destination: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "download")
				if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			},
			want: "create destination",
		},
		{
			name: "response read failure",
			url:  "https://github.com/file",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: failingReader{}, ContentLength: -1, Header: make(http.Header)}, nil
			})},
			destination: func(t *testing.T) string { return filepath.Join(t.TempDir(), "download") },
			want:        "read response",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := downloadOne(context.Background(), test.client, test.url, test.destination(t), sha(good), func(int) {})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("downloadOne() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestDownloadOneReportsIncrementalProgress(t *testing.T) {
	payload := []byte("progress payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	var progress []int
	destination := filepath.Join(t.TempDir(), "download")
	if err := downloadOne(context.Background(), server.Client(), server.URL, destination, sha(payload), func(value int) { progress = append(progress, value) }); err != nil {
		t.Fatal(err)
	}
	if len(progress) < 2 || progress[len(progress)-1] != 100 {
		t.Fatalf("progress = %v", progress)
	}
}

func TestValidateProvenanceRejectsMalformedTrustEvidence(t *testing.T) {
	manifest := validManifest()
	base := formalProvenance(manifest)
	var decoded map[string]any
	if err := json.Unmarshal(base, &decoded); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any) []byte
		want   string
	}{
		{name: "malformed", mutate: func(map[string]any) []byte { return []byte(`{`) }, want: "decode release provenance"},
		{name: "build type", mutate: func(value map[string]any) []byte {
			value["buildNumber"] = "40"
			data, _ := json.Marshal(value)
			return data
		}, want: "buildNumber"},
		{name: "dirty", mutate: func(value map[string]any) []byte { value["dirty"] = true; data, _ := json.Marshal(value); return data }, want: "trust claims"},
		{name: "missing submission", mutate: func(value map[string]any) []byte {
			value["notarizationSubmissionId"] = " "
			data, _ := json.Marshal(value)
			return data
		}, want: "notarizationSubmissionId is missing"},
		{name: "missing built time", mutate: func(value map[string]any) []byte {
			delete(value, "builtAt")
			data, _ := json.Marshal(value)
			return data
		}, want: "builtAt is missing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copyValue := make(map[string]any, len(decoded))
			for key, value := range decoded {
				copyValue[key] = value
			}
			if err := validateProvenance(test.mutate(copyValue), manifest); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateProvenance() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPlatformInstallerRejectsEveryVerificationStage(t *testing.T) {
	tests := []struct {
		name  string
		stage string
		want  string
	}{
		{name: "disk verify", stage: "disk-verify", want: "verify update disk image"},
		{name: "stapler", stage: "stapler", want: "validate update notarization ticket"},
		{name: "disk assessment", stage: "disk-assess", want: "assess update disk image"},
		{name: "mount", stage: "mount", want: "mount update disk image"},
		{name: "source metadata", stage: "source-metadata", want: "does not match"},
		{name: "unexpected app", stage: "unexpected-app", want: "unexpected application"},
		{name: "installed path", stage: "installed-path", want: "installed path unavailable"},
		{name: "staging copy", stage: "ditto", want: "stage update beside installed application"},
		{name: "staged signature", stage: "staged-signature", want: "verify staged update"},
		{name: "helper launch", stage: "helper", want: "helper unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
			runner := func(_ context.Context, name string, args ...string) (string, error) {
				if test.stage == "disk-verify" && name == "/usr/bin/hdiutil" && args[0] == "verify" {
					return "", errors.New("verify failed")
				}
				if test.stage == "stapler" && name == "/usr/bin/xcrun" {
					return "", errors.New("stapler failed")
				}
				if test.stage == "disk-assess" && name == "/usr/sbin/spctl" {
					return "", errors.New("assessment failed")
				}
				if name == "/usr/bin/hdiutil" && args[0] == "attach" {
					if test.stage == "mount" {
						return "", errors.New("mount failed")
					}
					mountPoint := args[len(args)-2]
					if err := makeSyntheticApp(filepath.Join(mountPoint, "aulycMail.app")); err != nil {
						return "", err
					}
					if test.stage == "unexpected-app" {
						if err := os.Mkdir(filepath.Join(mountPoint, "Other.app"), 0o700); err != nil {
							return "", err
						}
					}
				}
				if name == "/usr/bin/ditto" {
					if test.stage == "ditto" {
						return "", errors.New("copy failed")
					}
					if err := makeSyntheticApp(args[1]); err != nil {
						return "", err
					}
				}
				if name == "/usr/bin/plutil" {
					plistPath := args[len(args)-1]
					if test.stage == "source-metadata" && strings.Contains(plistPath, string(filepath.Separator)+"mounted"+string(filepath.Separator)) {
						return "wrong", nil
					}
					values := map[string]string{
						"CFBundleIdentifier": bundleIdentifier, "CFBundleShortVersionString": manifest.Version,
						"CFBundleVersion": fmt.Sprint(manifest.BuildNumber), "AULYCSemanticVersion": manifest.Version,
						"AULYCCommitSHA": manifest.Commit, "LSMinimumSystemVersion": minimumSystemTarget,
					}
					return values[args[1]], nil
				}
				if name == "/usr/bin/codesign" && args[0] == "--verify" && test.stage == "staged-signature" && strings.Contains(args[len(args)-1], ".aulycMail-update-") {
					return "", errors.New("signature failed")
				}
				if name == "/usr/bin/codesign" && args[0] == "-dv" {
					return "TeamIdentifier=" + teamIdentifier + "\nAuthority=Developer ID Application: Test\nRuntime Version=14.0\n", nil
				}
				if name == "/usr/bin/lipo" {
					return "arm64\n", nil
				}
				return "", nil
			}
			installer := &platformInstaller{
				run: runner,
				installedPath: func() (string, error) {
					if test.stage == "installed-path" {
						return "", errors.New("installed path unavailable")
					}
					return target, nil
				},
				launchHelper: func(string, string) error {
					if test.stage == "helper" {
						return errors.New("helper unavailable")
					}
					return nil
				},
			}
			err := installer.PrepareAndLaunch(context.Background(), manifest, server.Client(), func(string, int) {})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("PrepareAndLaunch() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestVerifyAppRejectsInvalidDeveloperIdentityAndArchitecture(t *testing.T) {
	manifest := validManifest()
	tests := []struct {
		name  string
		stage string
		want  string
	}{
		{name: "developer identity", stage: "identity", want: "Developer ID identity"},
		{name: "architecture", stage: "architecture", want: "arm64-only"},
		{name: "application assessment", stage: "assessment", want: "assess update application"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appPath := filepath.Join(t.TempDir(), "aulycMail.app")
			if err := makeSyntheticApp(appPath); err != nil {
				t.Fatal(err)
			}
			runner := func(_ context.Context, name string, args ...string) (string, error) {
				if name == "/usr/bin/plutil" {
					values := map[string]string{
						"CFBundleIdentifier": bundleIdentifier, "CFBundleShortVersionString": manifest.Version,
						"CFBundleVersion": fmt.Sprint(manifest.BuildNumber), "AULYCSemanticVersion": manifest.Version,
						"AULYCCommitSHA": manifest.Commit, "LSMinimumSystemVersion": minimumSystemTarget,
					}
					return values[args[1]], nil
				}
				if name == "/usr/bin/codesign" && args[0] == "-dv" {
					if test.stage == "identity" {
						return "TeamIdentifier=OTHER", nil
					}
					return "TeamIdentifier=" + teamIdentifier + "\nAuthority=Developer ID Application: Test\nRuntime Version=14.0\n", nil
				}
				if name == "/usr/bin/lipo" {
					if test.stage == "architecture" {
						return "arm64 x86_64", nil
					}
					return architecture, nil
				}
				if name == "/usr/sbin/spctl" && test.stage == "assessment" {
					return "", errors.New("rejected")
				}
				return "", nil
			}
			if err := verifyApp(context.Background(), appPath, manifest, runner); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("verifyApp() error = %v, want %q", err, test.want)
			}
		})
	}

	root := t.TempDir()
	realApp := filepath.Join(root, "real.app")
	if err := os.Mkdir(realApp, 0o700); err != nil {
		t.Fatal(err)
	}
	symlinkApp := filepath.Join(root, "aulycMail.app")
	if err := os.Symlink(realApp, symlinkApp); err != nil {
		t.Fatal(err)
	}
	if err := verifyApp(context.Background(), symlinkApp, manifest, nil); err == nil {
		t.Fatal("symlink application was accepted")
	}
}
