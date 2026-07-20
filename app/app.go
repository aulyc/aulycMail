package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	goSync "sync"
	"sync/atomic"
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/activitylog"
	"aulyc.local/aulycmail/internal/appstate"
	"aulyc.local/aulycmail/internal/certificate"
	"aulyc.local/aulycmail/internal/contact"
	"aulyc.local/aulycmail/internal/contactpane"
	"aulyc.local/aulycmail/internal/credentials"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/draft"
	"aulyc.local/aulycmail/internal/folder"
	"aulyc.local/aulycmail/internal/imap"
	"aulyc.local/aulycmail/internal/logging"
	"aulyc.local/aulycmail/internal/message"
	"aulyc.local/aulycmail/internal/notification"
	"aulyc.local/aulycmail/internal/platform"
	"aulyc.local/aulycmail/internal/settings"
	"aulyc.local/aulycmail/internal/sync"
	"aulyc.local/aulycmail/internal/undo"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// MailtoData holds parsed mailto: URL data
type MailtoData struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc"`
	Bcc     []string `json:"bcc"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

// maxMailtoURLLength is the maximum allowed length for a mailto URL (2KB)
const maxMailtoURLLength = 2048

// maxEmailLength is the maximum allowed length for an email address (RFC 5321)
const maxEmailLength = 254

// maxSubjectLength is the maximum allowed length for a subject (RFC 5322 line length)
const maxSubjectLength = 998

// maxBodyLength is the maximum allowed body length (64KB)
const maxBodyLength = 64 * 1024

// ParseMailtoURL parses a mailto: URL and extracts email data with input validation.
// Returns nil if the URL is invalid or doesn't start with "mailto:".
func ParseMailtoURL(rawURL string) *MailtoData {
	if len(rawURL) > maxMailtoURLLength {
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(rawURL), "mailto:") {
		return nil
	}

	data := &MailtoData{}

	// Remove mailto: prefix
	rest := rawURL[7:]

	// Split into address part and query part
	addrPart := rest
	queryPart := ""
	if queryStart := strings.Index(rest, "?"); queryStart != -1 {
		addrPart = rest[:queryStart]
		queryPart = rest[queryStart+1:]
	}

	// Parse To addresses (comma-separated, URL-encoded)
	if addrPart != "" {
		decoded, err := url.QueryUnescape(addrPart)
		if err == nil {
			addrPart = decoded
		}
		for _, addr := range strings.Split(addrPart, ",") {
			addr = sanitizeField(strings.TrimSpace(addr))
			if addr == "" || !isValidEmail(addr) {
				continue
			}
			data.To = append(data.To, addr)
		}
	}

	// Parse query parameters
	if queryPart == "" {
		return data
	}

	params, err := url.ParseQuery(queryPart)
	if err != nil {
		return data
	}

	if subject := params.Get("subject"); subject != "" {
		subject = sanitizeField(subject)
		if len(subject) > maxSubjectLength {
			subject = subject[:maxSubjectLength]
		}
		data.Subject = subject
	}
	if body := params.Get("body"); body != "" {
		body = sanitizeField(body)
		if len(body) > maxBodyLength {
			body = body[:maxBodyLength]
		}
		data.Body = body
	}
	if cc := params.Get("cc"); cc != "" {
		for _, addr := range strings.Split(cc, ",") {
			addr = sanitizeField(strings.TrimSpace(addr))
			if addr == "" || !isValidEmail(addr) {
				continue
			}
			data.Cc = append(data.Cc, addr)
		}
	}
	if bcc := params.Get("bcc"); bcc != "" {
		for _, addr := range strings.Split(bcc, ",") {
			addr = sanitizeField(strings.TrimSpace(addr))
			if addr == "" || !isValidEmail(addr) {
				continue
			}
			data.Bcc = append(data.Bcc, addr)
		}
	}

	return data
}

// sanitizeField strips CR and LF characters to prevent header injection.
func sanitizeField(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// isValidEmail performs basic email validation: must contain @, non-empty local-part
// and domain, and total length ≤ 254 chars (RFC 5321).
func isValidEmail(email string) bool {
	if len(email) > maxEmailLength {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return false
	}
	domain := email[at+1:]
	return len(domain) > 0
}

// App struct holds the application state and dependencies
type App struct {
	// Embedded Contacts bridge. Go's method promotion makes Contacts_* methods
	// Wails-bindable while keeping contacts logic outside the host App.
	*contactpane.ContactsBridge

	ctx context.Context

	// ready is the backend-up signal the frontend polls before mounting the
	// main app. False until Startup completes. The boot splash in
	// index.html stays visible while ready is false; flipping it true is
	// what lets main.ts proceed to mount(App). See IsReady().
	ready bool

	// Paths
	paths *platform.Paths

	// Database
	db *database.DB

	// Stores
	accountStore        *account.Store
	activityLogStore    *activitylog.Store
	folderStore         *folder.Store
	messageStore        *message.Store
	attachmentStore     *message.AttachmentStore
	contactStore        *contact.Store
	draftStore          *draft.Store
	settingsStore       *settings.Store
	appStateStore       *appstate.Store
	imageAllowlistStore *settings.ImageAllowlistStore

	// IMAP
	imapPool   *imap.Pool
	syncEngine *sync.Engine

	// Background sync (polling + IDLE)
	syncScheduler *sync.Scheduler
	idleManager   *imap.IdleManager

	// Credentials (keyring with fallback)
	credStore *credentials.Store

	// Certificate trust store (TOFU)
	certStore *certificate.Store

	// Shared draft operations
	draftOps draftOps

	// Shared compose operations
	composeOps composeOps

	// Undo system
	undoStack *undo.Stack

	// Pending mailto: URL data (from command line)
	PendingMailto *MailtoData

	// Native macOS file-open requests. Finder may deliver these before the
	// frontend has mounted, so the batcher owns readiness and short batching.
	externalFileOpen *externalFileOpenBatcher

	// Full-text search indexer
	ftsIndexer *message.FTSIndexer

	// Sync management - tracks active syncs per account for cancel-and-restart
	syncContexts    map[string]context.CancelFunc // keyed by "accountID:folderID"
	syncLastRequest map[string]time.Time          // last sync request time for debounce
	syncCancelled   bool                          // set by CancelAllSyncs to stop SyncAllComplete loop
	wakeSyncing     bool                          // guards syncAfterWake against concurrent calls
	syncMu          goSync.Mutex                  // protects sync maps

	// Draft IMAP sync goroutine tracking — cancel in-flight syncDraftToIMAP
	draftSyncContexts map[string]context.CancelFunc // keyed by draft ID
	draftSyncDone     map[string]chan struct{}      // closed when goroutine exits

	// Sleep/wake detection for auto-sync on wake
	sleepWakeMonitor platform.SleepWakeMonitor

	// Network connectivity monitoring (event-driven, zero polling)
	networkMonitor platform.NetworkMonitor

	// System theme detection (XDG Settings Portal on Linux)
	themeMonitor platform.ThemeMonitor

	// Desktop notifications with click handling
	notifierMu       goSync.RWMutex
	notifier         notification.Notifier
	dockBadgeEnabled atomic.Bool

	// DebugMode function reference (injected from main)
	debugMode func() bool

	// Single-instance lock (set by main before wails.Run)
	SingleInstanceLock platform.SingleInstanceLock

	// Autostart manager
	autostartMgr platform.AutostartManager

	// Window hidden state (background mode)
	windowHidden bool
}

// NewApp creates a new App application struct
func NewApp(debugModeFn func() bool) *App {
	application := &App{
		debugMode: debugModeFn,
	}
	application.externalFileOpen = newExternalFileOpenBatcher(
		externalFileOpenDebounce,
		application.emitExternalOpenFiles,
	)
	return application
}

// StartupDialogInfo holds the user-facing dialog content for a startup
// failure: title, body, and an optional URL the dialog should expose
// behind an action button. main.go's preflight glue inspects the URL
// field to decide between ShowDialog and ShowDialogWithLink.
type StartupDialogInfo struct {
	Title       string
	Text        string
	ActionLabel string // button text; empty means no action button
	ActionURL   string // URL opened when the action button is clicked
}

// StartupDialogInfoFor returns the user-facing dialog content for a startup
// error returned by App.Preflight. Known sentinel types get a tailored
// message + action URL; everything else falls back to a generic message.
//
// URLs in the returned Text are rendered as clickable links by dialog
// backends that support markup. Other backends show the URL as selectable
// plain text and use the ActionURL field to drive an action button.
func StartupDialogInfoFor(err error) StartupDialogInfo {
	const databaseRecoveryURL = "https://aulyc.com/aulycmail/support/database-recovery"

	var schemaTooNew *database.ErrSchemaTooNew
	if errors.As(err, &schemaTooNew) {
		text := fmt.Sprintf(
			"aulycMail cannot open your database because its schema (version %d) is newer "+
				"than this build of aulycMail supports (max version %d).\n\n"+
				"This usually means you downgraded aulycMail. To recover, either reinstall "+
				"the newer version, or follow the rollback instructions to bring your "+
				"database back to version %d:\n\n"+
				"%s",
			schemaTooNew.DBVersion, schemaTooNew.BuildVersion, schemaTooNew.BuildVersion,
			databaseRecoveryURL,
		)
		return StartupDialogInfo{
			Title:       "aulycMail could not start",
			Text:        text,
			ActionLabel: "Open Help",
			ActionURL:   databaseRecoveryURL,
		}
	}
	return StartupDialogInfo{
		Title: "aulycMail could not start",
		Text:  fmt.Sprintf("aulycMail could not start.\n\nDetails: %v", err),
	}
}

// Preflight performs the early-startup steps that must succeed BEFORE the
// Wails window is shown: logging init, platform paths, directory creation,
// database open + migration, and credential store init.
// Returns an error on any failure; main.go is responsible for
// surfacing the failure to the user (via StartupDialogInfoFor + a native
// dialog) and exiting before wails.Run is called.
//
// Splitting these steps out of Startup is intentional: Wails calls Startup
// AFTER it has already created the OS window, so a failure inside Startup
// would briefly flash a half-rendered app window before the error dialog
// appears. Preflight runs in main.go before wails.Run so the user only
// ever sees the error dialog.
func (a *App) Preflight() error {
	logLevel := "fatal"
	if a.debugMode != nil && a.debugMode() {
		logLevel = "debug"
	}
	_ = logging.Init(logging.Config{
		Level:   logLevel,
		Console: true,
	})
	log := logging.WithComponent("app")

	paths, err := platform.GetPaths()
	if err != nil {
		return fmt.Errorf("get platform paths: %w", err)
	}
	a.paths = paths

	if err := paths.EnsureDirectories(); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}
	log.Info().
		Str("config", paths.Config).
		Str("data", paths.Data).
		Str("cache", paths.Cache).
		Msg("Initialized paths")

	db, err := database.Open(paths.DatabasePath())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	a.db = db
	log.Info().Str("path", paths.DatabasePath()).Msg("Opened database")

	if err := db.Migrate(); err != nil {
		return err
	}
	log.Info().Msg("Database migrations complete")

	credStore, err := credentials.NewStore(db.DB, paths.Data)
	if err != nil {
		return fmt.Errorf("init credential store: %w", err)
	}
	a.credStore = credStore

	return nil
}

// shuttingDown tracks if shutdown has been initiated to prevent multiple triggers
var shuttingDown bool

// Startup is called when the app starts
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Set single-instance onShow callback immediately — must happen before any
	// potentially-blocking D-Bus calls (theme monitor, sleep/wake, network)
	// that could delay the rest of Startup.
	if a.SingleInstanceLock != nil {
		a.SingleInstanceLock.SetOnShow(func(data string) {
			if strings.HasPrefix(data, "mailto:") {
				a.handleExternalMailto(data)
				return
			}
			a.ShowWindow()
		})
	}

	// macOS background mode: re-show the window when the app is reactivated
	// (Dock-icon click or Cmd+Tab) while it's hidden. Hiding uses orderOut,
	// which AppKit won't restore on a Dock click by itself.
	platform.SetActivationHandler(func() {
		if a.windowHidden {
			a.ShowWindow()
		}
		if notifier := a.currentNotifier(); notifier != nil {
			notifier.RefreshSettings()
		}
	})
	platform.StartActivationObserver()
	// Let a genuine quit (Dock "Quit", menu Quit, Cmd+Q — all call
	// terminate:) bypass background-mode hiding in BeforeClose.
	platform.InstallTerminateHook()

	// Logging, paths, db open, migrations, and credential store are all
	// initialized in Preflight (called from main.go before wails.Run). By
	// the time Startup runs, a.paths, a.db, and a.credStore are non-nil.
	log := logging.WithComponent("app")
	db := a.db

	// Initialize stores
	a.accountStore = account.NewStore(db)
	a.activityLogStore = activitylog.NewStore(db)
	a.folderStore = folder.NewStore(db)
	a.messageStore = message.NewStore(db)
	a.attachmentStore = message.NewAttachmentStore(db)
	a.contactStore = contact.NewStore(db.DB)
	a.draftStore = draft.NewStore(db)
	a.settingsStore = settings.NewStore(db)
	a.appStateStore = appstate.NewStore(db.DB)
	a.imageAllowlistStore = settings.NewImageAllowlistStore(db)

	// Replace the default macOS menu with a minimal App + Edit menu. Done after
	// the settings store exists, since menuLabels() reads the language setting.
	// The custom App-menu items emit events the frontend handles.
	platform.SetMenuHandler(func(action string) {
		switch action {
		case "settings":
			wailsRuntime.EventsEmit(a.ctx, "menu:openSettings")
		case "backupViewer":
			wailsRuntime.EventsEmit(a.ctx, "menu:openBackupViewer")
		case "about":
			wailsRuntime.EventsEmit(a.ctx, "menu:openAbout")
		}
	})
	platform.InstallAppMenu(a.menuLabels())
	platform.SetStatusItemHandler(func(action string) {
		switch action {
		case "show":
			a.ShowWindow()
		case "settings":
			a.ShowWindow()
			wailsRuntime.EventsEmit(a.ctx, "menu:openSettings")
		}
	})

	// Scale database connection pool based on number of accounts
	a.updateDBConnectionPool()

	// Initialize certificate trust store (TOFU)
	a.certStore = certificate.NewStore(db.DB)

	// Initialize IMAP connection pool
	poolConfig := imap.DefaultPoolConfig()
	a.imapPool = imap.NewPool(poolConfig, a.getIMAPCredentials)

	// Initialize shared draft operations
	a.draftOps = draftOps{
		accountStore: a.accountStore,
		folderStore:  a.folderStore,
		messageStore: a.messageStore,
		draftStore:   a.draftStore,
		imapPool:     a.imapPool,
	}

	// Initialize sync engine
	a.syncEngine = sync.NewEngine(a.imapPool, a.accountStore, a.folderStore, a.messageStore, a.attachmentStore, a.contactStore)

	// Set up sync progress callback to emit events to frontend
	a.syncEngine.SetProgressCallback(func(progress sync.SyncProgress) {
		wailsRuntime.EventsEmit(ctx, "sync:progress", map[string]interface{}{
			"accountId": progress.AccountID,
			"folderId":  progress.FolderID,
			"fetched":   progress.Fetched,
			"total":     progress.Total,
			"phase":     progress.Phase,
		})
	})

	// Start connection pool cleanup routine
	a.imapPool.StartCleanupRoutine(ctx)

	// Start periodic WAL checkpoint routine to prevent WAL file from growing too large
	go a.db.StartCheckpointRoutine(ctx)

	// Built-in Contacts pane. ContactsBridge is Wails-bound and lazy:
	// no contacts-specific stores are opened until a Contacts_* method is
	// actually called.
	a.initContactsBridge()

	// Initialize undo stack (max 50 commands, 30 second timeout)
	a.undoStack = undo.NewStack(50, 30*time.Second)

	// Initialize shared compose operations
	a.composeOps = composeOps{
		accountStore: a.accountStore,
		folderStore:  a.folderStore,
		credStore:    a.credStore,
		certStore:    a.certStore,
		contactStore: a.contactStore,
		messageStore: a.messageStore,
		draftOps:     &a.draftOps,
	}

	// Initialize network connectivity monitor (event-driven, zero polling).
	// Must be initialized before background sync so scheduler and IDLE
	// can use it to skip operations when offline.
	a.initNetworkMonitor(ctx)

	// Initialize and start background email sync (polling + IDLE)
	a.initBackgroundSync(ctx)

	// Seed native unread badges with the count carried over from the last
	// session, before the first sync refreshes them.
	a.refreshUnreadBadges()

	// Sync any pending drafts from previous sessions
	go a.syncAllPendingDrafts()

	// Initialize FTS indexer for full-text search
	a.ftsIndexer = message.NewFTSIndexer(db.DB)

	// Initialize sync context tracking for cancel-and-restart
	a.syncContexts = make(map[string]context.CancelFunc)
	a.syncLastRequest = make(map[string]time.Time)
	a.draftSyncContexts = make(map[string]context.CancelFunc)
	a.draftSyncDone = make(map[string]chan struct{})

	// IMPORTANT: backend-ready signal. The frontend's main.ts waits for the
	// "app:ready" event (with IsReady() as a one-shot fallback) and will NOT
	// mount the main app until that event fires. If you remove, reorder, or
	// skip these two lines, the UI will never load — the boot splash will
	// stay visible forever.
	//
	// Placement: BEFORE desktop integration inits below (initNotifications,
	// initSleepWakeMonitor, initThemeMonitor). Those calls are best-effort
	// system integration, NOT prerequisites for the frontend. At this point the
	// frontend has everything it needs: stores constructed, migrations applied,
	// built-in panes registered, network monitor up, IPC server running,
	// background sync started.
	//
	// We do NOT poll IsReady from the frontend because Wails' IPC bridge is
	// unnecessary for high-rate polling. Use the event for the normal case and
	// IsReady for the "event fired before listener registered" race.
	a.ready = true
	wailsRuntime.EventsEmit(a.ctx, "app:ready")

	// Initialize desktop notifications with click handling
	a.initNotifications(ctx)

	// Initialize sleep/wake monitor for auto-sync on wake
	a.initSleepWakeMonitor(ctx)

	// Initialize system theme monitor
	a.initThemeMonitor(ctx)

	// Set up FTS progress callback to emit events to frontend
	a.ftsIndexer.SetProgressCallback(func(folderID string, indexed, total int) {
		percentage := 0
		if total > 0 {
			percentage = (indexed * 100) / total
		}
		wailsRuntime.EventsEmit(ctx, "fts:progress", map[string]interface{}{
			"folderId":   folderID,
			"indexed":    indexed,
			"total":      total,
			"percentage": percentage,
		})
	})

	a.ftsIndexer.SetCompleteCallback(func(folderID string) {
		wailsRuntime.EventsEmit(ctx, "fts:complete", map[string]interface{}{
			"folderId": folderID,
		})
	})

	// Start background FTS indexing after a short delay to let initial sync complete
	go func() {
		defer recoverPanic("app", "FTS indexing")
		time.Sleep(5 * time.Second)
		log.Info().Msg("Starting background FTS indexing")
		wailsRuntime.EventsEmit(ctx, "fts:indexing", map[string]interface{}{
			"status": "started",
		})
		if err := a.ftsIndexer.IndexAllFolders(ctx); err != nil {
			log.Error().Err(err).Msg("Background FTS indexing failed")
		} else {
			log.Info().Msg("Background FTS indexing completed")
			wailsRuntime.EventsEmit(ctx, "fts:indexing", map[string]interface{}{
				"status": "completed",
			})
		}
	}()

	// Initialize autostart manager
	a.autostartMgr = platform.NewAutostartManager()

	log.Info().Msg("aulycMail started successfully")
}

// IsReady reports whether Startup has fully completed. The frontend calls
// this ONCE at boot as a safety net for the "Go emitted app:ready before
// the frontend listener registered" race. Always safe to call: reads a
// bool field, no nil dereference possible, fires regardless of which
// stores are or aren't initialized.
//
// IMPORTANT: do NOT call this in a polling loop — it saturates the Wails
// IPC bridge. The frontend should use EventsOn('app:ready', ...) for the
// normal path; IsReady() is a one-shot check only.
func (a *App) IsReady() bool {
	return a.ready
}

// BeforeClose is called when the window is about to close (e.g., OS close signal)
func (a *App) BeforeClose(ctx context.Context) bool {
	if shuttingDown {
		return false
	}

	// Background mode: hide window instead of quitting — UNLESS this is a
	// genuine quit (Dock "Quit" / menu Quit / Cmd+Q all call terminate:,
	// which the macOS hook flags). A real quit must fall through to shutdown.
	runBg, _ := a.settingsStore.GetRunBackground()
	if runBg && !platform.RealQuitRequested() {
		log := logging.WithComponent("app")
		log.Info().Msg("Window close requested, hiding to background")
		wailsRuntime.WindowHide(a.ctx)
		a.windowHidden = true
		return true
	}

	// Normal shutdown flow
	log := logging.WithComponent("app")
	log.Info().Msg("Window close requested, shutting down")

	shuttingDown = true

	// Quit immediately (deferred to a goroutine so this close callback can
	// return first; no artificial delay, so it's as snappy as any other app).
	go func() {
		defer recoverPanic("app", "shutdown")
		wailsRuntime.Quit(a.ctx)
	}()

	// Prevent immediate close
	return true
}

// NotifyStartupComplete signals the desktop environment that startup is done.
// Called from the frontend after WindowShow() so KDE/Plasma sees the placeholder
// → real window handoff cleanly (avoiding the taskbar-icon flash from #154).
func (a *App) NotifyStartupComplete() {
	platform.NotifyStartupComplete()
	if a.externalFileOpen != nil {
		a.externalFileOpen.SetReady()
	}
}

// ShowWindow brings the window to the foreground from hidden/minimized state.
// Used by single-instance activation, notification clicks, etc.
func (a *App) ShowWindow() {
	log := logging.WithComponent("app")
	log.Info().Msg("Showing window")

	wailsRuntime.WindowUnminimise(a.ctx)
	wailsRuntime.WindowShow(a.ctx)
	a.windowHidden = false

	// Emit event so frontend can also attempt to focus
	wailsRuntime.EventsEmit(a.ctx, "window:show")
}

// QuitApp forces a real quit, bypassing background mode.
// Used by frontend or future tray menu to actually exit.
func (a *App) QuitApp() {
	if shuttingDown {
		return
	}
	shuttingDown = true

	log := logging.WithComponent("app")
	log.Info().Msg("Quit requested")
	go func() {
		defer recoverPanic("app", "shutdown")
		wailsRuntime.Quit(a.ctx)
	}()
}

// GetStartHiddenActive always returns false — the "start hidden" option was
// removed; the window is always shown on startup.
func (a *App) GetStartHiddenActive() bool {
	return false
}

// Shutdown is called when the app is closing
func (a *App) Shutdown(ctx context.Context) {
	log := logging.WithComponent("app")
	if a.externalFileOpen != nil {
		a.externalFileOpen.Stop()
	}

	// Stop email sync scheduler
	if a.syncScheduler != nil {
		a.syncScheduler.Stop()
		log.Info().Msg("Email sync scheduler stopped")
	}

	// Stop IDLE manager
	if a.idleManager != nil {
		a.idleManager.Stop()
		log.Info().Msg("IDLE manager stopped")
	}

	// Stop sleep/wake monitor
	if a.sleepWakeMonitor != nil {
		_ = a.sleepWakeMonitor.Stop()
		log.Info().Msg("Sleep/wake monitor stopped")
	}

	// Stop network monitor
	if a.networkMonitor != nil {
		_ = a.networkMonitor.Stop()
		log.Info().Msg("Network monitor stopped")
	}

	// Stop theme monitor
	if a.themeMonitor != nil {
		_ = a.themeMonitor.Stop()
		log.Info().Msg("Theme monitor stopped")
	}

	// Stop notification listener
	if notifier := a.currentNotifier(); notifier != nil {
		notifier.Stop()
		log.Info().Msg("Notification listener stopped")
	}

	// Close all IMAP connections
	if a.imapPool != nil {
		a.imapPool.CloseAll()
		log.Info().Msg("IMAP connections closed")
	}

	if a.db != nil {
		a.db.Close()
		log.Info().Msg("Database closed")
	}

	log.Info().Msg("aulycMail shutdown complete")
}

// updateDBConnectionPool scales the database connection pool based on account count.
// This should be called at startup and whenever accounts are added or removed.
// It also refreshes the contact store's own-address set so auto-collection never
// harvests the user's own mailboxes or sender identities into the address book.
func (a *App) updateDBConnectionPool() {
	accounts, err := a.accountStore.List()
	if err != nil {
		// On error, use a reasonable default
		a.db.UpdateIdleConns(0)
	} else {
		a.db.UpdateIdleConns(len(accounts))
	}

	if _, err := a.refreshContactOwnEmails(); err != nil {
		log := logging.WithComponent("app")
		log.Warn().Err(err).Msg("Failed to refresh contact own-address exclusions")
	}
}

// refreshContactOwnEmails atomically replaces the contact store's exclusion
// set with all configured mailbox and sender-identity addresses, then removes
// historical contact rows for those addresses. If the query fails,
// SetOwnEmails is not called, preserving the previous known-good set.
func (a *App) refreshContactOwnEmails() ([]string, error) {
	if a.accountStore == nil || a.contactStore == nil {
		return nil, fmt.Errorf("contact own-address stores are not ready")
	}
	emails, err := a.accountStore.ListOwnEmailAddresses()
	if err != nil {
		return nil, err
	}
	a.contactStore.SetOwnEmails(emails)
	for _, email := range emails {
		if err := a.contactStore.PurgeOwnEmail(email); err != nil {
			return nil, fmt.Errorf("failed to purge own address from contacts: %w", err)
		}
	}
	return emails, nil
}

func (a *App) refreshContactOwnEmailsBestEffort(operation string) {
	if _, err := a.refreshContactOwnEmails(); err != nil {
		log := logging.WithComponent("app")
		log.Warn().Err(err).Str("operation", operation).Msg("Failed to refresh contact own-address exclusions")
	}
}

// getIMAPCredentials returns password IMAP credentials for an account.
func (a *App) getIMAPCredentials(accountID string) (*imap.ClientConfig, error) {
	return a.composeOps.getIMAPCredentials(accountID)
}

// menuLabels returns the localized strings for the native macOS menu. The menu
// is built once at startup; changing the language takes effect after a restart.
// Defaults to Chinese (this build's primary language) when no language is set.
func (a *App) menuLabels() platform.MenuLabels {
	zh := true
	if lang, err := a.GetLanguage(); err == nil && lang != "" && !strings.HasPrefix(lang, "zh") {
		zh = false
	}
	if zh {
		return platform.MenuLabels{
			Settings: "设置", BackupViewer: "备份查看器", About: "关于", Quit: "退出",
			Edit: "编辑", Undo: "撤销", Redo: "重做",
			Cut: "剪切", Copy: "复制", Paste: "粘贴", Delete: "删除",
		}
	}
	return platform.MenuLabels{
		Settings: "Settings", BackupViewer: "Backup Viewer", About: "About", Quit: "Quit",
		Edit: "Edit", Undo: "Undo", Redo: "Redo",
		Cut: "Cut", Copy: "Copy", Paste: "Paste", Delete: "Delete",
	}
}

// statusItemLabels returns localized strings for the macOS menu bar status item.
func (a *App) statusItemLabels() platform.StatusItemLabels {
	zh := true
	if lang, err := a.GetLanguage(); err == nil && lang != "" && !strings.HasPrefix(lang, "zh") {
		zh = false
	}
	if zh {
		return platform.StatusItemLabels{
			Open: "打开 aulycMail", Settings: "设置", Quit: "退出",
		}
	}
	return platform.StatusItemLabels{
		Open: "Open aulycMail", Settings: "Settings", Quit: "Quit",
	}
}

// OpenURL opens a URL in the system browser with proper shell escaping
// This bypasses Wails' BrowserOpenURL which has strict validation against shell metacharacters
func (a *App) OpenURL(url string) error {
	log := logging.WithComponent("app")
	log.Debug().Str("url", redactURLForLog(url)).Msg("Opening URL in system browser")

	// Validate URL and check protocol for security
	// This prevents file:// URLs and other potentially dangerous schemes
	if url == "" {
		return fmt.Errorf("empty URL")
	}

	// Allow common safe protocols
	// Note: We're being permissive here to allow legitimate email links
	// The main security comes from using exec.Command properly
	if !isAllowedProtocol(url) {
		log.Warn().Str("url", redactURLForLog(url)).Msg("Rejecting URL with disallowed protocol")
		return fmt.Errorf("URL protocol not allowed for security reasons")
	}

	if runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Start the command without waiting for it to complete
	// Browser opening should be async - we don't need to wait
	cmd := exec.Command("open", url)
	if err := cmd.Start(); err != nil {
		log.Error().Err(err).Str("url", redactURLForLog(url)).Msg("Failed to open URL in browser")
		return fmt.Errorf("failed to open URL: %w", err)
	}

	log.Debug().Str("url", redactURLForLog(url)).Msg("Successfully started browser process")
	return nil
}

func redactURLForLog(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Sprintf("[invalid-url length=%d]", len(rawURL))
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "mailto":
	default:
		if parsed.Scheme == "" {
			return fmt.Sprintf("[relative-or-invalid-url length=%d]", len(rawURL))
		}
		return parsed.Scheme + ":[redacted]"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.String()
}

// isAllowedProtocol checks if a URL uses an allowed protocol
// Prevents file:// URLs and other potentially dangerous schemes
func isAllowedProtocol(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	case "mailto":
		return true
	default:
		return false
	}
}
