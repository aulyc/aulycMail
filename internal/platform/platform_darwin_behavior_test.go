//go:build darwin

package platform

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDarwinSingleInstanceLockActivationAndStaleSocketRecovery(t *testing.T) {
	home, err := os.MkdirTemp("/tmp", "am-platform-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	primary := NewSingleInstanceLock().(*darwinSingleInstanceLock)
	commands := make(chan string, 4)
	primary.SetOnShow(func(command string) { commands <- command })
	acquired, err := primary.TryLock("show")
	if err != nil || !acquired {
		t.Fatalf("primary TryLock() = %v, %v", acquired, err)
	}

	secondary := NewSingleInstanceLock().(*darwinSingleInstanceLock)
	acquired, err = secondary.TryLock("mailto:test@example.com")
	if err != nil || acquired {
		t.Fatalf("secondary TryLock() = %v, %v", acquired, err)
	}
	select {
	case command := <-commands:
		if command != "mailto:test@example.com" {
			t.Fatalf("activation command = %q", command)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for activation command")
	}

	runConnection := func(command string) {
		t.Helper()
		server, client := net.Pipe()
		done := make(chan struct{})
		go func() {
			primary.handleConnection(server)
			close(done)
		}()
		if _, err := client.Write([]byte(command + "\n")); err != nil {
			t.Fatal(err)
		}
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for connection handler")
		}
	}
	runConnection("unsupported")
	select {
	case command := <-commands:
		t.Fatalf("unexpected rejected command callback: %q", command)
	default:
	}

	primary.SetOnShow(nil)
	runConnection("show")

	stalePath := primary.socketPath
	primary.Unlock()
	if err := os.WriteFile(stalePath, []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	recovered := NewSingleInstanceLock().(*darwinSingleInstanceLock)
	acquired, err = recovered.TryLock("show")
	if err != nil || !acquired {
		t.Fatalf("stale recovery TryLock() = %v, %v", acquired, err)
	}
	recovered.Unlock()
}

func TestDarwinAutostartManagerUsesOnlyTemporaryHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager := NewAutostartManager().(*darwinAutostartManager)
	if manager.IsEnabled() {
		t.Fatal("autostart unexpectedly enabled")
	}
	if err := manager.Enable(); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	if !manager.IsEnabled() {
		t.Fatal("autostart not enabled")
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
	contents, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), launchAgentLabel) || !strings.Contains(string(contents), manager.execCommand()) {
		t.Fatalf("unexpected LaunchAgent plist: %s", contents)
	}
	if err := manager.Disable(); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if manager.IsEnabled() {
		t.Fatal("autostart still enabled after Disable")
	}
	if err := manager.Disable(); err != nil {
		t.Fatalf("idempotent Disable() error = %v", err)
	}
}

func TestDarwinNetworkMonitorStateAndWaitersWithoutNativeStartup(t *testing.T) {
	monitor := NewNetworkMonitor().(*DarwinNetworkMonitor)
	if !monitor.IsConnected() {
		t.Fatal("new monitor should optimistically start connected")
	}
	monitor.updateState(true)
	monitor.updateState(false)
	select {
	case event := <-monitor.Events():
		if event.Connected {
			t.Fatalf("disconnect event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("missing disconnect event")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if monitor.WaitForConnection(cancelled) {
		t.Fatal("cancelled WaitForConnection() = true")
	}

	waited := make(chan bool, 1)
	go func() { waited <- monitor.WaitForConnection(context.Background()) }()
	monitor.updateState(true)
	select {
	case connected := <-waited:
		if !connected {
			t.Fatal("restored WaitForConnection() = false")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for restored connection")
	}
	monitor.Invalidate()
	if monitor.IsConnected() {
		t.Fatal("Invalidate() left monitor connected")
	}
	if err := monitor.Stop(); err != nil {
		t.Fatalf("inactive Stop() error = %v", err)
	}
}

func TestDarwinSleepWakeMonitorInactiveSurface(t *testing.T) {
	monitor := NewSleepWakeMonitor().(*DarwinSleepWakeMonitor)
	if monitor.Events() == nil {
		t.Fatal("Events() returned nil")
	}
	if err := monitor.Stop(); err != nil {
		t.Fatalf("inactive Stop() error = %v", err)
	}
}

func TestDarwinThemeMonitorReportsFrontendFallback(t *testing.T) {
	monitor := NewThemeMonitor().(*DarwinThemeMonitor)
	if err := monitor.Start(context.Background()); err == nil {
		t.Fatal("Darwin theme monitor unexpectedly started")
	}
	if got := monitor.GetTheme(); got != SystemThemeNoPreference {
		t.Fatalf("Darwin theme preference = %q", got)
	}
	if monitor.Events() == nil {
		t.Fatal("Darwin theme event surface is nil")
	}
	if err := monitor.Stop(); err != nil {
		t.Fatalf("Darwin theme Stop() error = %v", err)
	}
}
