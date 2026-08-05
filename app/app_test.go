package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aulyc.local/aulycmail/internal/credentials"
	"aulyc.local/aulycmail/internal/database"
	"aulyc.local/aulycmail/internal/settings"
)

func TestParseMailtoURL_Valid(t *testing.T) {
	result := ParseMailtoURL("mailto:test@example.com")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if len(result.To) != 1 || result.To[0] != "test@example.com" {
		t.Errorf("To = %v, want [test@example.com]", result.To)
	}
}

func TestParseMailtoURL_WithParams(t *testing.T) {
	result := ParseMailtoURL("mailto:test@example.com?subject=Hello&body=World")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if result.Subject != "Hello" {
		t.Errorf("Subject = %q, want %q", result.Subject, "Hello")
	}
	if result.Body != "World" {
		t.Errorf("Body = %q, want %q", result.Body, "World")
	}
}

func TestParseMailtoURL_MultiRecipient(t *testing.T) {
	result := ParseMailtoURL("mailto:a@b.com,c@d.com")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if len(result.To) != 2 {
		t.Fatalf("len(To) = %d, want 2", len(result.To))
	}
	if result.To[0] != "a@b.com" {
		t.Errorf("To[0] = %q, want %q", result.To[0], "a@b.com")
	}
	if result.To[1] != "c@d.com" {
		t.Errorf("To[1] = %q, want %q", result.To[1], "c@d.com")
	}
}

func TestParseMailtoURL_WithCcBcc(t *testing.T) {
	result := ParseMailtoURL("mailto:a@b.com?cc=c@d.com&bcc=e@f.com")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if len(result.Cc) != 1 || result.Cc[0] != "c@d.com" {
		t.Errorf("Cc = %v, want [c@d.com]", result.Cc)
	}
	if len(result.Bcc) != 1 || result.Bcc[0] != "e@f.com" {
		t.Errorf("Bcc = %v, want [e@f.com]", result.Bcc)
	}
}

func TestParseMailtoURL_TooLong(t *testing.T) {
	longURL := "mailto:test@example.com?" + strings.Repeat("x", 3000)
	result := ParseMailtoURL(longURL)
	if result != nil {
		t.Error("ParseMailtoURL with 3000+ char URL should return nil")
	}
}

func TestParseMailtoURL_InvalidScheme(t *testing.T) {
	result := ParseMailtoURL("http://example.com")
	if result != nil {
		t.Error("ParseMailtoURL with http:// scheme should return nil")
	}
}

func TestParseMailtoURL_Empty(t *testing.T) {
	result := ParseMailtoURL("")
	if result != nil {
		t.Error("ParseMailtoURL with empty string should return nil")
	}
}

func TestParseMailtoURL_NoAddress(t *testing.T) {
	result := ParseMailtoURL("mailto:?subject=Hello")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if len(result.To) != 0 {
		t.Errorf("To = %v, want empty", result.To)
	}
	if result.Subject != "Hello" {
		t.Errorf("Subject = %q, want %q", result.Subject, "Hello")
	}
}

