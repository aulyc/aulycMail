package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/notification"
	"aulyc.local/aulycmail/internal/platform"
	mailSync "aulyc.local/aulycmail/internal/sync"
)

type recordingNotifier struct {
	mu            sync.Mutex
	notifications []notification.Notification
	showErr       error
	clickHandler  notification.ClickHandler
	settings      notification.SettingsHandler
	starts        int
	stops         int
	refreshes     int
}

func (n *recordingNotifier) Start(context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.starts++
	return nil
}

func (n *recordingNotifier) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stops++
}

func (n *recordingNotifier) Show(item notification.Notification) (uint32, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifications = append(n.notifications, item)
	return uint32(len(n.notifications)), n.showErr
}

func (n *recordingNotifier) SetClickHandler(handler notification.ClickHandler) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.clickHandler = handler
}

func (n *recordingNotifier) SetSettingsHandler(handler notification.SettingsHandler) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.settings = handler
}

func (n *recordingNotifier) RefreshSettings() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.refreshes++
}

func (n *recordingNotifier) items() []notification.Notification {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]notification.Notification(nil), n.notifications...)
}

type testNetworkMonitor struct {
	events      chan platform.NetworkEvent
	connected   atomic.Bool
	waitResult  atomic.Bool
	waitCalls   atomic.Int32
	invalidates atomic.Int32
	starts      atomic.Int32
	stops       atomic.Int32
}

func newTestNetworkMonitor() *testNetworkMonitor {
	return &testNetworkMonitor{events: make(chan platform.NetworkEvent, 4)}
}

func (m *testNetworkMonitor) Start(context.Context) error {
	m.starts.Add(1)
	return nil
}

func (m *testNetworkMonitor) Events() <-chan platform.NetworkEvent { return m.events }
func (m *testNetworkMonitor) IsConnected() bool                    { return m.connected.Load() }
func (m *testNetworkMonitor) WaitForConnection(context.Context) bool {
	m.waitCalls.Add(1)
	return m.waitResult.Load()
}
func (m *testNetworkMonitor) Invalidate() {
	m.invalidates.Add(1)
	m.connected.Store(false)
}
func (m *testNetworkMonitor) Stop() error {
	m.stops.Add(1)
	return nil
}

type testSleepWakeMonitor struct {
	events chan platform.SleepWakeEvent
	starts atomic.Int32
	stops  atomic.Int32
}

type testThemeMonitor struct {
	events chan platform.SystemTheme
	stops  atomic.Int32
}

func (m *testThemeMonitor) Start(context.Context) error { return nil }
func (m *testThemeMonitor) GetTheme() platform.SystemTheme {
	return platform.SystemThemeNoPreference
}
func (m *testThemeMonitor) Events() <-chan platform.SystemTheme { return m.events }
func (m *testThemeMonitor) Stop() error {
	m.stops.Add(1)
	return nil
}

func newTestSleepWakeMonitor() *testSleepWakeMonitor {
	return &testSleepWakeMonitor{events: make(chan platform.SleepWakeEvent, 4)}
}

func (m *testSleepWakeMonitor) Start(context.Context) error {
	m.starts.Add(1)
	return nil
}
func (m *testSleepWakeMonitor) Events() <-chan platform.SleepWakeEvent { return m.events }
func (m *testSleepWakeMonitor) Stop() error {
	m.stops.Add(1)
	return nil
}

func waitForBackgroundCondition(t *testing.T, description string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
}

