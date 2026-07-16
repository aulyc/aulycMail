# aulycMail Email Client - Build System (macOS-only slim build)
#
# Usage:
#   make build    - Build production binary (aulycMail.app)
#   make dev      - Run in development mode
#   make help     - Show all available targets

.PHONY: all build build-app dev dev-race generate clean test lint lint-go lint-frontend \
        fmt fmt-check check-go check-frontend check ci release-candidate isolated-release-artifact \
        frontend-deps frontend-update normalize-wails-bindings install uninstall \
        dmg release-dmg test-release-dmg install-dmg install-release-dmg install-test-release-dmg install-darwin \
        quit-running-darwin launch-darwin uninstall-darwin version-check version-test \
        prepare-test-release prepare-formal-release release-preflight release-golangci-lint release-check \
        release-tag release-tag-check release-backup-preflight release-backup-push \
        release-test release-formal help

# Go module path
MODULE := aulyc.local/aulycmail

# version.json is the only version source. The Node tool validates and syncs
# Wails/npm metadata; local runtime versions also identify the Git commit.
VERSION := $(shell node tools/version-bump.mjs get version 2>/dev/null)
BASE_VERSION := $(shell node tools/version-bump.mjs get base 2>/dev/null)
BUILD_NUMBER := $(shell node tools/version-bump.mjs get build 2>/dev/null)
COMMIT_SHA := $(shell git rev-parse HEAD 2>/dev/null || printf unknown)
COMMIT_SHORT := $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
SOURCE_DIRTY := $(shell if [ -n "$$(git status --porcelain --untracked-files=all 2>/dev/null)" ]; then printf true; else printf false; fi)
LOCAL_APP_VERSION := $(shell node tools/version-bump.mjs local-version $(COMMIT_SHORT) $(if $(filter true,$(SOURCE_DIRTY)),--dirty) 2>/dev/null)
APP_VERSION ?= $(LOCAL_APP_VERSION)
BUNDLE_BUILD_NUMBER ?= 0

# aulycMail is a password-auth mail client; only non-secret build metadata is injected.
LDFLAGS = -X $(MODULE)/app.Version=$(APP_VERSION) \
	-X $(MODULE)/app.BuildNumber=$(BUNDLE_BUILD_NUMBER) \
	-X $(MODULE)/app.CommitSHA=$(COMMIT_SHA) \
	-X $(MODULE)/internal/imap.ClientVersion=$(APP_VERSION)

# Wails build tags
BUILD_TAGS := webkit2_41
GO_BUILD_TAGS := desktop,$(BUILD_TAGS),wv2runtime.download,production
APP_BUNDLE := build/bin/aulycMail.app
APP_BINARY := build/bin/aulycMail
DMG_PATH ?= dist/aulycMail-$(VERSION)-build.$(BUNDLE_BUILD_NUMBER).dmg
RELEASE_DMG_PATH := dist/aulycMail-$(VERSION)-build.$(BUILD_NUMBER).dmg
DMG_VOLUME_NAME ?= aulycMail Installer
SIGN_IDENTITY ?=
NOTARY_PROFILE ?=
RELEASE_BUMP ?= auto
RELEASE_CHANNEL ?=
RELEASE_TAG ?= $(VERSION)
RELEASE_OUTPUT_DIR ?= $(abspath dist)
BACKUP_REMOTE ?= backup
BACKUP_BRANCH ?= main
CENTRAL_FORMAL_GIT := /Users/crp/Projects/Codex 开发规范/scripts/formal_release_git.py
CLEAN_FRONTEND_INSTALL ?= 0
RELEASE_METADATA_FILES := version.json wails.json frontend/package.json \
	frontend/package-lock.json CHANGELOG.md

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

# Build production binary (aulycMail.app) and ad-hoc sign it
# (ad-hoc signature is required for macOS notifications to work).
build: version-check build-app

