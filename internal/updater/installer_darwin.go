//go:build darwin

package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const maxUpdateArtifactSize int64 = 2 << 30

type commandRunner func(context.Context, string, ...string) (string, error)

type platformInstaller struct {
	run           commandRunner
	installedPath func() (string, error)
	launchHelper  func(string, string) error
}

func newPlatformInstaller() Installer {
	return &platformInstaller{run: runCommand, installedPath: installedApplicationPath, launchHelper: launchApplyHelper}
}

func (p *platformInstaller) PrepareAndLaunch(ctx context.Context, manifest Manifest, client *http.Client, progress func(string, int)) error {
	temporaryRoot, err := os.MkdirTemp("", "aulycmail-update-")
	if err != nil {
		return fmt.Errorf("create update workspace: %w", err)
	}
	defer os.RemoveAll(temporaryRoot)

	provenancePath := filepath.Join(temporaryRoot, manifest.Provenance.File)
	if _, err := downloadVerified(ctx, client, manifest.Provenance, provenancePath, func(percent int) { progress(StateDownloading, min(percent/10, 10)) }); err != nil {
		return fmt.Errorf("download release provenance: %w", err)
	}
	provenanceData, err := os.ReadFile(provenancePath)
	if err != nil {
		return fmt.Errorf("read release provenance: %w", err)
	}
	if err := validateProvenance(provenanceData, manifest); err != nil {
		return err
	}

	dmgPath := filepath.Join(temporaryRoot, manifest.Artifact.File)
	if _, err := downloadVerified(ctx, client, manifest.Artifact, dmgPath, func(percent int) { progress(StateDownloading, 10+percent*7/10) }); err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	progress(StateVerifying, 82)
	if _, err := p.run(ctx, "/usr/bin/hdiutil", "verify", dmgPath); err != nil {
		return fmt.Errorf("verify update disk image: %w", err)
	}
	if _, err := p.run(ctx, "/usr/bin/xcrun", "stapler", "validate", dmgPath); err != nil {
		return fmt.Errorf("validate update notarization ticket: %w", err)
	}
	if _, err := p.run(ctx, "/usr/sbin/spctl", "--assess", "--type", "open", "--context", "context:primary-signature", "--verbose=4", dmgPath); err != nil {
		return fmt.Errorf("assess update disk image: %w", err)
	}

	mountPoint := filepath.Join(temporaryRoot, "mounted")
	if err := os.Mkdir(mountPoint, 0o700); err != nil {
		return fmt.Errorf("create mount point: %w", err)
	}
	if _, err := p.run(ctx, "/usr/bin/hdiutil", "attach", "-readonly", "-nobrowse", "-mountpoint", mountPoint, dmgPath); err != nil {
		return fmt.Errorf("mount update disk image: %w", err)
	}
	defer func() { _, _ = p.run(context.Background(), "/usr/bin/hdiutil", "detach", mountPoint) }()

	sourceApp := filepath.Join(mountPoint, "aulycMail.app")
	if err := verifyApp(ctx, sourceApp, manifest, p.run); err != nil {
		return err
	}
	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		return fmt.Errorf("inspect update disk image: %w", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".app") && entry.Name() != "aulycMail.app" {
			return fmt.Errorf("update disk image contains an unexpected application")
		}
	}

	targetApp, err := p.installedPath()
	if err != nil {
		return err
	}
	stagedApp := filepath.Join(filepath.Dir(targetApp), ".aulycMail-update-"+uuid.NewString()+".app")
	if _, err := p.run(ctx, "/usr/bin/ditto", sourceApp, stagedApp); err != nil {
		return fmt.Errorf("stage update beside installed application: %w", err)
	}
	cleanupStaged := true
	defer func() {
		if cleanupStaged {
			_ = os.RemoveAll(stagedApp)
		}
	}()
	progress(StateVerifying, 94)
	if err := verifyApp(ctx, stagedApp, manifest, p.run); err != nil {
		return fmt.Errorf("verify staged update: %w", err)
	}

	if err := p.launchHelper(targetApp, stagedApp); err != nil {
		return err
	}
	cleanupStaged = false
	progress(StateInstalling, 100)
	return nil
}

func downloadVerified(ctx context.Context, client *http.Client, item Downloadable, destination string, progress func(int)) (string, error) {
	var failures []string
	for _, candidate := range item.Downloads {
		if err := downloadOne(ctx, client, candidate.URL, destination, item.SHA256, progress); err != nil {
			failures = append(failures, candidate.Source+": "+err.Error())
			_ = os.Remove(destination)
			continue
		}
		return candidate.Source, nil
	}
	return "", fmt.Errorf("all download sources failed: %s", strings.Join(failures, "; "))
}

