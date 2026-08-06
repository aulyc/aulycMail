package updater

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type failingSettings struct {
	getErr       error
	setErr       error
	automaticErr error
	values       map[string]string
}

func (s *failingSettings) Get(key string) (string, error) {
	return s.values[key], s.getErr
}

func (s *failingSettings) Set(key, value string) error {
	if s.setErr == nil {
		s.values[key] = value
	}
	return s.setErr
}

func (s *failingSettings) GetAutomaticUpdateChecks() (bool, error) {
	return true, s.automaticErr
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingReader) Close() error             { return nil }

func TestCheckReturnsCurrentStateWhileBusy(t *testing.T) {
	service := NewService(Config{CurrentVersion: "1.0.0", CurrentBuild: 1})
	service.status = Status{State: StateInstalling, CurrentVersion: "1.0.0", CurrentBuildNumber: 1}
	service.installing = true
	status, err := service.Check(context.Background())
	if err != nil || status.State != StateInstalling {
		t.Fatalf("Check() = %#v, %v", status, err)
	}

	service.installing = false
	service.checking = true
	status, err = service.Check(context.Background())
	if err != nil || status.State != StateInstalling {
		t.Fatalf("busy Check() = %#v, %v", status, err)
	}
}

func TestCheckReportsRunningVersionFailure(t *testing.T) {
	data, _ := jsonMarshalForTest(validManifest())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(data) }))
	defer server.Close()
	store := &failingSettings{values: map[string]string{}}
	service := NewService(Config{
		CurrentVersion: "invalid", CurrentBuild: 1, Store: store, HTTPClient: server.Client(),
		ManifestSources: []ManifestSource{{Source: "github", URL: server.URL}},
	})
	status, err := service.Check(context.Background())
	if err == nil || status.State != StateFailed || status.FailureOperation != FailureOperationCheck {
		t.Fatalf("Check() = %#v, %v", status, err)
	}
	if store.values[lastFailureKey] == "" {
		t.Fatal("failed check timestamp was not recorded")
	}
}

func TestCheckIfDueHandlesMissingAndFailingSettings(t *testing.T) {
	NewService(Config{CurrentVersion: "1.0.0", CurrentBuild: 1}).CheckIfDue(context.Background())

	service := NewService(Config{
		CurrentVersion: "1.0.0", CurrentBuild: 1,
		Store: &failingSettings{automaticErr: errors.New("settings unavailable"), values: map[string]string{}},
	})
	service.CheckIfDue(context.Background())
	if service.Status().State != StateIdle {
		t.Fatalf("status = %#v", service.Status())
	}
}

func TestDueHonorsFailureThrottleAndIgnoresInvalidTimestamps(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	store := &failingSettings{values: map[string]string{lastFailureKey: now.Add(-time.Hour).Format(time.RFC3339)}}
	service := NewService(Config{CurrentVersion: "1.0.0", CurrentBuild: 1, Store: store, Now: func() time.Time { return now }})
	if service.due() {
		t.Fatal("recent failure did not throttle update checks")
	}
	store.values[lastFailureKey] = "not-a-time"
	if !service.due() {
		t.Fatal("invalid persisted timestamp blocked update checks")
	}
	store.getErr = errors.New("read unavailable")
	if _, ok := service.readTimestamp(lastSuccessKey); ok {
		t.Fatal("settings read error returned a timestamp")
	}
	if _, ok := NewService(Config{}).readTimestamp(lastSuccessKey); ok {
		t.Fatal("nil settings returned a timestamp")
	}
}

func TestInstallRejectsMissingInstallerAndReturnsCurrentStateWhileBusy(t *testing.T) {
	manifest := validManifest()
	service := NewService(Config{CurrentVersion: "0.6.0", CurrentBuild: 1})
	service.manifest = &manifest
	service.status = Status{State: StateAvailable, CanInstall: true}
	if _, err := service.Install(context.Background()); err == nil || !strings.Contains(err.Error(), "installation is unavailable") {
		t.Fatalf("Install() error = %v", err)
	}

	service.installing = true
	status, err := service.Install(context.Background())
	if err != nil || status.State != StateAvailable {
		t.Fatalf("busy Install() = %#v, %v", status, err)
	}
}

func TestSafeRedirectRejectsCredentialedAndExcessiveChains(t *testing.T) {
	credentialed, _ := http.NewRequest(http.MethodGet, "https://user@github.com/file", nil)
	if err := safeRedirect(credentialed, nil); err == nil {
		t.Fatal("credentialed redirect was accepted")
	}
	request, _ := http.NewRequest(http.MethodGet, "https://github.com/file", nil)
	if err := safeRedirect(request, make([]*http.Request, 10)); err == nil {
		t.Fatal("excessive redirect chain was accepted")
	}
}

func TestFetchManifestRejectsHTTPAndBodyFailures(t *testing.T) {
	tests := []struct {
		name   string
		client *http.Client
		url    string
	}{
		{
			name: "connection error",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("offline")
			})},
			url: "https://github.com/manifest",
		},
		{
			name: "body read error",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: failingReader{}, Header: make(http.Header)}, nil
			})},
			url: "https://github.com/manifest",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(Config{
				CurrentVersion: "1.0.0", CurrentBuild: 1, HTTPClient: test.client,
				ManifestSources: []ManifestSource{{Source: "github", URL: test.url}},
			})
			if _, _, err := service.fetchManifest(context.Background()); err == nil || !strings.Contains(err.Error(), "all update sources failed") {
				t.Fatalf("fetchManifest() error = %v", err)
			}
		})
	}

	large := strings.Repeat("x", manifestLimit+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, large) }))
	defer server.Close()
	service := NewService(Config{
		CurrentVersion: "1.0.0", CurrentBuild: 1, HTTPClient: server.Client(),
		ManifestSources: []ManifestSource{{Source: "github", URL: server.URL}},
	})
	if _, _, err := service.fetchManifest(context.Background()); err == nil {
		t.Fatal("oversized manifest was accepted")
	}
}

func TestSchedulerCanBeCancelledBeforeFirstCheck(t *testing.T) {
	service := NewService(Config{CurrentVersion: "1.0.0", CurrentBuild: 1, StartupDelay: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	service.Start(ctx)
	cancel()
	service.Stop()
}

func jsonMarshalForTest(value any) ([]byte, error) {
	return json.Marshal(value)
}