# Internal production build shared by local CI, the pre-tag release candidate,
# and the isolated tagged-source build. Callers provide the runtime identity.
build-app:
	@if [ "$$(uname -m)" != "arm64" ]; then \
		echo 'aulycMail supports Apple Silicon arm64 builds only.'; \
		exit 1; \
	fi
	@echo "Building aulycMail..."
	@node tools/ensure-frontend-dist.mjs
	$(DARWIN_LINK_WARN_ENV) wails generate module
	@tools/normalize_wails_bindings.sh
	@if [ "$(CLEAN_FRONTEND_INSTALL)" = "1" ] || [ ! -d frontend/node_modules ]; then \
		echo "Installing frontend dependencies from package-lock.json..."; \
		cd frontend && npm ci; \
	else \
		echo "Skipping npm install"; \
	fi
	@echo "Compiling frontend..."
	cd frontend && npm run build
	@echo "Compiling application..."
	mkdir -p build/bin
	GOARCH=arm64 $(DARWIN_LINK_WARN_ENV) go build -trimpath -buildvcs=false -tags $(GO_BUILD_TAGS) -ldflags "$(LDFLAGS) -s -w" -o $(APP_BINARY)
	@echo "Packaging macOS app bundle..."
	SEMANTIC_VERSION="$(APP_VERSION)" SHORT_VERSION="$(BASE_VERSION)" \
		BUILD_NUMBER="$(BUNDLE_BUILD_NUMBER)" COMMIT_SHA="$(COMMIT_SHA)" \
		bash tools/package_macos_app.sh
	@echo "Injecting macOS asset-catalog icon (fills the Liquid Glass plate on macOS 26)..."
	bash tools/inject_macos_icon.sh $(APP_BUNDLE) build/appicon.png
	@echo "Ad-hoc signing aulycMail.app (required for macOS notifications)..."
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
	@echo "Starting aulycMail in development mode..."
	$(DARWIN_LINK_WARN_ENV) wails dev -ldflags "$(LDFLAGS)" -tags $(BUILD_TAGS)

# Run in development mode with Go's race detector enabled. Builds significantly
# slower and adds ~5-10x runtime overhead, but instruments every memory access
# and prints exactly which line + goroutines collide on any unsynchronized
# shared-memory access. Use this when chasing a suspected data race —
# reproduce the crash and the detector report points right at it.
dev-race:
	@echo "Starting aulycMail in development mode with -race..."
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

# Package the current local app bundle as a non-release DMG.
dmg:
	@./tools/package_macos_dmg.sh --output "$(DMG_PATH)" \
		--volume-name "$(DMG_VOLUME_NAME)" \
		--source-root "$(CURDIR)" \
		--release-channel local

# Build from an isolated exact tag, Developer ID sign, notarize, and staple.
release-dmg: release-tag-check
	@if [ -z "$(SIGN_IDENTITY)" ]; then \
		echo 'SIGN_IDENTITY is required, e.g. make release-dmg SIGN_IDENTITY="Developer ID Application: nan ma (M9M7M2ARFD)" NOTARY_PROFILE=aulyc-notary'; \
		exit 1; \
	fi
	@if [ -z "$(NOTARY_PROFILE)" ]; then \
		echo 'NOTARY_PROFILE is required, e.g. NOTARY_PROFILE=aulyc-notary'; \
		exit 1; \
	fi
	@node tools/release-worktree.mjs build \
		--repo "$(CURDIR)" \
		--tag "$(VERSION)" \
		--channel formal \
		--output-dir "$(RELEASE_OUTPUT_DIR)" \
		--sign-identity "$(SIGN_IDENTITY)" \
		--notary-profile "$(NOTARY_PROFILE)"

# Build an exact tagged test-release DMG from an isolated detached worktree.
test-release-dmg: release-tag-check
	@node tools/release-worktree.mjs build \
		--repo "$(CURDIR)" \
		--tag "$(VERSION)" \
		--channel test \
		--output-dir "$(RELEASE_OUTPUT_DIR)"

