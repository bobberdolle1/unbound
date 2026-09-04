# ============================================================================
# UNBOUND — common tasks
# ============================================================================
# GitHub Actions is currently unavailable on this repository (account billing),
# so `make check` is what verifies a change before it is pushed. It runs the
# same steps .github/workflows/ci.yml would.
#
#   make check      everything CI would run
#   make quick      the same, minus cross-compilation
#   make build      CLI binary for the host platform
#   make gui        desktop app via Wails
# ============================================================================

SHELL := /usr/bin/env bash

# Keep in step with the newest CHANGELOG entry; release builds pass this to
# -ldflags so the binary stops reporting a stale version.
VERSION ?= 0.4.0
LDFLAGS := -s -w -X unbound/engine.Version=$(VERSION)

GOOS_HOST := $(shell go env GOOS 2>/dev/null)
BIN_NAME  := unbound$(if $(filter windows,$(GOOS_HOST)),.exe,)
ifeq ($(GOOS_HOST),darwin)
export CGO_LDFLAGS := -framework UniformTypeIdentifiers
endif

.DEFAULT_GOAL := help
.PHONY: help check quick fmt vet test frontend build gui clean install-hooks

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

check: ## Run every check CI would (gofmt, vet, tests, cross-compile, builds)
	@./scripts/check.sh

quick: ## Same as check, skipping the slow cross-compilation step
	@./scripts/check.sh --quick

sync-logo: ## Synchronize logo master SVG to all derived PNG/ICO/ICNS assets
	@python3 scripts/generate_all_icons.py

fmt: ## Format all Go sources
	@gofmt -w $$(gofmt -l . | grep -v '^frontend/') 2>/dev/null || true
	@echo "formatted"

vet: ## go vet for every supported platform
	@for os in linux darwin windows; do \
		printf '%-8s ' "$$os"; \
		GOOS=$$os go vet ./... && echo OK || exit 1; \
	done

test: ## Run the Go test suite with the race detector
	@go test -race ./...

frontend: ## Build the desktop UI (required before any Go build)
	@cd frontend && npm ci --no-audit --no-fund && npm run build

# main.go embeds frontend/dist, so the Go build cannot run before the UI is
# built. Depending on it here avoids a confusing //go:embed failure.
build: frontend ## Build the desktop GUI binary for the host platform
	@go build -trimpath -tags desktop,production -ldflags="$(LDFLAGS)" -o build/bin/$(BIN_NAME) .
	@echo "built build/bin/$(BIN_NAME) ($(VERSION))"

gui: ## Build the native desktop app via Wails
	@command -v wails >/dev/null || { \
		echo "wails not installed: go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0"; \
		exit 1; }
	@wails build -clean -ldflags "-X unbound/engine.Version=$(VERSION)"
	@if [ "$(GOOS_HOST)" = "darwin" ]; then \
		app="build/bin/unbound.app"; \
		[ -d "$$app" ] || app="build/bin/Unbound.app"; \
		[ -d "$$app" ] || { echo "Wails produced no macOS app bundle"; exit 1; }; \
		xattr -cr "$$app" 2>/dev/null || true; \
		codesign --force --deep -s - "$$app"; \
		codesign --verify --deep --strict "$$app"; \
	fi

clean: ## Remove build artifacts
	@rm -rf build/bin frontend/dist
	@echo "cleaned"
