BUILD_DIR = build
RELEASE_DIR ?= release
CLI_BINARY = gpusched
CLI_MOCK_BINARY = gpusched_mock
DESKTOP_BINARY = gpusched-desktop
DESKTOP_APP_BUNDLE = gpusched.app
DESKTOP_ICON_SOURCE = frontend/static/gpu2.png
DESKTOP_ICON = cmd/desktop/build/appicon.png
LINUX_DESKTOP_ASSETS = cmd/desktop/build/linux
HOST_OS = $(shell go env GOOS)

# Version stamped into the desktop binary for the in-app self-updater. Defaults
# to "dev" (which always reports an update available, for local testing); the
# release flow passes the real tag. Injected with -ldflags -X.
DESKTOP_VERSION ?= dev
DESKTOP_VERSIONPKG = github.com/john221wick/gpuSchedularSN/internal/desktop

# Native desktop builds: only darwin can be built natively (and only on a Mac).
# Linux desktop is always built via Docker (see desktop-linux-release) because
# Wails on Linux needs CGO + webkit2gtk and cannot be cross-compiled from macOS.
ifeq ($(HOST_OS),darwin)
DEFAULT_DESKTOP_PLATFORMS = darwin/amd64 darwin/arm64
else
DEFAULT_DESKTOP_PLATFORMS =
endif

DESKTOP_PLATFORMS ?= $(DEFAULT_DESKTOP_PLATFORMS)
LINUX_DESKTOP_PLATFORMS ?= linux/amd64 linux/arm64
LINUX_DESKTOP_BUILDER_IMAGE = gpusched-linux-builder
CLI_RELEASE_ASSETS = $(RELEASE_DIR)/gpusched-linux-amd64 $(RELEASE_DIR)/gpusched-linux-arm64 $(RELEASE_DIR)/gpusched-darwin-amd64 $(RELEASE_DIR)/gpusched-darwin-arm64
DESKTOP_RELEASE_ASSETS = $(foreach platform,$(DESKTOP_PLATFORMS),$(RELEASE_DIR)/gpusched-desktop-$(subst /,-,$(platform))$(if $(filter darwin/%,$(platform)),.tar.gz,))
# Each Linux arch ships two binaries: the default (webkit2gtk-4.0) and a
# -webkit41 variant for newer distros that dropped 4.0 (Ubuntu 24.04+, etc).
LINUX_DESKTOP_RELEASE_ASSETS = $(foreach platform,$(LINUX_DESKTOP_PLATFORMS),$(RELEASE_DIR)/gpusched-desktop-$(subst /,-,$(platform)).tar.gz $(RELEASE_DIR)/gpusched-desktop-$(subst /,-,$(platform))-webkit41.tar.gz)
RELEASE_ASSETS = $(DESKTOP_RELEASE_ASSETS) $(LINUX_DESKTOP_RELEASE_ASSETS) $(CLI_RELEASE_ASSETS)

.PHONY: cli cli-mock desktop desktop-release desktop-linux-release cli-release build clean test frontend sync-desktop-frontend sync-desktop-icon

# --- CLI (existing) ---

cli: $(BUILD_DIR)/$(CLI_BINARY)

cli-mock: $(BUILD_DIR)/$(CLI_MOCK_BINARY)

$(BUILD_DIR)/$(CLI_BINARY):
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(CLI_BINARY) ./cmd/

$(BUILD_DIR)/$(CLI_MOCK_BINARY):
	@mkdir -p $(BUILD_DIR)
	go build -tags mock -o $(BUILD_DIR)/$(CLI_MOCK_BINARY) ./cmd/

# --- Desktop (Wails) ---

frontend:
	cd frontend && CI=true pnpm install && pnpm build

sync-desktop-icon:
	@test -f "$(DESKTOP_ICON_SOURCE)" || { echo "Missing desktop icon: $(DESKTOP_ICON_SOURCE)"; exit 1; }
	@mkdir -p cmd/desktop/build
	@if command -v sips >/dev/null 2>&1; then \
		sips -z 1024 1024 "$(DESKTOP_ICON_SOURCE)" --out "$(DESKTOP_ICON)" >/dev/null; \
	else \
		cp "$(DESKTOP_ICON_SOURCE)" "$(DESKTOP_ICON)"; \
	fi

sync-desktop-frontend: frontend
	rm -rf cmd/desktop/frontend/dist
	mkdir -p cmd/desktop/frontend
	cp -R frontend/dist cmd/desktop/frontend/dist

desktop: sync-desktop-icon sync-desktop-frontend
	cd cmd/desktop && wails build -s -o ../../$(BUILD_DIR)/$(DESKTOP_BINARY)

desktop-dev: sync-desktop-icon
	cd cmd/desktop && wails dev

desktop-mock: sync-desktop-icon
	cd cmd/desktop && wails dev -tags mock

# --- Release Artifacts ---