# Runs only inside the temporary detached worktree created above.
isolated-release-artifact:
	@if [ "$(RELEASE_CHANNEL)" != "test" ] && [ "$(RELEASE_CHANNEL)" != "formal" ]; then \
		echo 'RELEASE_CHANNEL must be test or formal.'; \
		exit 1; \
	fi
	@node tools/release-identity.mjs verify-source --root "$(CURDIR)" --tag "$(RELEASE_TAG)"
	@$(MAKE) build-app APP_VERSION="$(VERSION)" BUNDLE_BUILD_NUMBER="$(BUILD_NUMBER)"
	@node tools/release-identity.mjs verify-source --root "$(CURDIR)" --tag "$(RELEASE_TAG)"
	@./tools/package_macos_dmg.sh \
		--source-root "$(CURDIR)" \
		--app "$(APP_BUNDLE)" \
		--output "$(RELEASE_OUTPUT_DIR)/aulycMail-$(VERSION)-build.$(BUILD_NUMBER).dmg" \
		--volume-name "$(DMG_VOLUME_NAME)" \
		--release-channel "$(RELEASE_CHANNEL)" \
		--tag "$(RELEASE_TAG)" \
		$(if $(filter test,$(RELEASE_CHANNEL)),--adhoc-sign) \
		$(if $(filter formal,$(RELEASE_CHANNEL)),--sign "$(SIGN_IDENTITY)" --notary-profile "$(NOTARY_PROFILE)")
	@node tools/release-identity.mjs verify-source --root "$(CURDIR)" --tag "$(RELEASE_TAG)"

# Install the current signed/notarized DMG into /Applications and launch it.
install-dmg: quit-running-darwin
	@./tools/install_macos_dmg.sh --dmg "$(DMG_PATH)"

# Build a signed/notarized release DMG, then install that exact artifact locally.
install-release-dmg: release-dmg quit-running-darwin
	@./tools/install_macos_dmg.sh --dmg "$(RELEASE_DMG_PATH)"

# Build and install the exact tagged, ad-hoc signed test release.
install-test-release-dmg: test-release-dmg quit-running-darwin
	@./tools/install_macos_dmg.sh --dmg "$(RELEASE_DMG_PATH)" --allow-adhoc

# Prepare, commit, validate, tag, build, and install a beta/RC test release.
# Recursive Make calls re-read version.json after the automatic release commit.
release-test:
	@$(MAKE) prepare-test-release RELEASE_BUMP="$(RELEASE_BUMP)"
	@$(MAKE) release-tag
	@$(MAKE) install-test-release-dmg

# Prepare, commit, validate, tag, notarize, build, and install a stable release.
release-formal:
	@if [ -z "$(SIGN_IDENTITY)" ]; then \
		echo 'SIGN_IDENTITY is required for a formal release.'; \
		exit 1; \
	fi
	@if [ -z "$(NOTARY_PROFILE)" ]; then \
		echo 'NOTARY_PROFILE is required for a formal release.'; \
		exit 1; \
	fi
	@$(MAKE) release-backup-preflight
	@$(MAKE) prepare-formal-release RELEASE_BUMP="$(RELEASE_BUMP)"
	@$(MAKE) release-tag
	@$(MAKE) install-release-dmg
	@$(MAKE) release-backup-push

## Code Quality

# Verify that all derived version metadata matches version.json.
version-check:
	@node tools/version-bump.mjs verify

