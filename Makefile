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
#   make site       build the website
#   make deploy-site  publish the website to gh-pages
# ============================================================================

SHELL := /usr/bin/env bash

# Keep in step with the newest CHANGELOG entry; release builds pass this to
# -ldflags so the binary stops reporting a stale version.
VERSION ?= 2.5.0
LDFLAGS := -s -w -X unbound/engine.Version=$(VERSION)

GOOS_HOST := $(shell go env GOOS 2>/dev/null)
BIN_NAME  := unbound$(if $(filter windows,$(GOOS_HOST)),.exe,)

.DEFAULT_GOAL := help
.PHONY: help check quick fmt vet test frontend site build gui deploy-site clean install-hooks

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

check: ## Run every check CI would (gofmt, vet, tests, cross-compile, builds)
	@./scripts/check.sh

quick: ## Same as check, skipping the slow cross-compilation step
	@./scripts/check.sh --quick

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
build: frontend ## Build the CLI binary for the host platform
	@go build -trimpath -ldflags="$(LDFLAGS)" -o build/bin/$(BIN_NAME) .
	@echo "built build/bin/$(BIN_NAME) ($(VERSION))"

gui: ## Build the desktop app via Wails
	@command -v wails >/dev/null || { \
		echo "wails not installed: go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0"; \
		exit 1; }
	@wails build -clean -ldflags "-X unbound/engine.Version=$(VERSION)"

site: ## Build the Astro website
	@cd website && npm ci --no-audit --no-fund && npm run build

deploy-site: site ## Publish the website to the gh-pages branch
	@cd website && npx gh-pages -d dist -m "Deploy website $(VERSION)"
	@echo "published to https://bobberdolle1.github.io/unbound/"

install-hooks: ## Run the checks automatically before every push
	@ln -sf ../../scripts/check.sh .git/hooks/pre-push
	@echo "pre-push hook installed (runs ./scripts/check.sh)"

clean: ## Remove build artifacts
	@rm -rf build/bin frontend/dist/assets website/dist
	@echo "cleaned"
