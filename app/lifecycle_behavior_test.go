package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/platform"
	"aulyc.local/aulycmail/internal/settings"
)

type lifecycleThemeMonitor struct {
	events   chan platform.SystemTheme
	initial  platform.SystemTheme
	startErr error
	starts   atomic.Int32
	stops    atomic.Int32
}

func (m *lifecycleThemeMonitor) Start(context.Context) error {
	m.starts.Add(1)
	return m.startErr
}

func (m *lifecycleThemeMonitor) GetTheme() platform.SystemTheme { return m.initial }
func (m *lifecycleThemeMonitor) Events() <-chan platform.SystemTheme {
	return m.events
}
func (m *lifecycleThemeMonitor) Stop() error {
	m.stops.Add(1)
	return nil
}

func TestWindowLifecycleUsesObservableNativeActionSeams(t *testing.T) {
	a, _, _ := newContactOwnEmailsTestApp(t)
	a.ctx = context.Background()
	a.settingsStore = settings.NewStore(a.db)
	recorder := &actionEventRecorder{}
	a.eventsEmit = recorder.emit

	var hidden, unminimised, shown, startupSignals atomic.Int32
	a.hideWindowAction = func() { hidden.Add(1) }
	a.unminimiseWindowAction = func() { unminimised.Add(1) }
	a.nativeShowWindowAction = func() { shown.Add(1) }
	a.notifyStartupAction = func() { startupSignals.Add(1) }

	a.windowHidden.Store(true)
	a.showWindow()
	if a.windowHidden.Load() || unminimised.Load() != 1 || shown.Load() != 1 || !recorder.has("window:show") {
		t.Fatalf("ShowWindow state = hidden:%v unminimised:%d shown:%d events:%#v", a.windowHidden.Load(), unminimised.Load(), shown.Load(), recorder.snapshot())
	}

	beforeEvents := len(recorder.snapshot())
	a.emitExternalOpenFiles(nil)
	if len(recorder.snapshot()) != beforeEvents {
		t.Fatal("empty external-file batch emitted an event")
	}
	a.emitExternalOpenFiles([]string{"/tmp/synthetic.eml"})
	if shown.Load() != 2 || !recorder.has(externalFileOpenEvent) {
		t.Fatalf("external-file window/event state = shown:%d events:%#v", shown.Load(), recorder.snapshot())
	}

	batcher := newExternalFileOpenBatcher(time.Hour, nil)
	a.externalFileOpen = batcher
	a.NotifyStartupComplete()
	batcher.mu.Lock()
	ready := batcher.ready
	batcher.mu.Unlock()
	if startupSignals.Load() != 1 || !ready {
		t.Fatalf("startup completion state = signals:%d ready:%v", startupSignals.Load(), ready)
	}
	batcher.Stop()

	if err := a.settingsStore.SetRunBackground(true); err != nil {
		t.Fatal(err)
	}
	if preventClose := a.BeforeClose(context.Background()); !preventClose || hidden.Load() != 1 || !a.windowHidden.Load() {
		t.Fatalf("background BeforeClose = prevent:%v hiddenCalls:%d hiddenState:%v", preventClose, hidden.Load(), a.windowHidden.Load())
	}

	if err := a.settingsStore.SetRunBackground(false); err != nil {
		t.Fatal(err)
	}
	quitCalls := make(chan struct{}, 2)
	a.quitAction = func() { quitCalls <- struct{}{} }
	if preventClose := a.BeforeClose(context.Background()); !preventClose {
		t.Fatal("normal BeforeClose should defer quitting and prevent immediate close")
	}
	select {
	case <-quitCalls:
	case <-time.After(time.Second):
		t.Fatal("BeforeClose did not request quit")
	}
	if preventClose := a.BeforeClose(context.Background()); preventClose {
		t.Fatal("re-entrant BeforeClose should allow the active shutdown to continue")
	}

	a.shuttingDown.Store(false)
	a.QuitApp()
	select {
	case <-quitCalls:
	case <-time.After(time.Second):
		t.Fatal("QuitApp did not request quit")
	}
	a.QuitApp()
	select {
	case <-quitCalls:
		t.Fatal("re-entrant QuitApp requested a second quit")
	default:
	}
}

func TestQuitAppConcurrentRequestsOnlyQuitOnce(t *testing.T) {
	a := &App{ctx: context.Background()}
	var quitCalls atomic.Int32
	a.quitAction = func() { quitCalls.Add(1) }

	const callers = 64
	start := make(chan struct{})
	var callersDone sync.WaitGroup
	callersDone.Add(callers)
	for range callers {
		go func() {
			defer callersDone.Done()
			<-start
			a.QuitApp()
		}()
	}
	close(start)
	callersDone.Wait()

	waitForBackgroundCondition(t, "single quit request", func() bool {
		return quitCalls.Load() > 0
	})
	if got := quitCalls.Load(); got != 1 {
		t.Fatalf("concurrent QuitApp calls requested %d quits, want 1", got)
	}
}

func TestThemeLifecycleCoversUnavailableInitialChangesAndCancellation(t *testing.T) {
	recorder := &actionEventRecorder{}
	a := &App{ctx: context.Background(), eventsEmit: recorder.emit}

	unavailable := &lifecycleThemeMonitor{
		events:   make(chan platform.SystemTheme),
		startErr: errors.New("synthetic unavailable theme service"),
	}
	a.newThemeMonitor = func() platform.ThemeMonitor { return unavailable }
	a.initThemeMonitor(context.Background())
	if unavailable.starts.Load() != 1 || a.currentThemeMonitor() != nil || a.GetSystemTheme() != "" {
		t.Fatalf("unavailable theme state = starts:%d monitor:%#v value:%q", unavailable.starts.Load(), a.currentThemeMonitor(), a.GetSystemTheme())
	}

	available := &lifecycleThemeMonitor{
		events:  make(chan platform.SystemTheme, 2),
		initial: platform.SystemThemeDark,
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.newThemeMonitor = func() platform.ThemeMonitor { return available }
	a.initThemeMonitor(ctx)
	if a.GetSystemTheme() != "dark" || !recorder.has("theme:system-preference") {
		t.Fatalf("initial theme state = value:%q events:%#v", a.GetSystemTheme(), recorder.snapshot())
	}
	available.events <- platform.SystemThemeLight
	waitForBackgroundCondition(t, "theme change event", func() bool {
		count := 0
		for _, eventName := range recorder.snapshot() {
			if eventName == "theme:system-preference" {
				count++
			}
		}
		return count >= 2
	})
	cancel()

	closed := &lifecycleThemeMonitor{events: make(chan platform.SystemTheme)}
	close(closed.events)
	a.setThemeMonitor(closed)
	done := make(chan struct{})
	go func() {
		a.processThemeEvents(context.Background(), closed)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("theme processor did not stop after the event channel closed")
	}

	a.setThemeMonitor(nil)
	a.processThemeEvents(context.Background(), nil)
}
