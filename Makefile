# aulycmail Email Client - Build System (macOS-only slim build)
#
# Usage:
#   make build    - Build production binary (aulycmail.app)
#   make dev      - Run in development mode
#   make help     - Show all available targets

.PHONY: all build dev dev-race generate clean test lint lint-go lint-frontend \
        fmt frontend-deps frontend-update install uninstall \
        install-darwin uninstall-darwin help

# Go module path
MODULE := github.com/aulyc/aulycmail

# No OAuth credentials are injected — aulycmail is a password-auth mail client.
LDFLAGS :=

# Wails build tags
BUILD_TAGS := webkit2_41

# Default target
all: build

## Build Targets

# Build production binary (aulycmail.app) and ad-hoc sign it
# (ad-hoc signature is required for macOS notifications to work).
build:
	@echo "Building aulycmail..."
	wails build -ldflags "$(LDFLAGS) -s -w" -tags $(BUILD_TAGS)
	@echo "Injecting macOS asset-catalog icon (fills the Liquid Glass plate on macOS 26)..."
	bash tools/inject_macos_icon.sh build/bin/aulycmail.app build/appicon.png
	@echo "Ad-hoc signing aulycmail.app (required for macOS notifications)..."
	codesign --force --deep --sign - build/bin/aulycmail.app

# Run in development mode with hot reload
dev:
	@echo "Starting aulycmail in development mode..."
	wails dev -ldflags "$(LDFLAGS)" -tags $(BUILD_TAGS)

# Run in development mode with Go's race detector enabled. Builds significantly
# slower and adds ~5-10x runtime overhead, but instruments every memory access
# and prints exactly which line + goroutines collide on any unsynchronized
# shared-memory access. Use this when chasing a suspected data race —
# reproduce the crash and the detector report points right at it.
dev-race:
	@echo "Starting aulycmail in development mode with -race..."
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

# Install aulycmail on macOS
install-darwin: build
	@echo "Installing aulycmail.app to /Applications..."
	@if [ -d "/Applications/aulycmail.app" ]; then \
		echo "Removing existing installation..."; \
		rm -rf "/Applications/aulycmail.app"; \
	fi
	cp -R "build/bin/aulycmail.app" "/Applications/"
	@echo "Re-signing installed copy..."
	codesign --force --deep --sign - "/Applications/aulycmail.app"
	@echo ""
	@echo "Installation complete!"
	@echo "aulycmail is now available in /Applications."

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
	@echo ""
	@echo "Installation:"
	@echo "  make install      - Build and install aulycmail to /Applications"
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
	@echo "Environment Variables:"
	@echo "  GOOGLE_CLIENT_ID     - Google OAuth Client ID"
	@echo "  GOOGLE_CLIENT_SECRET - Google OAuth Client Secret (optional)"
	@echo "  MICROSOFT_CLIENT_ID  - Microsoft OAuth Client ID"
	@echo ""
	@echo "See .env.example for details on obtaining OAuth credentials."
