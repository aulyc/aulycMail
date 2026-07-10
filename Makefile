# aulycmail Email Client - Build System (macOS-only slim build)
#
# Usage:
#   make build    - Build production binary (aulycmail.app)
#   make dev      - Run in development mode
#   make help     - Show all available targets

.PHONY: all build dev dev-race generate clean test lint lint-go lint-frontend \
        fmt frontend-deps frontend-update normalize-wails-bindings install uninstall \
        dmg release-dmg install-dmg install-release-dmg install-darwin \
        quit-running-darwin launch-darwin uninstall-darwin help

# Go module path
MODULE := aulyc.local/aulycmail

# aulycmail is a password-auth mail client; no build-time credentials are injected.
LDFLAGS :=

# Wails build tags
BUILD_TAGS := webkit2_41
GO_BUILD_TAGS := desktop,$(BUILD_TAGS),wv2runtime.download,production
APP_BUNDLE := build/bin/aulycmail.app
APP_BINARY := build/bin/aulycmail
DMG_PATH := dist/aulycmail.dmg
DMG_VOLUME_NAME ?= aulycmail Installer
SIGN_IDENTITY ?=
NOTARY_PROFILE ?=

# Darwin cgo/linker flags keep build output actionable with current Xcode SDKs:
# - suppress duplicate libobjc linker noise from multiple Objective-C packages
# - link UniformTypeIdentifiers explicitly for Wails' macOS file dialog code
# - use one deployment target across cgo objects and the final link
# - silence Wails' macOS 15+ NSToolbar deprecation warning until Wails removes
#   its use of setShowsBaselineSeparator:
DARWIN_LINK_WARN_ENV :=
ifeq ($(shell uname -s),Darwin)
DARWIN_MIN_VERSION := 11.0
DARWIN_LINK_WARN_ENV := \
	CGO_CFLAGS_ALLOW='-mmacosx-version-min=.*|-Wno-deprecated-declarations' \
	CGO_CFLAGS='-mmacosx-version-min=$(DARWIN_MIN_VERSION) -Wno-deprecated-declarations' \
	CGO_LDFLAGS_ALLOW='-Wl,-no_warn_duplicate_libraries|-framework|Cocoa|Network|IOKit|CoreFoundation|Foundation|UserNotifications|Security|WebKit|AppKit|UniformTypeIdentifiers|-mmacosx-version-min=.*' \
	CGO_LDFLAGS='-Wl,-no_warn_duplicate_libraries -framework UniformTypeIdentifiers -mmacosx-version-min=$(DARWIN_MIN_VERSION)'
endif

# Go package directories. Do not use `go test ./...` directly: npm packages can
# vendor their own Go modules under frontend/node_modules, and those should not
# become part of this repository's Go test surface.
GO_PACKAGES := $(shell find . \
	-path './frontend/node_modules' -prune -o \
	-path './frontend/dist' -prune -o \
	-name '*.go' -print | xargs -n1 dirname | sort -u | sed 's,^\./,./,')

# Default target
all: build

## Build Targets

# Build production binary (aulycmail.app) and ad-hoc sign it
# (ad-hoc signature is required for macOS notifications to work).
build:
	@echo "Building aulycmail..."
	$(DARWIN_LINK_WARN_ENV) wails generate module
	@tools/normalize_wails_bindings.sh
	@if [ ! -d frontend/node_modules ]; then \
		echo "Installing frontend dependencies..."; \
		cd frontend && npm install; \
	else \
		echo "Skipping npm install"; \
	fi
	@echo "Compiling frontend..."
	cd frontend && npm run build
	@echo "Compiling application..."
	mkdir -p build/bin
	$(DARWIN_LINK_WARN_ENV) go build -trimpath -buildvcs=false -tags $(GO_BUILD_TAGS) -ldflags "$(LDFLAGS) -s -w -w -s" -o $(APP_BINARY)
	@echo "Packaging macOS app bundle..."
	bash tools/package_macos_app.sh
	@echo "Injecting macOS asset-catalog icon (fills the Liquid Glass plate on macOS 26)..."
	bash tools/inject_macos_icon.sh $(APP_BUNDLE) build/appicon.png
	@echo "Ad-hoc signing aulycmail.app (required for macOS notifications)..."
	@codesign_log=$$(mktemp); \
	if codesign --force --deep --sign - $(APP_BUNDLE) >"$$codesign_log" 2>&1; then \
		rm -f "$$codesign_log"; \
	else \
		cat "$$codesign_log"; \
		rm -f "$$codesign_log"; \
		exit 1; \
	fi