func TestBackgroundNotificationUsesLatestConversationMetadata(t *testing.T) {
	a, accountFixture, _ := newContactOwnEmailsTestApp(t)
	a.folderStore = folder.NewStore(a.db)
	a.messageStore = message.NewStore(a.db)
	inbox := &folder.Folder{
		ID: "background-inbox", AccountID: accountFixture.ID,
		Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox, Subscribed: true,
	}
	if err := a.folderStore.Create(inbox); err != nil {
		t.Fatalf("create inbox: %v", err)
	}
	mail := &message.Message{
		ID: "background-message", AccountID: accountFixture.ID, FolderID: inbox.ID, UID: 1,
		MessageID: "background@example.com", ThreadID: "background-thread",
		Subject: "Quarterly update", FromName: "Alice", FromEmail: "alice@example.com",
		Date: time.Now().UTC(),
	}
	if err := a.messageStore.Create(mail); err != nil {
		t.Fatalf("create notification message: %v", err)
	}

	notifier := &recordingNotifier{}
	a.notifier = notifier
	info := mailSync.NewMailInfo{
		AccountID: accountFixture.ID, AccountName: "Owner account",
		FolderID: inbox.ID, Count: 1,
	}
	a.handleNewMailNotification(info)
	items := notifier.items()
	if len(items) != 1 {
		t.Fatalf("notifications = %#v", items)
	}
	if got := items[0]; got.Title != "New email from Alice" || got.Body != "Quarterly update" || got.Icon != "mail-unread" || got.Data.AccountID != accountFixture.ID || got.Data.FolderID != inbox.ID || got.Data.ThreadID != mail.ThreadID {
		t.Fatalf("single-message notification = %#v", got)
	}

	a.sendSystemNotification(info, "Fallback sender", "", "sender@example.com", "fallback-thread")
	items = notifier.items()
	if got := items[1]; got.Title != "New email from sender@example.com" || got.Body != "Fallback sender" || got.Data.ThreadID != "fallback-thread" {
		t.Fatalf("email fallback notification = %#v", got)
	}

	info.Count = 3
	a.sendSystemNotification(info, "ignored subject", "Ignored", "ignored@example.com", "")
	items = notifier.items()
	if got := items[2]; got.Title != "New emails" || got.Body != "Owner account" {
		t.Fatalf("multi-message notification = %#v", got)
	}

	notifier.showErr = errors.New("notifications unavailable")
	a.sendSystemNotification(info, "", "", "", "")
	if len(notifier.items()) != 4 {
		t.Fatal("notification failure prevented the attempted notification from being observed")
	}

	a.notifier = nil
	if got := a.currentNotifier(); got != nil {
		t.Fatalf("currentNotifier() = %#v, want nil", got)
	}
	a.sendSystemNotification(info, "", "", "", "")
}

func TestBackgroundLifecycleInitializesAndStopsWithoutEnabledAccounts(t *testing.T) {
	a, accountFixture, _ := newContactOwnEmailsTestApp(t)
	if err := a.accountStore.SetEnabled(accountFixture.ID, false); err != nil {
		t.Fatalf("disable account: %v", err)
	}
	a.folderStore = folder.NewStore(a.db)
	network := newTestNetworkMonitor()
	a.networkMonitor = network

	ctx, cancel := context.WithCancel(context.Background())
	a.ctx = ctx
	a.initBackgroundSync(ctx)
	if a.syncCoordinator == nil || a.syncScheduler == nil || a.idleManager == nil {
		t.Fatalf("background services = coordinator %#v scheduler %#v idle %#v", a.syncCoordinator, a.syncScheduler, a.idleManager)
	}
	if _, ok := a.idleManager.AccountStatus(accountFixture.ID); ok {
		t.Fatal("disabled account unexpectedly started IDLE")
	}
	cancel()
	a.syncScheduler.Stop()
	a.idleManager.Stop()
}

func TestSleepWakeProcessingClosesConnectionsAndDefersWithoutNetwork(t *testing.T) {
	network := newTestNetworkMonitor()
	network.connected.Store(true)
	network.waitResult.Store(false)
	sleepWake := newTestSleepWakeMonitor()
	idleConfig := imap.DefaultIdleConfig()
	idleConfig.ReconnectBackoff = time.Millisecond
	idleConfig.MaxReconnectBackoff = 2 * time.Millisecond
	idleConfig.MaxReconnectAttempts = 1
	idleConfig.ShutdownTimeout = 100 * time.Millisecond
	idleManager := imap.NewIdleManager(idleConfig, func(string) (*imap.ClientConfig, error) {
		return nil, errors.New("synthetic credentials unavailable")
	})
	idleManager.SetConnectivityCheck(func() bool { return false })
	idleManager.Start(context.Background())
	idleManager.StartAccount("sleep-account", "Sleep Account")
	waitForBackgroundCondition(t, "sleep account registration", func() bool {
		_, ok := idleManager.AccountStatus("sleep-account")
		return ok
	})

	a := &App{
		ctx: context.Background(), networkMonitor: network,
		sleepWakeMonitor: sleepWake, idleManager: idleManager,
	}
	a.initSyncBridge()
	done := make(chan struct{})
	go func() {
		a.processSleepWakeEvents(context.Background())
		close(done)
	}()
	sleepWake.events <- platform.SleepWakeEvent{IsSleeping: true, Timestamp: time.Now()}
	waitForBackgroundCondition(t, "sleep invalidation", func() bool {
		return network.invalidates.Load() == 1
	})
	if _, ok := idleManager.AccountStatus("sleep-account"); ok {
		t.Fatal("sleep retained an IDLE account")
	}

	sleepWake.events <- platform.SleepWakeEvent{IsSleeping: false, Timestamp: time.Now()}
	waitForBackgroundCondition(t, "wake network wait", func() bool {
		return network.waitCalls.Load() == 1
	})
	close(sleepWake.events)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sleep/wake event processor did not stop after channel close")
	}

	withoutMonitorDone := make(chan struct{})
	go func() {
		(&App{}).processSleepWakeEvents(context.Background())
		close(withoutMonitorDone)
	}()
	select {
	case <-withoutMonitorDone:
	case <-time.After(time.Second):
		t.Fatal("nil sleep/wake monitor did not return")
	}
}