func downloadOne(ctx context.Context, client *http.Client, sourceURL, destination, expectedSHA string, progress func(int)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return fmt.Errorf("create request")
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("connection failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxUpdateArtifactSize {
		return fmt.Errorf("file is too large")
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	hash := sha256.New()
	reader := io.LimitReader(response.Body, maxUpdateArtifactSize+1)
	buffer := make([]byte, 256<<10)
	var written int64
	for {
		count, readErr := reader.Read(buffer)
		if count > 0 {
			written += int64(count)
			if written > maxUpdateArtifactSize {
				_ = file.Close()
				return fmt.Errorf("file is too large")
			}
			if _, err := file.Write(buffer[:count]); err != nil {
				_ = file.Close()
				return fmt.Errorf("write destination: %w", err)
			}
			_, _ = hash.Write(buffer[:count])
			if response.ContentLength > 0 {
				progress(int(written * 100 / response.ContentLength))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = file.Close()
			return fmt.Errorf("read response: %w", readErr)
		}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync destination: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close destination: %w", err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expectedSHA {
		return fmt.Errorf("SHA-256 mismatch")
	}
	progress(100)
	return nil
}

func validateProvenance(data []byte, manifest Manifest) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode release provenance: %w", err)
	}
	requireString := func(key, expected string) error {
		actual, ok := value[key].(string)
		if !ok || actual != expected {
			return fmt.Errorf("release provenance %s does not match", key)
		}
		return nil
	}
	for key, expected := range map[string]string{
		"application": "aulycMail", "releaseProfile": releaseProfile, "version": manifest.Version,
		"releaseChannel": releaseChannel, "tag": manifest.Tag, "commit": manifest.Commit,
		"artifact": manifest.Artifact.File, "sha256": manifest.Artifact.SHA256,
		"architecture": architecture, "bundleIdentifier": bundleIdentifier,
		"teamIdentifier": teamIdentifier, "minimumSystemVersion": minimumSystemTarget,
		"signatureType": "developer-id", "sourceRepository": sourceRepository,
		"sourceBranch": "main", "sourceRemoteCommit": manifest.Commit, "sourceRemoteTagCommit": manifest.Commit,
	} {
		if err := requireString(key, expected); err != nil {
			return err
		}
	}
	build, ok := value["buildNumber"].(json.Number)
	if !ok || build.String() != strconv.Itoa(manifest.BuildNumber) {
		return fmt.Errorf("release provenance buildNumber does not match")
	}
	if value["dirty"] != false || value["hardenedRuntime"] != true || value["notarized"] != true {
		return fmt.Errorf("release provenance trust claims are invalid")
	}
	for _, key := range []string{"notarizationSubmissionId", "builtAt", "sourceRemoteVerifiedAt"} {
		actual, ok := value[key].(string)
		if !ok || strings.TrimSpace(actual) == "" {
			return fmt.Errorf("release provenance %s is missing", key)
		}
	}
	return nil
}

func verifyApp(ctx context.Context, appPath string, manifest Manifest, run commandRunner) error {
	info, err := os.Lstat(appPath)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("update does not contain a regular aulycMail.app")
	}
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	checks := map[string]string{
		"CFBundleIdentifier":         bundleIdentifier,
		"CFBundleShortVersionString": manifest.Version,
		"CFBundleVersion":            strconv.Itoa(manifest.BuildNumber),
		"AULYCSemanticVersion":       manifest.Version,
		"AULYCCommitSHA":             manifest.Commit,
		"LSMinimumSystemVersion":     minimumSystemTarget,
	}
	for key, expected := range checks {
		output, err := run(ctx, "/usr/bin/plutil", "-extract", key, "raw", "-o", "-", plistPath)
		if err != nil || strings.TrimSpace(output) != expected {
			return fmt.Errorf("update application %s does not match", key)
		}
	}
	executable := filepath.Join(appPath, "Contents", "MacOS", "aulycMail")
	if _, err := run(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", "--verbose=2", appPath); err != nil {
		return fmt.Errorf("verify update application signature: %w", err)
	}
	details, err := run(ctx, "/usr/bin/codesign", "-dv", "--verbose=4", appPath)
	if err != nil || !strings.Contains(details, "TeamIdentifier="+teamIdentifier) || !strings.Contains(details, "Authority=Developer ID Application:") || !strings.Contains(details, "Runtime Version=") {
		return fmt.Errorf("update application Developer ID identity is invalid")
	}
	archs, err := run(ctx, "/usr/bin/lipo", "-archs", executable)
	if err != nil || strings.TrimSpace(archs) != architecture {
		return fmt.Errorf("update application architecture is not arm64-only")
	}
	if _, err := run(ctx, "/usr/sbin/spctl", "--assess", "--type", "execute", "--verbose=4", appPath); err != nil {
		return fmt.Errorf("assess update application: %w", err)
	}
	return nil
}

func installedApplicationPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate running application: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve running application: %w", err)
	}
	appPath := applicationBundlePath(resolved)
	if appPath != "/Applications/aulycMail.app" {
		return "", fmt.Errorf("automatic installation requires aulycMail.app in /Applications")
	}
	return appPath, nil
}

func applicationBundlePath(executable string) string {
	return filepath.Clean(filepath.Join(filepath.Dir(executable), "..", ".."))
}

func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed: %w", filepath.Base(name), err)
	}
	return string(output), nil
}
