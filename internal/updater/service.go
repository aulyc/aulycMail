package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	StateIdle        = "idle"
	StateChecking    = "checking"
	StateUpToDate    = "upToDate"
	StateAvailable   = "available"
	StateDownloading = "downloading"
	StateVerifying   = "verifying"
	StateInstalling  = "installing"
	StateFailed      = "failed"

	FailureOperationCheck   = "check"
	FailureOperationInstall = "install"

	lastSuccessKey = "update_last_success_at"
	lastFailureKey = "update_last_failure_at"
)

const (
	manifestLimit     = 256 << 10
	automaticDelay    = 30 * time.Second
	automaticInterval = time.Hour
	successThrottle   = 24 * time.Hour
	failureThrottle   = 6 * time.Hour
)

type SettingStore interface {
	Get(string) (string, error)
	Set(string, string) error
	GetAutomaticUpdateChecks() (bool, error)
}

type Status struct {
	State              string `json:"state"`
	CurrentVersion     string `json:"currentVersion"`
	CurrentBuildNumber int    `json:"currentBuildNumber"`
	LatestVersion      string `json:"latestVersion,omitempty"`
	LatestBuildNumber  int    `json:"latestBuildNumber,omitempty"`
	ReleasePageURL     string `json:"releasePageURL,omitempty"`
	Source             string `json:"source,omitempty"`
	Progress           int    `json:"progress,omitempty"`
	Message            string `json:"message,omitempty"`
	FailureOperation   string `json:"failureOperation,omitempty"`
	CanInstall         bool   `json:"canInstall"`
	CheckedAt          string `json:"checkedAt,omitempty"`
}

type Config struct {
	CurrentVersion  string
	CurrentBuild    int
	Store           SettingStore
	HTTPClient      *http.Client
	OnChange        func(Status)
	Installer       Installer
	Now             func() time.Time
	StartupDelay    time.Duration
	TickInterval    time.Duration
	ManifestSources []ManifestSource
}

type ManifestSource struct {
	Source string
	URL    string
}

type Service struct {
	mu              sync.Mutex
	currentVersion  string
	currentBuild    int
	store           SettingStore
	client          *http.Client
	onChange        func(Status)
	installer       Installer
	now             func() time.Time
	startupDelay    time.Duration
	tickInterval    time.Duration
	manifestSources []ManifestSource
	status          Status
	manifest        *Manifest
	checking        bool
	installing      bool
	cancelScheduler context.CancelFunc
}

func NewService(config Config) *Service {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second, CheckRedirect: safeRedirect}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	delay := config.StartupDelay
	if delay == 0 {
		delay = automaticDelay
	}
	interval := config.TickInterval
	if interval == 0 {
		interval = automaticInterval
	}
	sources := config.ManifestSources
	if len(sources) == 0 {
		sources = []ManifestSource{{Source: "github", URL: GitHubManifestURL}, {Source: "gitee", URL: GiteeManifestURL}}
	}
	return &Service{
		currentVersion:  config.CurrentVersion,
		currentBuild:    config.CurrentBuild,
		store:           config.Store,
		client:          client,
		onChange:        config.OnChange,
		installer:       config.Installer,
		now:             now,
		startupDelay:    delay,
		tickInterval:    interval,
		manifestSources: append([]ManifestSource(nil), sources...),
		status:          Status{State: StateIdle, CurrentVersion: config.CurrentVersion, CurrentBuildNumber: config.CurrentBuild},
	}
}

func safeRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 10 || request.URL.Scheme != "https" || request.URL.User != nil {
		return fmt.Errorf("unsafe update redirect")
	}
	host := strings.ToLower(request.URL.Hostname())
	allowed := host == "github.com" ||
		host == "objects.githubusercontent.com" ||
		host == "release-assets.githubusercontent.com" ||
		host == "gitee.com" ||
		host == "raw.giteeusercontent.com" ||
		host == "foruda.gitee.com"
	if !allowed {
		return fmt.Errorf("update redirect host is not allowed")
	}
	return nil
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Service) publish(status Status) Status {
	s.mu.Lock()
	s.status = status
	callback := s.onChange
	s.mu.Unlock()
	if callback != nil {
		callback(status)
	}
	return status
}

func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.cancelScheduler != nil {
		s.mu.Unlock()
		return
	}
	schedulerCtx, cancel := context.WithCancel(ctx)
	s.cancelScheduler = cancel
	s.mu.Unlock()
	go func() {
		timer := time.NewTimer(s.startupDelay)
		defer timer.Stop()
		select {
		case <-schedulerCtx.Done():
			return
		case <-timer.C:
			s.CheckIfDue(schedulerCtx)
		}
		ticker := time.NewTicker(s.tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-schedulerCtx.Done():
				return
			case <-ticker.C:
				s.CheckIfDue(schedulerCtx)
			}
		}
	}()
}

func (s *Service) Stop() {
	s.mu.Lock()
	if s.cancelScheduler != nil {
		s.cancelScheduler()
		s.cancelScheduler = nil
	}
	s.mu.Unlock()
}

func (s *Service) CheckIfDue(ctx context.Context) {
	if s.store == nil {
		return
	}
	enabled, err := s.store.GetAutomaticUpdateChecks()
	if err != nil || !enabled || !s.due() {
		return
	}
	_, _ = s.Check(ctx)
}

