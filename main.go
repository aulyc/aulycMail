package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aulyc/aulycmail/app"
	"github.com/aulyc/aulycmail/internal/platform"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

// Command-line flags
var (
	debugMode   = flag.Bool("debug", false, "Enable debug logging")
	dbusNotify  = flag.Bool("dbus-notify", false, "Use direct D-Bus notifications instead of portal (Linux only)")
	versionFlag = flag.Bool("version", false, "Show version and exit")
)

// DebugMode returns whether debug logging is enabled
// Can be enabled via --debug flag or AULYCMAIL_DEBUG=1 environment variable
func DebugMode() bool {
	return *debugMode || os.Getenv("AULYCMAIL_DEBUG") == "1"
}

func main() {
	platform.MonitorGBMErrors()
	flag.Parse()

	if *versionFlag {
		fmt.Println(app.Version)
		return
	}

	// On Windows, GUI apps have no console. Allocate one for debug output.
	if DebugMode() {
		platform.AttachConsole()
	}

	// Check for mailto: URL in non-flag arguments
	var mailtoData *app.MailtoData
	var rawMailtoArg string
	args := flag.Args()
	for _, arg := range args {
		if strings.HasPrefix(strings.ToLower(arg), "mailto:") {
			mailtoData = app.ParseMailtoURL(arg)
			rawMailtoArg = arg
			break
		}
	}

	runMainMode(mailtoData, rawMailtoArg)
}

// runMainMode runs the main application window
func runMainMode(mailtoData *app.MailtoData, rawMailtoArg string) {
	// Determine activation message: pass raw mailto URL if present, otherwise just "show"
	activateMsg := "show"
	if rawMailtoArg != "" {
		activateMsg = rawMailtoArg
	}

	// Single-instance detection: if another instance is running, activate it and exit
	lock := platform.NewSingleInstanceLock()
	locked, err := lock.TryLock(activateMsg)
	if err != nil {
		println("Warning: single-instance check failed:", err.Error())
	}
	if !locked {
		// Existing instance was activated
		return
	}
	defer lock.Unlock()

	// Title bar is hardcoded to the OS-native chrome — Frameless off.
	nativeTitleBar := true

	// Create an instance of the app structure
	application := app.NewApp(DebugMode, *dbusNotify)
	application.SingleInstanceLock = lock

	// Store mailto data if provided (will be used after startup)
	if mailtoData != nil {
		application.PendingMailto = mailtoData
	}

	// Run pre-Wails startup checks (paths, DB open, migrations, credential
	// store). On failure, surface a native error dialog and exit before the
	// Wails window is created — otherwise the user would see a half-rendered
	// app window briefly flash before the dialog appears.
	//
	// Skipped under the `bindings` build tag — see preflight_bindings.go.
	runPreflight(application)

	// Create application with options
	err = wails.Run(&options.App{
		Title:                    "aulycmail",
		Width:                    1300,
		Height:                   800,
		// Floor for the narrowest pane layout (rail + sidebar-min + list-min +
		// viewer toolbar). The frontend raises this dynamically when the panes
		// are widened so the viewer's action toolbar always fits.
		MinWidth:                 1138,
		MinHeight:                400,
		Frameless:                !nativeTitleBar,
		StartHidden:              true, // Hide until frontend is ready to prevent white flash
		EnableDefaultContextMenu: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        application.Startup,
		OnShutdown:       application.Shutdown,
		OnBeforeClose:    application.BeforeClose,
		Bind: []interface{}{
			application,
		},
		Linux: &linux.Options{
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
			ProgramName:      "aulycmail",
		},
		// Provide a Mac options block so the green traffic-light zoom/maximize
		// button stays enabled — Wails leaves it disabled when Mac is nil
		// (DisableZoom defaults to false here, so zoom is on).
		Mac: &mac.Options{},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
