# Aerion Email Client - Build System (macOS-only slim build)
#
# Usage:
#   make build    - Build production binary (Aerion.app)
#   make dev      - Run in development mode
#   make help     - Show all available targets
#
# OAuth credentials are loaded from .env or .env.local files
# See .env.example for required variables

.PHONY: all build dev dev-race generate clean test lint lint-go lint-frontend \
        fmt frontend-deps frontend-update install uninstall \
        install-darwin uninstall-darwin help

# Load environment variables from .env files.
# .env.local overrides .env. All OAuth credentials live in the root .env —
# extension packages no longer carry their own OAuth client vars.
-include .env
-include .env.local
export

# Go module path
MODULE := github.com/hkdb/aerion

# Build flags for injecting OAuth credentials at compile time.
#
#   GOOGLE_CLIENT_ID/SECRET   — mail's Google-verified client. Also backs
#                               first-party extensions for any scopes their
#                               manifest declares in
#                               first_party_uses_core_for_scopes (today:
#                               contacts.readonly). Surfaced as
#                               "Aerion - Google" in the picker.
#   MICROSOFT_CLIENT_ID       — mail's Azure AD app registration. Also
#                               backs microsoft-contacts and
#                               microsoft-calendar (Microsoft Graph
#                               doesn't gate scopes behind verification).
#                               Surfaced as "Aerion - Microsoft".
#   GOOGLE_TESTING_CLIENT_ID/SECRET — shared un-Google-verified test
#                               project for extensions that need broader
#                               scopes than the mail project carries
#                               (contacts.readwrite, full Calendar).
#                               Single client backs google-contacts AND
#                               google-calendar slots. Surfaced as
#                               "Aerion - Google (Testing)".
LDFLAGS := -X '$(MODULE)/internal/oauth2.GoogleClientID=$(GOOGLE_CLIENT_ID)' \
           -X '$(MODULE)/internal/oauth2.GoogleClientSecret=$(GOOGLE_CLIENT_SECRET)' \
           -X '$(MODULE)/internal/oauth2.MicrosoftClientID=$(MICROSOFT_CLIENT_ID)' \
           -X '$(MODULE)/internal/oauth2.GoogleTestingClientID=$(GOOGLE_TESTING_CLIENT_ID)' \
           -X '$(MODULE)/internal/oauth2.GoogleTestingClientSecret=$(GOOGLE_TESTING_CLIENT_SECRET)'

# Wails build tags
BUILD_TAGS := webkit2_41

# Default target
all: build

## Build Targets

# Build production binary (Aerion.app) and ad-hoc sign it
# (ad-hoc signature is required for macOS notifications to work).
build:
	@echo "Building Aerion..."
	@if [ -z "$(GOOGLE_CLIENT_ID)" ] && [ -z "$(MICROSOFT_CLIENT_ID)" ]; then \
		echo "Warning: No OAuth credentials configured. Gmail/Outlook OAuth will not work."; \
		echo "See .env.example for required variables."; \
	fi
	wails build -ldflags "$(LDFLAGS)" -tags $(BUILD_TAGS)
	@echo "Ad-hoc signing Aerion.app (required for macOS notifications)..."
	codesign --force --deep --sign - build/bin/Aerion.app

# Run in development mode with hot reload
dev:
	@echo "Starting Aerion in development mode..."
	wails dev -ldflags "$(LDFLAGS)" -tags $(BUILD_TAGS)

# Run in development mode with Go's race detector enabled. Builds significantly
# slower and adds ~5-10x runtime overhead, but instruments every memory access
# and prints exactly which line + goroutines collide on any unsynchronized
# shared-memory access. Use this when chasing a suspected data race —
# reproduce the crash and the detector report points right at it.
dev-race:
	@echo "Starting Aerion in development mode with -race..."
	wails dev -ldflags "$(LDFLAGS)" -tags $(BUILD_TAGS) -race

# Generate Wails TypeScript bindings
generate:
	@echo "Generating Wails bindings..."
	wails generate module

## Code Quality

# Run Go tests
test:
	@echo "Running tests..."
	go test ./...

# Run all linters (Go + frontend)
lint: lint-go lint-frontend

# Run Go linter (requires golangci-lint)
lint-go:
	@echo "Running Go linter..."
	golangci-lint run

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
	rm -f aerion

# Install frontend dependencies
frontend-deps:
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

# Update frontend dependencies
frontend-update:
	@echo "Updating frontend dependencies..."
	cd frontend && npm update

## Installation (macOS)

# Install Aerion to /Applications
install: install-darwin
uninstall: uninstall-darwin

# Install Aerion on macOS
install-darwin: build
	@echo "Installing Aerion.app to /Applications..."
	@if [ -d "/Applications/Aerion.app" ]; then \
		echo "Removing existing installation..."; \
		rm -rf "/Applications/Aerion.app"; \
	fi
	cp -R "build/bin/Aerion.app" "/Applications/"
	@echo "Re-signing installed copy..."
	codesign --force --deep --sign - "/Applications/Aerion.app"
	@echo ""
	@echo "Installation complete!"
	@echo "Aerion is now available in /Applications."

# Uninstall Aerion from macOS
uninstall-darwin:
	@echo "Uninstalling Aerion from /Applications..."
	rm -rf "/Applications/Aerion.app"
	@echo "Uninstallation complete!"

## Help

# Show available targets
help:
	@echo "Aerion Email Client - Build System (macOS-only)"
	@echo ""
	@echo "Build Targets:"
	@echo "  make build        - Build production binary (Aerion.app)"
	@echo "  make dev          - Run in development mode with hot reload"
	@echo "  make dev-race     - Run in development mode with race detector"
	@echo "  make generate     - Generate Wails TypeScript bindings"
	@echo ""
	@echo "Installation:"
	@echo "  make install      - Build and install Aerion to /Applications"
	@echo "  make uninstall    - Uninstall Aerion from /Applications"
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
	@echo "Environment Variables:"
	@echo "  GOOGLE_CLIENT_ID     - Google OAuth Client ID"
	@echo "  GOOGLE_CLIENT_SECRET - Google OAuth Client Secret (optional)"
	@echo "  MICROSOFT_CLIENT_ID  - Microsoft OAuth Client ID"
	@echo ""
	@echo "See .env.example for details on obtaining OAuth credentials."