# Run in development mode with hot reload
dev:
	@echo "Starting aulycmail in development mode..."
	$(DARWIN_LINK_WARN_ENV) wails dev -ldflags "$(LDFLAGS)" -tags $(BUILD_TAGS)

# Run in development mode with Go's race detector enabled. Builds significantly
# slower and adds ~5-10x runtime overhead, but instruments every memory access
# and prints exactly which line + goroutines collide on any unsynchronized
# shared-memory access. Use this when chasing a suspected data race —
# reproduce the crash and the detector report points right at it.
dev-race:
	@echo "Starting aulycmail in development mode with -race..."
	$(DARWIN_LINK_WARN_ENV) wails dev -ldflags "$(LDFLAGS)" -tags $(BUILD_TAGS) -race

# Generate Wails TypeScript bindings
generate:
	@echo "Generating Wails bindings..."
	wails generate module
	@tools/normalize_wails_bindings.sh

# Normalize generated Wails bindings so local generation does not create
# whitespace-only diffs.
normalize-wails-bindings:
	@tools/normalize_wails_bindings.sh

# Package the current app bundle as a drag-to-Applications DMG. Pass
# SIGN_IDENTITY to sign the staged app and DMG, and NOTARY_PROFILE to notarize.
dmg:
	@./tools/package_macos_dmg.sh --output "$(DMG_PATH)" \
		--volume-name "$(DMG_VOLUME_NAME)" \
		$(if $(SIGN_IDENTITY),--sign "$(SIGN_IDENTITY)") \
		$(if $(NOTARY_PROFILE),--notary-profile "$(NOTARY_PROFILE)")

# Build, Developer ID sign, notarize, and staple a release DMG.
release-dmg: build
	@if [ -z "$(SIGN_IDENTITY)" ]; then \
		echo 'SIGN_IDENTITY is required, e.g. make release-dmg SIGN_IDENTITY="Developer ID Application: nan ma (M9M7M2ARFD)" NOTARY_PROFILE=aulyc-notary'; \
		exit 1; \
	fi
	@if [ -z "$(NOTARY_PROFILE)" ]; then \
		echo 'NOTARY_PROFILE is required, e.g. NOTARY_PROFILE=aulyc-notary'; \
		exit 1; \
	fi
	@./tools/package_macos_dmg.sh --output "$(DMG_PATH)" \
		--volume-name "$(DMG_VOLUME_NAME)" \
		--sign "$(SIGN_IDENTITY)" \
		--notary-profile "$(NOTARY_PROFILE)"

# Install the current signed/notarized DMG into /Applications and launch it.
install-dmg: quit-running-darwin
	@./tools/install_macos_dmg.sh --dmg "$(DMG_PATH)"

# Build a signed/notarized release DMG, then install that DMG locally.
install-release-dmg: release-dmg install-dmg

## Code Quality

# Run Go tests
test:
	@echo "Running tests..."
	$(DARWIN_LINK_WARN_ENV) go test $(GO_PACKAGES)

# Run all linters (Go + frontend)
lint: lint-go lint-frontend

# Run Go linter. Prefer golangci-lint when installed; otherwise keep the local
# quality gate usable with go vet.
lint-go:
	@echo "Running Go linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found; running go vet instead"; \
		go vet $(GO_PACKAGES); \
	fi

# Run frontend linter (ESLint)
lint-frontend:
	@echo "Running frontend linter..."
	cd frontend && npm run lint

# Format Go code
fmt:
	@echo "Formatting Go code..."
	go fmt ./...