func TestSyncAfterWakeCooldownAndDisabledAccountCompletion(t *testing.T) {
	a, accountFixture, _ := newContactOwnEmailsTestApp(t)
	a.ctx = context.Background()
	a.folderStore = folder.NewStore(a.db)
	now := time.Now().UTC()
	inbox := &folder.Folder{
		ID: "wake-inbox", AccountID: accountFixture.ID,
		Name: "INBOX", Path: "INBOX", Type: folder.TypeInbox,
		Subscribed: true, LastSync: &now,
	}
	if err := a.folderStore.Create(inbox); err != nil {
		t.Fatalf("create recent inbox: %v", err)
	}
	idleConfig := imap.DefaultIdleConfig()
	idleConfig.ReconnectBackoff = time.Millisecond
	idleConfig.MaxReconnectBackoff = 2 * time.Millisecond
	idleConfig.MaxReconnectAttempts = 1
	idleConfig.ShutdownTimeout = 100 * time.Millisecond
	idleManager := imap.NewIdleManager(idleConfig, func(string) (*imap.ClientConfig, error) {
		return nil, errors.New("synthetic credentials unavailable")
	})
	idleManager.SetConnectivityCheck(func() bool { return false })
	a.idleManager = idleManager

	a.syncAfterWake()
	a.SyncBridge.mu.Lock()
	wakeSyncing := a.SyncBridge.wakeSyncing
	a.SyncBridge.mu.Unlock()
	if wakeSyncing {
		t.Fatal("cooldown path left wakeSyncing set")
	}
	if _, ok := idleManager.AccountStatus(accountFixture.ID); !ok {
		t.Fatal("cooldown path did not restart IDLE for the enabled account")
	}
	idleManager.Stop()

	a.SyncBridge.mu.Lock()
	a.SyncBridge.wakeSyncing = true
	a.SyncBridge.mu.Unlock()
	a.syncAfterWake()
	a.SyncBridge.mu.Lock()
	if !a.SyncBridge.wakeSyncing {
		a.SyncBridge.mu.Unlock()
		t.Fatal("guarded syncAfterWake cleared another wake sync's guard")
	}
	a.SyncBridge.wakeSyncing = false
	a.SyncBridge.mu.Unlock()

	if err := a.accountStore.SetEnabled(accountFixture.ID, false); err != nil {
		t.Fatalf("disable account for full wake path: %v", err)
	}
	a.idleManager = nil
	inbox.LastSync = nil
	if err := a.folderStore.Update(inbox); err != nil {
		t.Fatalf("clear inbox last sync: %v", err)
	}
	a.syncAfterWake()
	waitForBackgroundCondition(t, "disabled-account wake completion", func() bool {
		a.SyncBridge.mu.Lock()
		defer a.SyncBridge.mu.Unlock()
		return !a.SyncBridge.wakeSyncing
	})

	a.restartIDLE()
}

func TestNetworkEventsWakeAndShutdownLifecycleAreObservable(t *testing.T) {
	a, accountFixture, _ := newContactOwnEmailsTestApp(t)
	if err := a.accountStore.SetEnabled(accountFixture.ID, false); err != nil {
		t.Fatalf("disable account: %v", err)
	}
	a.folderStore = folder.NewStore(a.db)
	a.ctx = context.Background()
	network := newTestNetworkMonitor()
	a.networkMonitor = network
	recorder := &actionEventRecorder{}
	a.eventsEmit = recorder.emit

	done := make(chan struct{})
	go func() {
		a.processNetworkEvents(context.Background())
		close(done)
	}()
	network.events <- platform.NetworkEvent{Connected: false, Timestamp: time.Now()}
	waitForBackgroundCondition(t, "offline event", func() bool { return recorder.has("network:offline") })
	network.connected.Store(true)
	network.events <- platform.NetworkEvent{Connected: true, Timestamp: time.Now()}
	waitForBackgroundCondition(t, "online event", func() bool { return recorder.has("network:online") })
	waitForBackgroundCondition(t, "network-triggered sync completion", func() bool {
		a.SyncBridge.mu.Lock()
		defer a.SyncBridge.mu.Unlock()
		return !a.SyncBridge.wakeSyncing
	})
	close(network.events)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("network event processor did not stop after channel close")
	}

	nilMonitorDone := make(chan struct{})
	go func() {
		(&App{}).processNetworkEvents(context.Background())
		close(nilMonitorDone)
	}()
	select {
	case <-nilMonitorDone:
	case <-time.After(time.Second):
		t.Fatal("nil network monitor did not return")
	}

	wakeNetwork := newTestNetworkMonitor()
	wakeNetwork.connected.Store(true)
	wakeNetwork.waitResult.Store(true)
	a.networkMonitor = wakeNetwork
	a.handleSystemWake()
	waitForBackgroundCondition(t, "successful wake sync completion", func() bool {
		a.SyncBridge.mu.Lock()
		defer a.SyncBridge.mu.Unlock()
		return !a.SyncBridge.wakeSyncing
	})

	sleepWake := newTestSleepWakeMonitor()
	theme := &testThemeMonitor{events: make(chan platform.SystemTheme)}
	notifier := &recordingNotifier{}
	a.sleepWakeMonitor = sleepWake
	a.setThemeMonitor(theme)
	a.notifier = notifier
	a.Shutdown(context.Background())
	if wakeNetwork.stops.Load() != 1 || sleepWake.stops.Load() != 1 || theme.stops.Load() != 1 || notifier.stops != 1 {
		t.Fatalf("shutdown stops = network:%d sleep:%d theme:%d notifier:%d", wakeNetwork.stops.Load(), sleepWake.stops.Load(), theme.stops.Load(), notifier.stops)
	}
}

