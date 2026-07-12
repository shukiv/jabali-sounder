.PHONY: help build run test test-short test-coverage lint fmt vet tidy clean \
	test-ui test-e2e test-all ui-install ui-build migrate-up migrate-down \
	desktop-stage desktop-build server-stage server-build \
	android-stage android-lib android-apk docker-build docker-run

GO         := go
API_PKG    := ./manager-api/...
ALL_PKG    := $(API_PKG)
BIN        := ./bin/jabali-sounder
COVER      := coverage.out
MIN_COV    := 80
DESKTOP_TAGS ?= desktop,production,gtk3

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
	CGO_ENABLED=1 $(GO) build -tags $(DESKTOP_TAGS) -ldflags "$(VLDFLAGS)" -o ./bin/jabali-sounder-desktop ./manager-api/cmd/desktop

# --- Android (Wails v3 mobile). Requires the Android SDK (API 35) + NDK 26.3. ---
NDK_VERSION ?= 26.3.11579264
ANDROID_SDK ?= $(HOME)/Android/Sdk
ANDROID_NDK ?= $(ANDROID_SDK)/ndk/$(NDK_VERSION)
NDK_TC      := $(ANDROID_NDK)/toolchains/llvm/prebuilt/linux-x86_64
JNILIBS     := build/android/app/src/main/jniLibs
MOBILE_PKG  := ./manager-api/cmd/desktop

android-stage: ui-build ## Stage the SPA where the mobile lib embeds it (//go:embed dist)
	rm -rf manager-api/cmd/desktop/dist
	mkdir -p manager-api/cmd/desktop/dist
	cp -R manager-ui/dist/. manager-api/cmd/desktop/dist/

android-lib: android-stage ## Cross-compile libwails.so for arm64 + x86_64 via the NDK
	mkdir -p $(JNILIBS)/arm64-v8a $(JNILIBS)/x86_64
	CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=$(NDK_TC)/bin/aarch64-linux-android21-clang \
	  $(GO) build -buildmode=c-shared -tags "android,debug" -o $(JNILIBS)/arm64-v8a/libwails.so $(MOBILE_PKG)
	CGO_ENABLED=1 GOOS=android GOARCH=amd64 CC=$(NDK_TC)/bin/x86_64-linux-android21-clang \
	  $(GO) build -buildmode=c-shared -tags "android,debug" -o $(JNILIBS)/x86_64/libwails.so $(MOBILE_PKG)
	rm -f $(JNILIBS)/*/libwails.h

android-apk: android-lib ## Build a debug APK -> bin/jabali-sounder.apk
	cd build/android && chmod +x ./gradlew && ANDROID_HOME=$(ANDROID_SDK) ./gradlew assembleDebug --no-daemon
	mkdir -p bin
	cp build/android/app/build/outputs/apk/debug/app-debug.apk bin/jabali-sounder.apk
	@echo "APK: bin/jabali-sounder.apk"

server-stage: ui-build ## Stage the built SPA for the embedded-UI server binary
	rm -rf manager-api/cmd/server/dist
	mkdir -p manager-api/cmd/server/dist
	cp -R manager-ui/dist/. manager-api/cmd/server/dist/

server-build: server-stage ## Build the headless server binary with the SPA embedded (one binary, one port)
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -tags embedui -ldflags "-s -w $(VLDFLAGS)" -o ./bin/jabali-sounder-server ./manager-api/cmd/server

# --- Docker (headless server image) ---
DOCKER_IMAGE ?= jabali-sounder:latest

docker-build: ## Build the server Docker image (multi-stage: SPA + static server)
	docker build \
	  --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(DATE) \
	  -t $(DOCKER_IMAGE) .

docker-run: docker-build ## Run the image on :8484 with a persistent data volume
	docker run --rm -p 8484:8484 -v sounder-data:/data \
	  -e JABALI_SOUNDER_ADMIN_PASSWORD=$${JABALI_SOUNDER_ADMIN_PASSWORD:-changeme} \
	  $(DOCKER_IMAGE)