## Maintenance

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf build/bin
	rm -rf frontend/dist
	rm -f aulycmail

# Install frontend dependencies
frontend-deps:
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

# Update frontend dependencies
frontend-update:
	@echo "Updating frontend dependencies..."
	cd frontend && npm update

## Installation (macOS)

# Install aulycmail to /Applications
install: install-darwin
uninstall: uninstall-darwin

# Quit a running installed app before replacing the bundle.
quit-running-darwin:
	@echo "Checking for running aulycmail..."
	@if pgrep -x aulycmail >/dev/null; then \
		echo "Quitting running aulycmail..."; \
		quit_log=$$(mktemp); \
		if osascript -e 'tell application id "com.aulyc.aulycmail" to quit' >"$$quit_log" 2>&1; then \
			rm -f "$$quit_log"; \
		else \
			rm -f "$$quit_log"; \
			echo "Quit request returned a non-zero status; waiting for process exit..."; \
		fi; \
	fi
	@for i in $$(seq 1 20); do \
		if ! pgrep -x aulycmail >/dev/null; then \
			echo "No running aulycmail process found."; \
			exit 0; \
		fi; \
		sleep 0.5; \
	done; \
	echo "aulycmail is still running; quit it and retry installation."; \
	exit 1

# Install aulycmail on macOS
install-darwin: quit-running-darwin build
	@echo "Installing aulycmail.app to /Applications..."
	@if [ -d "/Applications/aulycmail.app" ]; then \
		echo "Removing existing installation..."; \
		rm -rf "/Applications/aulycmail.app"; \
	fi
	cp -R "build/bin/aulycmail.app" "/Applications/"
	@echo "Re-signing installed copy..."
	@codesign_log=$$(mktemp); \
	if codesign --force --deep --sign - "/Applications/aulycmail.app" >"$$codesign_log" 2>&1; then \
		rm -f "$$codesign_log"; \
	else \
		cat "$$codesign_log"; \
		rm -f "$$codesign_log"; \
		exit 1; \
	fi
	@echo ""
	@echo "Installation complete!"
	@echo "aulycmail is now available in /Applications."
	$(MAKE) launch-darwin

# Launch the installed macOS app.
launch-darwin:
	@echo "Launching aulycmail..."
	open "/Applications/aulycmail.app"

# Uninstall aulycmail from macOS
uninstall-darwin:
	@echo "Uninstalling aulycmail from /Applications..."
	rm -rf "/Applications/aulycmail.app"
	@echo "Uninstallation complete!"

## Help

# Show available targets
help:
	@echo "aulycmail Email Client - Build System (macOS-only)"
	@echo ""
	@echo "Build Targets:"
	@echo "  make build        - Build production binary (aulycmail.app)"
	@echo "  make dev          - Run in development mode with hot reload"
	@echo "  make dev-race     - Run in development mode with race detector"
	@echo "  make generate     - Generate Wails TypeScript bindings"
	@echo "  make dmg          - Package the current app bundle as a drag-to-Applications DMG"
	@echo "  make release-dmg  - Build, Developer ID sign, notarize, and staple a release DMG"
	@echo ""
	@echo "Installation:"
	@echo "  make install      - Build, install, and launch aulycmail from /Applications"
	@echo "  make install-dmg  - Install dist/aulycmail.dmg into /Applications"
	@echo "  make install-release-dmg - Build signed DMG, install it, and launch aulycmail"
	@echo "  make uninstall    - Uninstall aulycmail from /Applications"
	@echo ""
	@echo "Code Quality:"
	@echo "  make test          - Run Go tests"
	@echo "  make lint          - Run all linters (Go + frontend)"
	@echo "  make lint-go       - Run Go linter only (requires golangci-lint)"
	@echo "  make lint-frontend - Run frontend linter only (ESLint)"
	@echo "  make fmt           - Format Go code"
	@echo ""
	@echo "Maintenance:"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make frontend-deps   - Install frontend dependencies"
	@echo "  make frontend-update - Update frontend dependencies"
	@echo ""