func TestIdleNewMailSynchronizesHeadersAndNotifiesWithOnDemandBodies(t *testing.T) {
	fixture := newPublicActionFixture(t)
	fixture.app.syncScheduler = mailSync.NewScheduler(
		fixture.app.syncEngine,
		fixture.app.accountStore,
		fixture.app.folderStore,
	)
	notifier := &recordingNotifier{}
	fixture.app.notifier = notifier
	raw := []byte("From: Idle Sender <idle@example.com>\r\nTo: actions@example.com\r\nSubject: IDLE delivery\r\nMessage-ID: <idle-delivery@example.com>\r\nDate: Mon, 03 Aug 2026 09:00:00 +0800\r\n\r\nidle body\r\n")
	fixture.harness.append(t, raw)

	fixture.app.handleIdleNewMail(imap.MailEvent{
		Type:      imap.EventNewMail,
		AccountID: fixture.account.ID,
		Folder:    "INBOX",
		Count:     1,
	})

	rows, err := fixture.messages.ListByFolder(fixture.folders[folder.TypeInbox].ID, 0, 10)
	if err != nil || len(rows) != 1 || rows[0].Subject != "IDLE delivery" {
		t.Fatalf("IDLE synchronized rows = (%#v, %v)", rows, err)
	}
	stored, err := fixture.messages.Get(rows[0].ID)
	if err != nil || stored == nil || stored.BodyFetched {
		t.Fatalf("on-demand IDLE message = (%#v, %v)", stored, err)
	}
	items := notifier.items()
	if len(items) != 1 || items[0].Title != "New email from Idle Sender" || items[0].Body != "IDLE delivery" {
		t.Fatalf("IDLE notification = %#v", items)
	}
	if !fixture.events.has("folder:synced") {
		t.Fatalf("IDLE events = %#v", fixture.events.snapshot())
	}

	// An unknown account exercises the scheduler failure path without mutating
	// the successfully synchronized inbox or emitting a synthetic notification.
	fixture.app.handleIdleNewMail(imap.MailEvent{
		Type:      imap.EventNewMail,
		AccountID: "missing-account",
		Folder:    "INBOX",
		Count:     1,
	})
	if len(notifier.items()) != 1 {
		t.Fatalf("failed IDLE sync emitted a notification: %#v", notifier.items())
	}

	if _, err := fixture.app.db.Exec(`UPDATE accounts SET body_download_policy = ? WHERE id = ?`, account.BodyDownloadAll, fixture.account.ID); err != nil {
		t.Fatalf("enable complete body download: %v", err)
	}
	secondRaw := []byte("From: Body Sender <body@example.com>\r\nTo: actions@example.com\r\nSubject: IDLE body delivery\r\nMessage-ID: <idle-body@example.com>\r\nDate: Mon, 03 Aug 2026 09:05:00 +0800\r\n\r\ndownloaded body\r\n")
	fixture.harness.append(t, secondRaw)
	fixture.app.handleIdleNewMail(imap.MailEvent{
		Type:      imap.EventNewMail,
		AccountID: fixture.account.ID,
		Folder:    "INBOX",
		Count:     2,
	})
	waitForBackgroundCondition(t, "IDLE background body download", func() bool {
		rows, listErr := fixture.messages.ListByFolder(fixture.folders[folder.TypeInbox].ID, 0, 10)
		if listErr != nil {
			return false
		}
		for _, row := range rows {
			if row.Subject != "IDLE body delivery" {
				continue
			}
			stored, getErr := fixture.messages.Get(row.ID)
			return getErr == nil && stored != nil && stored.BodyFetched && strings.Contains(stored.BodyText, "downloaded body")
		}
		return false
	})
	if len(notifier.items()) != 2 {
		t.Fatalf("body-enabled IDLE notification = %#v", notifier.items())
	}
}
