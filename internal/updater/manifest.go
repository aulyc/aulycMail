package updater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	GitHubManifestURL = "https://raw.githubusercontent.com/aulyc/aulycMail/release-channel/latest.json"
	GiteeManifestURL  = "https://gitee.com/aulyc/aulycMail/raw/main/latest.json"

	latestSchemaV1      = "urn:codex-engineering-standards:dual-mirror-latest:1"
	latestSchemaV2      = "urn:codex-engineering-standards:dual-mirror-latest:2"
	policyID            = "aulyc-dual-mirror-v1"
	releaseProfile      = "macos-arm64-app"
	releaseChannel      = "formal"
	bundleIdentifier    = "com.aulyc.aulycmail"
	architecture        = "arm64"
	teamIdentifier      = "M9M7M2ARFD"
	sourceRepository    = "aulyc/aulycMail"
	publicRepository    = "aulyc/aulycMail"
	minimumSystemTarget = "11.0"
)

var (
	stableVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	versionPattern       = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?$`)
	commitPattern        = regexp.MustCompile(`^[a-f0-9]{40}$`)
	sha256Pattern        = regexp.MustCompile(`^[a-f0-9]{64}$`)
	safeFilenamePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
)

type Download struct {
	Source string `json:"source"`
	URL    string `json:"url"`
}

type Downloadable struct {
	File      string     `json:"file"`
	SHA256    string     `json:"sha256"`
	Downloads []Download `json:"downloads"`
}

type Manifest struct {
	Schema               string       `json:"$schema"`
	SchemaVersion        int          `json:"schemaVersion"`
	Policy               string       `json:"policy"`
	ReleaseProfile       string       `json:"releaseProfile"`
	ReleaseChannel       string       `json:"releaseChannel"`
	Version              string       `json:"version"`
	BuildNumber          int          `json:"buildNumber"`
	Tag                  string       `json:"tag"`
	Commit               string       `json:"commit"`
	BundleIdentifier     string       `json:"bundleIdentifier"`
	PluginIdentifier     *string      `json:"pluginIdentifier"`
	Architecture         string       `json:"architecture"`
	ReleasePageURL       string       `json:"releasePageURL"`
	Artifact             Downloadable `json:"artifact"`
	Provenance           Downloadable `json:"provenance"`
	TeamIdentifier       string       `json:"teamIdentifier"`
	MinimumSystemVersion string       `json:"minimumSystemVersion"`
}

func ParseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode update manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Manifest{}, fmt.Errorf("decode update manifest: trailing data")
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	provenanceLocation := releaseDownload
	switch {
	case m.Schema == latestSchemaV1 && m.SchemaVersion == 1:
	case m.Schema == latestSchemaV2 && m.SchemaVersion == 2:
		provenanceLocation = updateFeedDownload
	default:
		return fmt.Errorf("update manifest policy identity is invalid")
	}
	if m.Policy != policyID {
		return fmt.Errorf("update manifest policy identity is invalid")
	}
	if m.ReleaseProfile != releaseProfile || m.ReleaseChannel != releaseChannel {
		return fmt.Errorf("update manifest is not a formal macOS application release")
	}
	if !stableVersionPattern.MatchString(m.Version) || m.Tag != m.Version || m.BuildNumber <= 0 {
		return fmt.Errorf("update manifest version identity is invalid")
	}
	if !commitPattern.MatchString(m.Commit) {
		return fmt.Errorf("update manifest commit is invalid")
	}
	if m.BundleIdentifier != bundleIdentifier || m.PluginIdentifier != nil || m.Architecture != architecture {
		return fmt.Errorf("update manifest product identity is invalid")
	}
	if m.TeamIdentifier != teamIdentifier || m.MinimumSystemVersion != minimumSystemTarget {
		return fmt.Errorf("update manifest macOS identity is invalid")
	}
	page, err := url.Parse(m.ReleasePageURL)
	expectedReleasePath := "/" + publicRepository + "/releases/tag/" + m.Tag
	if err != nil || page.Scheme != "https" || page.Hostname() != "github.com" || page.User != nil || page.Port() != "" || page.Path != expectedReleasePath || page.RawPath != "" || page.RawQuery != "" || page.Fragment != "" {
		return fmt.Errorf("update manifest release page is unsafe")
	}
	if err := validateDownloadable(m.Artifact, "artifact", m.Tag, releaseDownload); err != nil {
		return err
	}
	if err := validateDownloadable(m.Provenance, "provenance", m.Tag, provenanceLocation); err != nil {
		return err
	}
	return nil
}

type downloadLocation int

const (
	releaseDownload downloadLocation = iota
	updateFeedDownload
)

func validateDownloadable(item Downloadable, label, tag string, location downloadLocation) error {
	if !safeFilenamePattern.MatchString(item.File) || !sha256Pattern.MatchString(item.SHA256) {
		return fmt.Errorf("update manifest %s identity is invalid", label)
	}
	if len(item.Downloads) != 2 || item.Downloads[0].Source != "github" || item.Downloads[1].Source != "gitee" {
		return fmt.Errorf("update manifest %s sources must be GitHub then Gitee", label)
	}
	for _, download := range item.Downloads {
		parsed, err := url.Parse(download.URL)
		if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("update manifest %s download URL is unsafe", label)
		}
		expectedHost, expectedPath := expectedDownloadLocation(download.Source, tag, item.File, location)
		if parsed.Hostname() != expectedHost || parsed.Path != expectedPath || parsed.RawPath != "" {
			return fmt.Errorf("update manifest %s download source is invalid", label)
		}
	}
	return nil
}

func expectedDownloadLocation(source, tag, file string, location downloadLocation) (string, string) {
	if location == updateFeedDownload {
		if source == "github" {
			return "raw.githubusercontent.com", "/aulyc/aulycMail/release-channel/updates/" + tag + "/" + file
		}
		return "gitee.com", "/aulyc/aulycMail/raw/main/updates/" + tag + "/" + file
	}
	host := "github.com"
	if source == "gitee" {
		host = "gitee.com"
	}
	return host, "/aulyc/aulycMail/releases/download/" + tag + "/" + file
}

type semanticVersion struct {
	major, minor, patch int
	pre                 string
}

func parseSemanticVersion(value string) (semanticVersion, error) {
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return semanticVersion{}, fmt.Errorf("invalid semantic version %q", value)
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	return semanticVersion{major: major, minor: minor, patch: patch, pre: match[4]}, nil
}

// IsNewer reports whether the formal manifest is newer than the running build.
// Stable releases sort after prereleases with the same numeric version.
func IsNewer(latest string, latestBuild int, current string, currentBuild int) (bool, error) {
	lv, err := parseSemanticVersion(latest)
	if err != nil {
		return false, err
	}
	cv, err := parseSemanticVersion(current)
	if err != nil {
		return false, err
	}
	for _, pair := range [][2]int{{lv.major, cv.major}, {lv.minor, cv.minor}, {lv.patch, cv.patch}} {
		if pair[0] != pair[1] {
			return pair[0] > pair[1], nil
		}
	}
	if lv.pre == "" && cv.pre != "" {
		return true, nil
	}
	if lv.pre != "" && cv.pre == "" {
		return false, nil
	}
	if lv.pre != cv.pre {
		return comparePrerelease(lv.pre, cv.pre) > 0, nil
	}
	return latestBuild > currentBuild, nil
}

func comparePrerelease(left, right string) int {
	lparts, rparts := strings.Split(left, "."), strings.Split(right, ".")
	for i := 0; i < len(lparts) && i < len(rparts); i++ {
		if lparts[i] == rparts[i] {
			continue
		}
		li, lerr := strconv.Atoi(lparts[i])
		ri, rerr := strconv.Atoi(rparts[i])
		switch {
		case lerr == nil && rerr == nil:
			if li > ri {
				return 1
			}
			return -1
		case lerr == nil:
			return -1
		case rerr == nil:
			return 1
		case lparts[i] > rparts[i]:
			return 1
		default:
			return -1
		}
	}
	if len(lparts) > len(rparts) {
		return 1
	}
	if len(lparts) < len(rparts) {
		return -1
	}
	return 0
}