func TestRedactURLForLogRemovesSensitiveParts(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"https query", "https://example.com/reset?token=secret#frag", "https://example.com/reset"},
		{"userinfo", "https://user:pass@example.com/path?x=1", "https://example.com/path"},
		{"mailto query", "mailto:user@example.com?subject=secret&body=token", "mailto:user@example.com"},
		{"disallowed scheme", "data:text/html,secret-token", "data:[redacted]"},
		{"relative", "/reset?token=secret", "[relative-or-invalid-url length=19]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactURLForLog(tt.in); got != tt.want {
				t.Fatalf("redactURLForLog(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsAllowedProtocolParsesScheme(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"https://example.com/path?token=secret", true},
		{"HTTPS://example.com/path", true},
		{"http:example.com/path", false},
		{"mailto:user@example.com?subject=Hi", true},
		{"file:///tmp/secret", false},
		{"data:text/html,secret", false},
		{"/relative/path", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isAllowedProtocol(tt.in); got != tt.want {
				t.Fatalf("isAllowedProtocol(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestMailtoValidationSanitizationAndBounds(t *testing.T) {
	result := ParseMailtoURL("MAILTO:valid%40example.com,missing-at,@domain.example,local@?subject=hello%0D%0Aworld&cc=copy@example.com,invalid&bcc=blind@example.com")
	if result == nil {
		t.Fatal("validated mailto returned nil")
	}
	if len(result.To) != 1 || result.To[0] != "valid@example.com" {
		t.Fatalf("validated recipients = %#v", result.To)
	}
	if result.Subject != "helloworld" {
		t.Fatalf("sanitized subject = %q", result.Subject)
	}
	if len(result.Cc) != 1 || len(result.Bcc) != 1 {
		t.Fatalf("validated cc/bcc = %#v / %#v", result.Cc, result.Bcc)
	}

	longSubject := strings.Repeat("s", maxSubjectLength+20)
	result = ParseMailtoURL("mailto:user@example.com?subject=" + longSubject)
	if result == nil || len(result.Subject) != maxSubjectLength {
		t.Fatalf("bounded subject length = %d", len(result.Subject))
	}

	result = ParseMailtoURL("mailto:user@example.com?broken=%zz")
	if result == nil || len(result.To) != 1 || result.Subject != "" {
		t.Fatalf("malformed query result = %#v", result)
	}
	if ParseMailtoURL("mailto:%zz") == nil {
		t.Fatal("malformed address escaping should preserve a valid empty mailto result")
	}
}

func TestAppCoreDefaultsEventsDialogsAndLocalizedNativeLabels(t *testing.T) {
	a := NewApp(nil)
	t.Cleanup(func() { a.externalFileOpen.Stop() })
	if a.IsReady() || a.GetStartHiddenActive() || a.externalFileOpen == nil {
		t.Fatalf("NewApp defaults = ready:%v startHidden:%v batcher:%#v", a.IsReady(), a.GetStartHiddenActive(), a.externalFileOpen)
	}
	recorder := &actionEventRecorder{}
	a.ctx = context.Background()
	a.eventsEmit = recorder.emit
	a.emitEvent("test:event", map[string]any{"ok": true})
	if !recorder.has("test:event") {
		t.Fatalf("injected event transport did not receive event: %#v", recorder.snapshot())
	}
	a.ready.Store(true)
	if !a.IsReady() {
		t.Fatal("IsReady did not expose initialized state")
	}

	known := StartupDialogInfoFor(&database.ErrSchemaTooNew{DBVersion: 42, BuildVersion: 41})
	if known.ActionLabel != "Open Help" || !strings.Contains(known.Text, "version 42") || !strings.Contains(known.ActionURL, "database-recovery") {
		t.Fatalf("schema-too-new dialog = %#v", known)
	}
	generic := StartupDialogInfoFor(errors.New("synthetic startup failure"))
	if generic.ActionLabel != "" || !strings.Contains(generic.Text, "synthetic startup failure") {
		t.Fatalf("generic startup dialog = %#v", generic)
	}

	configured, _, _ := newContactOwnEmailsTestApp(t)
	configured.settingsStore = settings.NewStore(configured.db)
	if labels := configured.menuLabels(); labels.Settings != "设置" || labels.Quit != "退出" {
		t.Fatalf("default menu labels = %#v", labels)
	}
	if labels := configured.statusItemLabels(); labels.Open != "打开 aulycMail" {
		t.Fatalf("default status labels = %#v", labels)
	}
	if err := configured.SetLanguage("en"); err != nil {
		t.Fatalf("SetLanguage(en): %v", err)
	}
	if labels := configured.menuLabels(); labels.Settings != "Settings" || labels.Quit != "Quit" {
		t.Fatalf("English menu labels = %#v", labels)
	}
	if labels := configured.statusItemLabels(); labels.Open != "Open aulycMail" {
		t.Fatalf("English status labels = %#v", labels)
	}
	(&App{}).refreshContactOwnEmailsBestEffort("test")
}

func TestOpenURLRejectsUnsafeAndIncompleteTargetsBeforeLaunching(t *testing.T) {
	a := &App{}
	for _, target := range []string{"", "file:///tmp/message.eml", "javascript:alert(1)", "https:///missing-host", "relative/path"} {
		if err := a.OpenURL(target); err == nil {
			t.Fatalf("OpenURL(%q) unexpectedly succeeded", target)
		}
	}
}

func TestAppPreflightInitializesPrivateLocalStateWithoutHostCredentials(t *testing.T) {
	for _, debug := range []bool{false, true} {
		t.Run(map[bool]string{false: "fatal logging", true: "debug logging"}[debug], func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			a := NewApp(func() bool { return debug })
			t.Cleanup(func() { a.externalFileOpen.Stop() })

			factoryCalled := false
			a.newCredentialStore = func(db *database.DB, dataDir string) (*credentials.Store, error) {
				factoryCalled = true
				if db == nil || dataDir != filepath.Join(home, "Library", "Application Support", "aulycmail") {
					t.Fatalf("credential factory inputs = db:%#v data:%q", db, dataDir)
				}
				return &credentials.Store{}, nil
			}

			if err := a.preflight(); err != nil {
				t.Fatalf("Preflight() error = %v", err)
			}
			t.Cleanup(func() { _ = a.db.Close() })
			if !factoryCalled || a.paths == nil || a.db == nil || a.credStore == nil {
				t.Fatalf("preflight state = paths:%#v db:%#v credentials:%#v factory:%v", a.paths, a.db, a.credStore, factoryCalled)
			}
			for _, dir := range []string{a.paths.Config, a.paths.Data, a.paths.Cache} {
				info, err := os.Stat(dir)
				if err != nil || !info.IsDir() {
					t.Fatalf("initialized directory %q = %#v, %v", dir, info, err)
				}
			}
			if got := a.paths.DatabasePath(); got != filepath.Join(a.paths.Data, "aulycmail.db") {
				t.Fatalf("database path = %q", got)
			}
			var schemaVersion int
			if err := a.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM migrations`).Scan(&schemaVersion); err != nil || schemaVersion == 0 {
				t.Fatalf("schema version = %d, %v", schemaVersion, err)
			}
		})
	}

	t.Run("credential initialization failure is surfaced", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		a := NewApp(nil)
		t.Cleanup(func() { a.externalFileOpen.Stop() })
		a.newCredentialStore = func(*database.DB, string) (*credentials.Store, error) {
			return nil, errors.New("synthetic credential failure")
		}
		err := a.preflight()
		if err == nil || !strings.Contains(err.Error(), "init credential store: synthetic credential failure") {
			t.Fatalf("Preflight() credential error = %v", err)
		}
		if a.db != nil {
			_ = a.db.Close()
		}
	})
}

func TestHandleExternalMailtoUsesWindowAndEventSeams(t *testing.T) {
	recorder := &actionEventRecorder{}
	windowsShown := 0
	a := &App{
		ctx:              context.Background(),
		eventsEmit:       recorder.emit,
		showWindowAction: func() { windowsShown++ },
	}

	a.handleExternalMailto("https://example.com")
	if windowsShown != 0 || recorder.has("mailto:external") {
		t.Fatalf("invalid mailto changed UI state: windows=%d events=%v", windowsShown, recorder.snapshot())
	}
	a.handleExternalMailto("mailto:person@example.com?subject=Hello&body=World")
	if windowsShown != 1 || !recorder.has("mailto:external") {
		t.Fatalf("valid mailto state: windows=%d events=%v", windowsShown, recorder.snapshot())
	}
}
