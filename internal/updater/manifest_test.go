package updater

import (
	"encoding/json"
	"strings"
	"testing"
)

func validManifest() Manifest {
	return Manifest{
		Schema: latestSchemaV2, SchemaVersion: 2, Policy: policyID,
		ReleaseProfile: releaseProfile, ReleaseChannel: releaseChannel,
		Version: "0.7.0", BuildNumber: 40, Tag: "0.7.0",
		Commit:           "0123456789abcdef0123456789abcdef01234567",
		BundleIdentifier: bundleIdentifier, PluginIdentifier: nil, Architecture: architecture,
		ReleasePageURL: "https://github.com/aulyc/aulycMail/releases/tag/0.7.0",
		Artifact:       Downloadable{File: "aulycMail-0.7.0-arm64.dmg", SHA256: repeatedHex("a"), Downloads: validReleaseDownloads("0.7.0", "aulycMail-0.7.0-arm64.dmg")},
		Provenance:     Downloadable{File: "aulycMail-0.7.0-arm64.manifest.json", SHA256: repeatedHex("b"), Downloads: validUpdateFeedDownloads("0.7.0", "aulycMail-0.7.0-arm64.manifest.json")},
		TeamIdentifier: teamIdentifier, MinimumSystemVersion: minimumSystemTarget,
	}
}

func validReleaseDownloads(tag, file string) []Download {
	return []Download{
		{Source: "github", URL: "https://github.com/aulyc/aulycMail/releases/download/" + tag + "/" + file},
		{Source: "gitee", URL: "https://gitee.com/aulyc/aulycMail/releases/download/" + tag + "/" + file},
	}
}

func validUpdateFeedDownloads(version, file string) []Download {
	return []Download{
		{Source: "github", URL: "https://raw.githubusercontent.com/aulyc/aulycMail/release-channel/updates/" + version + "/" + file},
		{Source: "gitee", URL: "https://gitee.com/aulyc/aulycMail/raw/main/updates/" + version + "/" + file},
	}
}

func repeatedHex(value string) string {
	result := ""
	for len(result) < 64 {
		result += value
	}
	return result
}