build: desktop-release desktop-linux-release cli-release

desktop-release: sync-desktop-icon sync-desktop-frontend
	@if [ -z "$(DESKTOP_PLATFORMS)" ]; then \
		echo "No native desktop platforms for host '$(HOST_OS)'; skipping (Linux handled by desktop-linux-release)."; \
		exit 0; \
	fi
	@mkdir -p $(RELEASE_DIR)
	@set -e; \
	for platform in $(DESKTOP_PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		echo "Building desktop app for $$os/$$arch..."; \
		( cd cmd/desktop && wails build -clean -s -ldflags "-s -w -X $(DESKTOP_VERSIONPKG).Version=$(DESKTOP_VERSION)" -platform $$os/$$arch -o $(DESKTOP_BINARY) ); \
		if [ "$$os" = "darwin" ]; then \
			test -d "cmd/desktop/build/bin/$(DESKTOP_APP_BUNDLE)" || { echo "Missing Wails app bundle: cmd/desktop/build/bin/$(DESKTOP_APP_BUNDLE)"; exit 1; }; \
			tar -czf "$(RELEASE_DIR)/gpusched-desktop-$$os-$$arch.tar.gz" -C cmd/desktop/build/bin $(DESKTOP_APP_BUNDLE); \
		else \
			test -f "cmd/desktop/build/bin/$(DESKTOP_BINARY)" || { echo "Missing Wails desktop binary: cmd/desktop/build/bin/$(DESKTOP_BINARY)"; exit 1; }; \
			cp "cmd/desktop/build/bin/$(DESKTOP_BINARY)" "$(RELEASE_DIR)/gpusched-desktop-$$os-$$arch"; \
			chmod +x "$(RELEASE_DIR)/gpusched-desktop-$$os-$$arch"; \
		fi; \
	done

# Linux desktop builds run inside Docker: Wails on Linux requires CGO +
# webkit2gtk, which cannot be cross-compiled from macOS. The builder image
# (docker/linux/Dockerfile) bakes the toolchain so this is fast and repeatable.
# Two variants are built per arch so every common distro is covered:
#   default        -> webkit2gtk-4.0 (Ubuntu 20.04/22.04, Debian 11/12)
#   -webkit41 tag  -> webkit2gtk-4.1 (Ubuntu 24.04+, Debian 13, Fedora 40+, Arch)
# install.sh detects the host's webkit and downloads the matching binary.
desktop-linux-release: sync-desktop-icon sync-desktop-frontend
	@command -v docker >/dev/null 2>&1 || { echo "docker is required to build the Linux desktop app"; exit 1; }
	@mkdir -p $(RELEASE_DIR)
	@set -e; \
	for platform in $(LINUX_DESKTOP_PLATFORMS); do \
		arch=$${platform#*/}; \
		echo "Building Linux desktop app for $$platform via Docker..."; \
		docker build --platform $$platform -t $(LINUX_DESKTOP_BUILDER_IMAGE):$$arch docker/linux; \
		for variant in 40 41; do \
			if [ "$$variant" = "41" ]; then tags="-tags webkit2_41"; suffix="-webkit41"; else tags=""; suffix=""; fi; \
			echo "  -> webkit$$variant ($$platform)"; \
			ldflags="-s -w -X $(DESKTOP_VERSIONPKG).Version=$(DESKTOP_VERSION) -X $(DESKTOP_VERSIONPKG).WebkitSuffix=$$suffix"; \
			docker run --rm --platform $$platform \
				-v "$(CURDIR)":/app \
				-v gpusched-gomod:/go/pkg/mod \
				-v gpusched-gobuild:/root/.cache/go-build \
				-w /app/cmd/desktop \
				$(LINUX_DESKTOP_BUILDER_IMAGE):$$arch \
				wails build -s -clean $$tags -ldflags "$$ldflags" -platform $$platform -o $(DESKTOP_BINARY); \
			test -f "cmd/desktop/build/bin/$(DESKTOP_BINARY)" || { echo "Missing Linux desktop binary for $$platform webkit$$variant"; exit 1; }; \
			pkg="$$(mktemp -d)"; \
			mkdir -p "$$pkg/bin" "$$pkg/share/applications" "$$pkg/share/icons"; \
			cp "cmd/desktop/build/bin/$(DESKTOP_BINARY)" "$$pkg/bin/$(DESKTOP_BINARY)"; chmod +x "$$pkg/bin/$(DESKTOP_BINARY)"; \
			cp "$(LINUX_DESKTOP_ASSETS)/$(DESKTOP_BINARY).desktop" "$$pkg/share/applications/"; \
			cp -R "$(LINUX_DESKTOP_ASSETS)/hicolor" "$$pkg/share/icons/"; \
			tar --no-xattrs -czf "$(RELEASE_DIR)/gpusched-desktop-linux-$$arch$$suffix.tar.gz" -C "$$pkg" .; \
			rm -rf "$$pkg"; \
		done; \
	done

cli-release:
	@mkdir -p $(RELEASE_DIR)
	@echo "Building CLI binaries..."
	@GOOS=linux GOARCH=amd64 go build -tags mock -o $(RELEASE_DIR)/gpusched-linux-amd64 ./cmd/
	@GOOS=linux GOARCH=arm64 go build -tags mock -o $(RELEASE_DIR)/gpusched-linux-arm64 ./cmd/
	@GOOS=darwin GOARCH=amd64 go build -tags mock -o $(RELEASE_DIR)/gpusched-darwin-amd64 ./cmd/
	@GOOS=darwin GOARCH=arm64 go build -tags mock -o $(RELEASE_DIR)/gpusched-darwin-arm64 ./cmd/

# --- Test ---

test:
	go test $(shell go list ./... | grep -v cmd/desktop)

test-mock:
	go test -tags mock ./...

# --- Clean ---

clean:
	rm -rf $(BUILD_DIR)
	rm -rf frontend/.svelte-kit frontend/build
	rm -rf $(RELEASE_DIR)

# --- Version Management ---

VERSION_FILE = internal/cli/cli.go
CURRENT_VERSION = $(shell grep -oE 'const Version = "[0-9]+\.[0-9]+\.[0-9]+"' $(VERSION_FILE) | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')

version:
	@echo "Current version: $(CURRENT_VERSION)"

# Patch version bump (0.0.1 → 0.0.2)
publish:
	@echo "Publishing patch version..."
	@MAJOR=$$(echo $(CURRENT_VERSION) | cut -d. -f1); \
	MINOR=$$(echo $(CURRENT_VERSION) | cut -d. -f2); \
	PATCH=$$(echo $(CURRENT_VERSION) | cut -d. -f3); \
	NEW_PATCH=$$((PATCH + 1)); \
	NEW_VERSION="$$MAJOR.$$MINOR.$$NEW_PATCH"; \
	echo "Bumping version: $(CURRENT_VERSION) → $$NEW_VERSION"; \
	sed -i.bak 's/const Version = "$(CURRENT_VERSION)"/const Version = "'$$NEW_VERSION'"/' $(VERSION_FILE) && rm $(VERSION_FILE).bak; \
	$(MAKE) _build-and-release NEW_VERSION=$$NEW_VERSION

# Minor version bump (0.1.0 → 0.2.0)
bump:
	@echo "Bumping minor version..."
	@MAJOR=$$(echo $(CURRENT_VERSION) | cut -d. -f1); \
	MINOR=$$(echo $(CURRENT_VERSION) | cut -d. -f2); \
	NEW_MINOR=$$((MINOR + 1)); \
	NEW_VERSION="$$MAJOR.$$NEW_MINOR.0"; \
	echo "Bumping version: $(CURRENT_VERSION) → $$NEW_VERSION"; \
	sed -i.bak 's/const Version = "$(CURRENT_VERSION)"/const Version = "'$$NEW_VERSION'"/' $(VERSION_FILE) && rm $(VERSION_FILE).bak; \
	$(MAKE) _build-and-release NEW_VERSION=$$NEW_VERSION

# Major version bump (1.0.0 → 2.0.0)
release:
	@echo "Releasing major version..."
	@MAJOR=$$(echo $(CURRENT_VERSION) | cut -d. -f1); \
	NEW_MAJOR=$$((MAJOR + 1)); \
	NEW_VERSION="$$NEW_MAJOR.0.0"; \
	echo "Bumping version: $(CURRENT_VERSION) → $$NEW_VERSION"; \
	sed -i.bak 's/const Version = "$(CURRENT_VERSION)"/const Version = "'$$NEW_VERSION'"/' $(VERSION_FILE) && rm $(VERSION_FILE).bak; \
	$(MAKE) _build-and-release NEW_VERSION=$$NEW_VERSION

_build-and-release:
	@echo "Building release artifacts for v$(NEW_VERSION)..."
	@rm -rf $(RELEASE_DIR) && mkdir -p $(RELEASE_DIR)
	@$(MAKE) desktop-release DESKTOP_VERSION=v$(NEW_VERSION)
	@$(MAKE) desktop-linux-release DESKTOP_VERSION=v$(NEW_VERSION)
	@$(MAKE) cli-release
	@echo "Committing version bump..."
	@git add $(VERSION_FILE)
	@git commit -m "Bump version to v$(NEW_VERSION)"
	@git tag v$(NEW_VERSION)
	@git push && git push --tags
	@echo "Creating GitHub release..."
	@gh release create v$(NEW_VERSION) \
		--title "v$(NEW_VERSION)" \
		--notes "Release v$(NEW_VERSION)" \
		$(RELEASE_ASSETS)
	@echo "✓ Released v$(NEW_VERSION)"
