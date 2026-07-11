.PHONY: help build run test test-short test-coverage lint fmt vet tidy clean \
	test-ui test-e2e test-all ui-install ui-build migrate-up migrate-down \
	desktop-stage desktop-build server-stage server-build

GO         := go
API_PKG    := ./manager-api/...
ALL_PKG    := $(API_PKG)
BIN        := ./bin/jabali-sounder
COVER      := coverage.out
MIN_COV    := 80
DESKTOP_TAGS ?= desktop,production,webkit2_41

# Build-time version stamping (update mechanism). Override VERSION on release.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%d)
VPKG    := git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/version
VLDFLAGS := -X $(VPKG).Version=$(VERSION) -X $(VPKG).Commit=$(COMMIT) -X $(VPKG).Date=$(DATE)

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Compile the sounder binary
	mkdir -p bin
	$(GO) build -ldflags "$(VLDFLAGS)" -o $(BIN) ./manager-api/cmd/server

run: ## Run the sounder server (dev)
	$(GO) run ./manager-api/cmd/server serve

test: ## Run all Go tests with race detector
	$(GO) test -race -count=1 $(ALL_PKG)

test-short: ## Run only fast unit tests (skip integration)
	$(GO) test -race -count=1 -short $(ALL_PKG)

test-coverage: ## Run tests with coverage (internal packages only)
	$(GO) test -race -count=1 -coverprofile=$(COVER) -covermode=atomic -coverpkg=./manager-api/internal/... ./manager-api/internal/...
	$(GO) tool cover -func=$(COVER) | tail -n 1

migrate-up: ## Run DB migrations up (requires JABALI_SOUNDER_DATABASE_URL)
	$(GO) run ./manager-api/cmd/server migrate up

migrate-down: ## Run DB migrations down (requires JABALI_SOUNDER_DATABASE_URL)
	$(GO) run ./manager-api/cmd/server migrate down

lint: ## Run golangci-lint
	golangci-lint run $(ALL_PKG)

fmt: ## Format all Go code
	$(GO) fmt $(ALL_PKG)

vet: ## Run go vet
	$(GO) vet $(ALL_PKG)

tidy: ## Tidy module deps
	$(GO) mod tidy

clean: ## Remove build artefacts
	rm -rf bin $(COVER)

# ---------- manager-ui (frontend) ----------

UI_DIR := manager-ui

ui-install: ## Install manager-ui npm deps (clean, reproducible)
	cd $(UI_DIR) && npm ci --no-audit --no-fund

ui-build: ## Build the manager-ui SPA (required before E2E — tests run against dist/)
	cd $(UI_DIR) && npm run build

test-ui: ## Run manager-ui unit tests (vitest)
	cd $(UI_DIR) && npx vitest run

test-e2e: ui-build ## Run Playwright E2E suite against the built SPA
	cd $(UI_DIR) && npx playwright test --project=chromium --reporter=list

test-all: test test-ui test-e2e ## Run everything: Go tests + vitest + Playwright

desktop-stage: ui-build ## Stage built SPA assets for the Wails desktop entrypoint
	rm -rf manager-api/cmd/desktop/dist
	mkdir -p manager-api/cmd/desktop/dist
	cp -R manager-ui/dist/. manager-api/cmd/desktop/dist/

desktop-build: desktop-stage ## Build the local standalone desktop binary for the current OS
	mkdir -p bin
	$(GO) build -tags $(DESKTOP_TAGS) -ldflags "$(VLDFLAGS)" -o ./bin/jabali-sounder-desktop ./manager-api/cmd/desktop

server-stage: ui-build ## Stage the built SPA for the embedded-UI server binary
	rm -rf manager-api/cmd/server/dist
	mkdir -p manager-api/cmd/server/dist
	cp -R manager-ui/dist/. manager-api/cmd/server/dist/

server-build: server-stage ## Build the headless server binary with the SPA embedded (one binary, one port)
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -tags embedui -ldflags "-s -w $(VLDFLAGS)" -o ./bin/jabali-sounder-server ./manager-api/cmd/server