func TestParseManifestAcceptsExactFormalIdentity(t *testing.T) {
	data, err := json.Marshal(validManifest())
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if manifest.Version != "0.7.0" || manifest.BuildNumber != 40 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestParseManifestKeepsReadOnlySchemaV1Compatibility(t *testing.T) {
	manifest := validManifest()
	manifest.Schema = latestSchemaV1
	manifest.SchemaVersion = 1
	manifest.Provenance.Downloads = validReleaseDownloads(manifest.Tag, manifest.Provenance.File)
	data, _ := json.Marshal(manifest)
	if _, err := ParseManifest(data); err != nil {
		t.Fatalf("legacy schema v1 manifest rejected: %v", err)
	}
}

func TestParseManifestRejectsUnknownFieldsAndSourceReordering(t *testing.T) {
	data, _ := json.Marshal(validManifest())
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	raw["unexpected"] = true
	withUnknown, _ := json.Marshal(raw)
	if _, err := ParseManifest(withUnknown); err == nil {
		t.Fatal("unknown field was accepted")
	}

	manifest := validManifest()
	manifest.Artifact.Downloads[0], manifest.Artifact.Downloads[1] = manifest.Artifact.Downloads[1], manifest.Artifact.Downloads[0]
	reordered, _ := json.Marshal(manifest)
	if _, err := ParseManifest(reordered); err == nil {
		t.Fatal("reordered sources were accepted")
	}
}

func TestIsNewerHandlesPrereleaseAndBuildIdentity(t *testing.T) {
	tests := []struct {
		name         string
		latest       string
		latestBuild  int
		current      string
		currentBuild int
		want         bool
	}{
		{"stable after matching beta", "0.6.0", 40, "0.6.0-beta.23", 39, true},
		{"greater stable version", "0.7.0", 1, "0.6.9", 999, true},
		{"same version greater build", "1.2.3", 9, "1.2.3", 8, true},
		{"same identity", "1.2.3", 8, "1.2.3", 8, false},
		{"older manifest", "1.2.2", 99, "1.2.3", 1, false},
		{"numeric prerelease order", "1.2.3-beta.10", 1, "1.2.3-beta.2", 1, true},
		{"numeric identifier before text", "1.2.3-beta.2", 1, "1.2.3-beta.alpha", 1, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := IsNewer(test.latest, test.latestBuild, test.current, test.currentBuild)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("IsNewer() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestManifestRejectsWrongProductAndUnsafeDownload(t *testing.T) {
	manifest := validManifest()
	manifest.BundleIdentifier = "example.invalid"
	if err := manifest.Validate(); err == nil {
		t.Fatal("wrong bundle identifier was accepted")
	}
	manifest = validManifest()
	manifest.Artifact.Downloads[0].URL = "https://example.test/update.dmg"
	if err := manifest.Validate(); err == nil {
		t.Fatal("unsafe download host was accepted")
	}
	if _, err := ParseManifest([]byte(`{"schemaVersion":1} trailing`)); err == nil {
		t.Fatal("trailing data was accepted")
	}
	if _, err := IsNewer("invalid", 1, "1.0.0", 1); err == nil {
		t.Fatal("invalid latest version was accepted")
	}
	manifest = validManifest()
	manifest.Provenance.Downloads = validReleaseDownloads(manifest.Tag, manifest.Provenance.File)
	if err := manifest.Validate(); err == nil {
		t.Fatal("schema v2 release-asset provenance URL was accepted")
	}
	manifest = validManifest()
	manifest.Provenance.Downloads[0].URL = "https://raw.githubusercontent.com/aulyc/aulycMail/main/updates/0.7.0/" + manifest.Provenance.File
	if err := manifest.Validate(); err == nil {
		t.Fatal("schema v2 provenance from the wrong GitHub branch was accepted")
	}
	manifest = validManifest()
	manifest.ReleasePageURL = "https://github.com/aulyc/aulycMail/releases/tag/0.6.0"
	if err := manifest.Validate(); err == nil {
		t.Fatal("release page for a different tag was accepted")
	}
	mutations := []func(*Manifest){
		func(value *Manifest) { value.Schema = latestSchemaV1 },
		func(value *Manifest) { value.ReleaseChannel = "test" },
		func(value *Manifest) { value.Version, value.Tag = "0.7.0-beta.1", "0.7.0-beta.1" },
		func(value *Manifest) { value.Commit = "short" },
		func(value *Manifest) { value.TeamIdentifier = "OTHER" },
		func(value *Manifest) {
			value.ReleasePageURL = "http://github.com/aulyc/aulycMail/releases/tag/0.7.0"
		},
	}
	for index, mutate := range mutations {
		candidate := validManifest()
		mutate(&candidate)
		if err := candidate.Validate(); err == nil {
			t.Fatalf("invalid manifest mutation %d was accepted", index)
		}
	}
}

func TestPrereleaseComparisonCoversSemverIdentifierRules(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "beta", right: "beta", want: 0},
		{left: "beta.1", right: "beta", want: 1},
		{left: "beta", right: "beta.1", want: -1},
		{left: "beta.11", right: "beta.2", want: 1},
		{left: "beta.2", right: "beta.11", want: -1},
		{left: "beta.alpha", right: "beta.2", want: 1},
		{left: "beta.2", right: "beta.alpha", want: -1},
		{left: "beta.zulu", right: "beta.alpha", want: 1},
		{left: "beta.alpha", right: "beta.zulu", want: -1},
	}
	for _, test := range tests {
		name := strings.ReplaceAll(test.left+"_vs_"+test.right, ".", "_")
		t.Run(name, func(t *testing.T) {
			if got := comparePrerelease(test.left, test.right); got != test.want {
				t.Fatalf("comparePrerelease(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
			}
		})
	}
}

func TestIsNewerRejectsInvalidCurrentAndOrdersStableAgainstPrerelease(t *testing.T) {
	if _, err := IsNewer("1.0.0", 1, "invalid", 1); err == nil {
		t.Fatal("invalid current version was accepted")
	}
	if newer, err := IsNewer("1.0.0-beta.1", 99, "1.0.0", 1); err != nil || newer {
		t.Fatalf("prerelease versus stable = %v, %v", newer, err)
	}
}

func TestManifestValidationRejectsRemainingIdentityClasses(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{name: "unknown schema", mutate: func(value *Manifest) { value.SchemaVersion = 99 }, want: "policy identity"},
		{name: "wrong policy", mutate: func(value *Manifest) { value.Policy = "other" }, want: "policy identity"},
		{name: "zero build", mutate: func(value *Manifest) { value.BuildNumber = 0 }, want: "version identity"},
		{name: "tag mismatch", mutate: func(value *Manifest) { value.Tag = "0.7.1" }, want: "version identity"},
		{name: "plugin identity", mutate: func(value *Manifest) { plugin := "mail"; value.PluginIdentifier = &plugin }, want: "product identity"},
		{name: "wrong architecture", mutate: func(value *Manifest) { value.Architecture = "x86_64" }, want: "product identity"},
		{name: "minimum system", mutate: func(value *Manifest) { value.MinimumSystemVersion = "12.0" }, want: "macOS identity"},
		{name: "artifact filename", mutate: func(value *Manifest) { value.Artifact.File = "../mail.dmg" }, want: "artifact identity"},
		{name: "artifact checksum", mutate: func(value *Manifest) { value.Artifact.SHA256 = "short" }, want: "artifact identity"},
		{name: "missing source", mutate: func(value *Manifest) { value.Artifact.Downloads = value.Artifact.Downloads[:1] }, want: "sources must be"},
		{name: "download user", mutate: func(value *Manifest) { value.Artifact.Downloads[0].URL = "https://user@github.com/file" }, want: "URL is unsafe"},
		{name: "download port", mutate: func(value *Manifest) { value.Artifact.Downloads[0].URL = "https://github.com:443/file" }, want: "URL is unsafe"},
		{name: "download query", mutate: func(value *Manifest) { value.Artifact.Downloads[0].URL += "?token=1" }, want: "URL is unsafe"},
		{name: "download fragment", mutate: func(value *Manifest) { value.Artifact.Downloads[0].URL += "#fragment" }, want: "URL is unsafe"},
		{name: "download encoded path", mutate: func(value *Manifest) {
			value.Artifact.Downloads[0].URL = "https://github.com/aulyc/aulycMail/releases/download/0.7.0/aulycMail-0.7.0-arm64%2edmg"
		}, want: "source is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := validManifest()
			test.mutate(&manifest)
			if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}