# Test the version parser and synchronization behavior.
version-test:
	@node --test tools/*.test.mjs

# Non-mutating Go formatting gate.
fmt-check:
	@files="$$(find . \
		-path './.git' -prune -o \
		-path './frontend/node_modules' -prune -o \
		-path './frontend/dist' -prune -o \
		-path './build/bin' -prune -o \
		-name '*.go' -print)"; \
	unformatted="$$(test -z "$$files" || gofmt -l $$files)"; \
	if [ -n "$$unformatted" ]; then \
		echo 'Go files require gofmt:'; \
		printf '%s\n' "$$unformatted"; \
		exit 1; \
	fi; \
	echo 'Go formatting verified.'

# Go-only quality gate. golangci-lint degrades explicitly to go vet.
check-go: fmt-check test lint-go

# Frontend-only quality gate.
check-frontend:
	@cd frontend && npm run test:unit
	@cd frontend && npm run check
	@cd frontend && npm run i18n:check
	@cd frontend && npm run lint
	@cd frontend && npm run knip

# Platform-neutral development gate used before review and by CI.
check: version-check version-test check-go check-frontend

# Complete non-release CI entrypoint: all checks followed by a production build.
ci: check
	@$(MAKE) build-app

# Automatic release preparation requires all functional changes to be committed.
# It updates only release metadata, then creates the dedicated release commit.
prepare-test-release:
	@node tools/release-prepare.mjs test --bump "$(RELEASE_BUMP)"
	@release_version=$$(node tools/version-bump.mjs get version); \
	if [ -n "$$(git status --porcelain -- $(RELEASE_METADATA_FILES))" ]; then \
		git add $(RELEASE_METADATA_FILES); \
	fi; \
	if [ "$$(git log -1 --format=%s)" != "chore: release $$release_version" ]; then \
		git commit --allow-empty -m "chore: release $$release_version"; \
	else \
		echo "Release commit already exists for $$release_version."; \
	fi
	@if [ -n "$$(git status --porcelain --untracked-files=all)" ]; then \
		echo 'Release preparation did not leave a clean worktree:'; \
		git status --short; \
		exit 1; \
	fi

prepare-formal-release:
	@node tools/release-prepare.mjs formal --bump "$(RELEASE_BUMP)"
	@release_version=$$(node tools/version-bump.mjs get version); \
	if [ -n "$$(git status --porcelain -- $(RELEASE_METADATA_FILES))" ]; then \
		git add $(RELEASE_METADATA_FILES); \
	fi; \
	if [ "$$(git log -1 --format=%s)" != "chore: release $$release_version" ]; then \
		git commit --allow-empty -m "chore: release $$release_version"; \
	else \
		echo "Release commit already exists for $$release_version."; \
	fi
	@if [ -n "$$(git status --porcelain --untracked-files=all)" ]; then \
		echo 'Release preparation did not leave a clean worktree:'; \
		git status --short; \
		exit 1; \
	fi

# Validate release metadata, release-only commit contents, and a clean worktree.
release-preflight:
	@node tools/release-identity.mjs verify-preflight --root "$(CURDIR)"

# Build the same production configuration used in the isolated final build,
# before a tag is allowed to be created.
release-candidate:
	@$(MAKE) build-app APP_VERSION="$(VERSION)" BUNDLE_BUILD_NUMBER="$(BUILD_NUMBER)" CLEAN_FRONTEND_INSTALL=1
	@bash tools/verify_release_candidate.sh --source-root "$(CURDIR)" --app "$(APP_BUNDLE)"

# Release lint is fail-closed and intentionally pins the version inline so an
# ordinary Make command-line variable cannot weaken the release requirement.
release-golangci-lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo 'golangci-lint v2.12.2 is required for release-check; no fallback is allowed.'; \
		exit 1; \
	fi
	@version_json=$$(golangci-lint version --json) || { \
		echo 'Unable to read golangci-lint version as JSON.'; \
		exit 1; \
	}; \
	actual_version=$$(printf '%s' "$$version_json" | node -e 'const fs = require("fs"); try { const value = JSON.parse(fs.readFileSync(0, "utf8")); if (typeof value.version !== "string") process.exit(2); process.stdout.write(value.version); } catch { process.exit(2); }') || { \
		echo 'golangci-lint version --json did not contain a valid version field.'; \
		exit 1; \
	}; \
	if [ "$$actual_version" != "2.12.2" ]; then \
		echo "golangci-lint v2.12.2 is required for release-check; found $$actual_version."; \
		exit 1; \
	fi; \
	echo "Verified release golangci-lint v$$actual_version at $$(command -v golangci-lint)."
	@golangci-lint config verify --config .golangci.yml
	@golangci-lint run

# Run the fail-closed release lint before the shared gates, then build and
# verify the pre-tag candidate. Recursive calls preserve this ordering under -j.
release-check: release-preflight
	@$(MAKE) release-golangci-lint
	@$(MAKE) check
	@$(MAKE) release-candidate
	@if [ -n "$$(git status --porcelain --untracked-files=all)" ]; then \
		echo 'Quality checks changed tracked files; review and commit them before tagging:'; \
		git status --short; \
		exit 1; \
	fi

# Public artifacts require an exact annotated tag at the release commit.
release-tag-check:
	@node tools/release-identity.mjs verify-tag --root "$(CURDIR)" --tag "$(VERSION)"

# Fail before release metadata changes if the private backup remote cannot be
# reached or the formal release is not running from the configured branch.
release-backup-preflight:
	@python3 "$(CENTRAL_FORMAL_GIT)" preflight --path "$(CURDIR)"

# Publish the verified formal release commit and annotated tag together, then
# independently confirm that both remote refs resolve to the local release.
release-backup-push: release-tag-check
	@python3 "$(CENTRAL_FORMAL_GIT)" push \
		--path "$(CURDIR)" \
		--tag "$(VERSION)" \
		--provenance "$(RELEASE_OUTPUT_DIR)/aulycMail-$(VERSION)-build.$(BUILD_NUMBER).manifest.json"

# Create the annotated release tag once; existing tags are never overwritten.
release-tag: release-check
	@if git show-ref --verify --quiet "refs/tags/$(VERSION)"; then \
		if [ "$$(git cat-file -t "$(VERSION)")" != tag ] || \
		   [ "$$(git rev-list -n 1 "$(VERSION)")" != "$$(git rev-parse HEAD)" ]; then \
			echo 'Existing tag $(VERSION) is not the expected annotated tag at HEAD.'; \
			exit 1; \
		fi; \
		echo 'Verified existing immutable release tag $(VERSION).'; \
	else \
		git tag -a "$(VERSION)" -m "aulycMail $(VERSION)"; \
		echo 'Created immutable release tag $(VERSION).'; \
	fi

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
		echo "Using golangci-lint at $$(command -v golangci-lint)"; \
		golangci-lint run; \
	else \
		echo "golangci-lint not found; running go vet instead (development fallback only)"; \
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
	rm -f aulycMail aulycmail

# Install frontend dependencies
frontend-deps:
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

# Update frontend dependencies
frontend-update:
	@echo "Updating frontend dependencies..."
	cd frontend && npm update

## Installation (macOS)

# Install aulycMail to /Applications
install: install-darwin
uninstall: uninstall-darwin

# Quit a running installed app before replacing the bundle.
quit-running-darwin:
	@echo "Checking for running aulycMail or legacy aulycmail..."
	@if pgrep -x aulycMail >/dev/null || pgrep -x aulycmail >/dev/null; then \
		echo "Quitting running aulycMail..."; \
		quit_log=$$(mktemp); \
		if osascript -e 'tell application id "com.aulyc.aulycmail" to quit' >"$$quit_log" 2>&1; then \
			rm -f "$$quit_log"; \
		else \
			rm -f "$$quit_log"; \
			echo "Quit request returned a non-zero status; waiting for process exit..."; \
		fi; \
	fi
	@for i in $$(seq 1 20); do \
		if ! pgrep -x aulycMail >/dev/null && ! pgrep -x aulycmail >/dev/null; then \
			echo "No running aulycMail process found."; \
			exit 0; \
		fi; \
		sleep 0.5; \
	done; \
	echo "aulycMail is still running; quit it and retry installation."; \
	exit 1

# Install aulycMail on macOS
install-darwin: quit-running-darwin build
	@echo "Installing aulycMail.app to /Applications..."
	@for app in "/Applications/aulycMail.app" "/Applications/aulycmail.app"; do \
		if [ -d "$$app" ]; then \
			echo "Removing existing installation at $$app..."; \
			rm -rf "$$app"; \
		fi; \
	done
	cp -R "$(APP_BUNDLE)" "/Applications/aulycMail.app"
	@echo "Re-signing installed copy..."
	@codesign_log=$$(mktemp); \
	if codesign --force --deep --sign - "/Applications/aulycMail.app" >"$$codesign_log" 2>&1; then \
		rm -f "$$codesign_log"; \
	else \
		cat "$$codesign_log"; \
		rm -f "$$codesign_log"; \
		exit 1; \
	fi
	@echo ""
	@echo "Installation complete!"
	@echo "aulycMail is now available in /Applications."
	$(MAKE) launch-darwin

# Launch the installed macOS app.
launch-darwin:
	@echo "Launching aulycMail..."
	open "/Applications/aulycMail.app"

# Uninstall aulycMail from macOS
uninstall-darwin:
	@echo "Uninstalling aulycMail from /Applications..."
	rm -rf "/Applications/aulycMail.app" "/Applications/aulycmail.app"
	@echo "Uninstallation complete!"

## Help

# Show available targets
help:
	@echo "aulycMail Email Client - Build System (macOS-only)"
	@echo ""
	@echo "Build Targets:"
	@echo "  make build        - Build production binary (aulycMail.app)"
	@echo "  make dev          - Run in development mode with hot reload"
	@echo "  make dev-race     - Run in development mode with race detector"
	@echo "  make generate     - Generate Wails TypeScript bindings"
	@echo "  make dmg          - Package the current app bundle as a non-release local DMG"
	@echo "  make release-test - Auto-version, commit, tag, ad-hoc sign, and install a test release"
	@echo "  make release-formal - Auto-version, commit, tag, notarize, and install a stable release"
	@echo "  make release-check - Require golangci-lint v2.12.2, run gates, and build the candidate"
	@echo "  make release-tag  - Create the annotated no-v tag after release checks pass"
	@echo "  make release-backup-preflight - Verify the private backup remote before formal release"
	@echo "  make release-backup-push - Atomically push and verify the formal release commit/tag"
	@echo "  make test-release-dmg - Build an ad-hoc test DMG from an isolated exact tag"
	@echo "  make release-dmg  - Build a formal notarized DMG from an isolated exact tag"
	@echo ""
	@echo "Installation:"
	@echo "  make install      - Build, install, and launch aulycMail from /Applications"
	@echo "  make install-dmg  - Verify and install DMG_PATH into /Applications"
	@echo "  make install-test-release-dmg - Build and install the exact tagged test DMG"
	@echo "  make install-release-dmg - Build signed DMG, install it, and launch aulycMail"
	@echo "  make uninstall    - Uninstall aulycMail from /Applications"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt-check     - Verify Go formatting without modifying files"
	@echo "  make check-go      - Run Go formatting, tests, and lint/vet"
	@echo "  make check-frontend - Run frontend unit/type/i18n/lint/knip checks"
	@echo "  make check         - Run version, Go, frontend, and release-tool tests"
	@echo "  make ci            - Run make check, then a full production build"
	@echo "  make test          - Run Go tests"
	@echo "  make lint          - Run all linters (Go + frontend)"
	@echo "  make lint-go       - Run golangci-lint, or explicitly fall back to go vet"
	@echo "  make release-golangci-lint - Verify v2.12.2/config and run fail-closed release lint"
	@echo "  make lint-frontend - Run frontend linter only (ESLint)"
	@echo "  make version-check - Verify files derived from version.json"
	@echo "  make version-test  - Test version parsing and synchronization"
	@echo "  make fmt           - Format Go code"
	@echo ""
	@echo "Maintenance:"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make frontend-deps   - Install frontend dependencies"
	@echo "  make frontend-update - Update frontend dependencies"
	@echo ""