func (s *Service) due() bool {
	now := s.now()
	if last, ok := s.readTimestamp(lastFailureKey); ok && now.Sub(last) < failureThrottle {
		return false
	}
	if last, ok := s.readTimestamp(lastSuccessKey); ok && now.Sub(last) < successThrottle {
		return false
	}
	return true
}

func (s *Service) readTimestamp(key string) (time.Time, bool) {
	if s.store == nil {
		return time.Time{}, false
	}
	value, err := s.store.Get(key)
	if err != nil || value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	return parsed, err == nil
}

func (s *Service) writeTimestamp(key string) {
	if s.store != nil {
		_ = s.store.Set(key, s.now().UTC().Format(time.RFC3339))
	}
}

func (s *Service) Check(ctx context.Context) (Status, error) {
	s.mu.Lock()
	if s.checking || s.installing {
		current := s.status
		s.mu.Unlock()
		return current, nil
	}
	s.checking = true
	base := s.status
	base.State, base.Message, base.FailureOperation, base.Progress, base.CanInstall = StateChecking, "", "", 0, false
	s.mu.Unlock()
	s.publish(base)

	manifest, source, err := s.fetchManifest(ctx)
	if err != nil {
		s.mu.Lock()
		s.checking = false
		s.mu.Unlock()
		s.writeTimestamp(lastFailureKey)
		failed := base
		failed.State, failed.Message, failed.FailureOperation, failed.CheckedAt = StateFailed, err.Error(), FailureOperationCheck, s.now().UTC().Format(time.RFC3339)
		return s.publish(failed), err
	}
	newer, err := IsNewer(manifest.Version, manifest.BuildNumber, s.currentVersion, s.currentBuild)
	if err != nil {
		s.mu.Lock()
		s.checking = false
		s.mu.Unlock()
		s.writeTimestamp(lastFailureKey)
		failed := base
		failed.State, failed.Message, failed.FailureOperation, failed.CheckedAt = StateFailed, err.Error(), FailureOperationCheck, s.now().UTC().Format(time.RFC3339)
		return s.publish(failed), err
	}
	next := Status{
		State: StateUpToDate, CurrentVersion: s.currentVersion, CurrentBuildNumber: s.currentBuild,
		LatestVersion: manifest.Version, LatestBuildNumber: manifest.BuildNumber,
		ReleasePageURL: manifest.ReleasePageURL, Source: source, CheckedAt: s.now().UTC().Format(time.RFC3339),
	}
	if newer {
		next.State, next.CanInstall = StateAvailable, true
	}
	s.mu.Lock()
	s.manifest = &manifest
	s.checking = false
	s.mu.Unlock()
	s.writeTimestamp(lastSuccessKey)
	return s.publish(next), nil
}

func (s *Service) fetchManifest(ctx context.Context) (Manifest, string, error) {
	var failures []string
	for _, candidate := range s.manifestSources {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.URL, nil)
		if err != nil {
			failures = append(failures, candidate.Source+": request failed")
			continue
		}
		request.Header.Set("Accept", "application/json")
		response, err := s.client.Do(request)
		if err != nil {
			failures = append(failures, candidate.Source+": connection failed")
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, manifestLimit+1))
		_ = response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK || len(body) > manifestLimit {
			failures = append(failures, candidate.Source+": invalid response")
			continue
		}
		manifest, parseErr := ParseManifest(body)
		if parseErr != nil {
			failures = append(failures, candidate.Source+": "+parseErr.Error())
			continue
		}
		return manifest, candidate.Source, nil
	}
	return Manifest{}, "", fmt.Errorf("all update sources failed: %s", strings.Join(failures, "; "))
}

func (s *Service) Install(ctx context.Context) (Status, error) {
	s.mu.Lock()
	if s.checking || s.installing {
		current := s.status
		s.mu.Unlock()
		return current, nil
	}
	if s.manifest == nil || s.status.State != StateAvailable || !s.status.CanInstall {
		current := s.status
		s.mu.Unlock()
		return current, fmt.Errorf("no verified update is available")
	}
	if s.installer == nil {
		current := s.status
		s.mu.Unlock()
		return current, fmt.Errorf("update installation is unavailable")
	}
	manifest := *s.manifest
	s.installing = true
	base := s.status
	base.State, base.CanInstall, base.Progress, base.Message, base.FailureOperation = StateDownloading, false, 0, "", ""
	s.mu.Unlock()
	s.publish(base)

	progress := func(state string, percent int) {
		next := base
		next.State, next.Progress = state, percent
		s.publish(next)
	}
	err := s.installer.PrepareAndLaunch(ctx, manifest, s.client, progress)
	s.mu.Lock()
	s.installing = false
	s.mu.Unlock()
	if err != nil {
		failed := base
		failed.State, failed.Message, failed.FailureOperation, failed.Progress = StateFailed, err.Error(), FailureOperationInstall, 0
		return s.publish(failed), err
	}
	installed := base
	installed.State, installed.Progress = StateInstalling, 100
	return s.publish(installed), nil
}

func ParseBuildNumber(value string) int {
	build, err := strconv.Atoi(value)
	if err != nil || build < 0 {
		return 0
	}
	return build
}
