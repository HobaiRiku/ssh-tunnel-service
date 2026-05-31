# ssh-tunnel-service — top-level Makefile
#
# Build model:
#   make dev / run     # No frontend build; binary serves "UI not built" placeholder.
#                      # Run `make ui-dev` in a separate terminal for the Vite dev server.
#   make ui-build      # Build the SPA into internal/web/static/
#   make build         # Build release binary (runs ui-build, adds -tags embedui).
#   make all           # Equivalent: ui-build + build

APP_NAME    := ssh-tunnel
PKG_VERSION := ssh-tunnel-service/internal/version

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
-X $(PKG_VERSION).Version=$(VERSION) \
-X $(PKG_VERSION).Commit=$(COMMIT) \
-X $(PKG_VERSION).BuildDate=$(DATE)

BUILD_DIR  := build/bin
DIST_DIR   := build/dist
UI_DIR     := ui
EMBED_DIR  := internal/web/static
LOCAL_HOME := $(CURDIR)/.ssh-tunnel-service-home
PNPM       ?= pnpm

DIST_TARGETS := \
darwin/amd64 \
darwin/arm64 \
linux/amd64 \
linux/arm64 \
linux/arm \
windows/amd64 \
windows/arm64

.PHONY: all build run dev tidy fmt vet test \
ui-install ui-dev ui-build ui-clean \
dist dist-clean dist-list clean \
release release-snapshot release-check print-version

all: ui-build build

# ---- Go --------------------------------------------------------------------

build: ui-build
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -tags embedui -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) .

run:
	SSH_TUNNEL_HOME=$(LOCAL_HOME) go run -ldflags "$(LDFLAGS)" . run

dev: run

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	SSH_TUNNEL_HOME=$(LOCAL_HOME) go test ./...

# ---- UI --------------------------------------------------------------------

ui-install:
	cd $(UI_DIR) && $(PNPM) install

ui-dev:
	cd $(UI_DIR) && $(PNPM) run dev

ui-build: ui-install
	cd $(UI_DIR) && $(PNPM) run build

ui-clean:
	rm -rf $(UI_DIR)/node_modules $(UI_DIR)/dist $(EMBED_DIR)

# ---- Cross-platform distribution -------------------------------------------

dist: dist-clean ui-build
	@mkdir -p $(DIST_DIR)
	@rm -f $(DIST_DIR)/SHA256SUMS
	@for target in $(DIST_TARGETS); do \
os=$${target%/*}; arch=$${target#*/}; \
ext=""; \
if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
out="$(DIST_DIR)/$(APP_NAME)_$(VERSION)_$${os}_$${arch}$${ext}"; \
echo ">> building $$out"; \
CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
go build -trimpath -tags embedui \
-ldflags "$(LDFLAGS)" \
-o "$$out" . || exit 1; \
done
	@echo ">> writing $(DIST_DIR)/SHA256SUMS"
	@cd $(DIST_DIR) && \
( command -v sha256sum >/dev/null && sha256sum $(APP_NAME)_$(VERSION)_* > SHA256SUMS ) || \
shasum -a 256 $(APP_NAME)_$(VERSION)_* > SHA256SUMS
	@ls -lh $(DIST_DIR)

dist-clean:
	rm -rf $(DIST_DIR)

dist-list:
	@echo "DIST_TARGETS:"
	@for t in $(DIST_TARGETS); do echo "  $$t"; done

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR)

print-version:
	@echo "version=$(VERSION) commit=$(COMMIT) date=$(DATE)"
