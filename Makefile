BUILD_DIR = build
RELEASE_DIR = release
CLI_BINARY = gpusched
CLI_MOCK_BINARY = gpusched_mock
DESKTOP_BINARY = gpusched-desktop
DESKTOP_APP_BUNDLE = gpusched.app
HOST_OS = $(shell go env GOOS)

ifeq ($(HOST_OS),darwin)
DEFAULT_DESKTOP_PLATFORMS = darwin/amd64 darwin/arm64
else ifeq ($(HOST_OS),linux)
DEFAULT_DESKTOP_PLATFORMS = linux/amd64 linux/arm64
else
DEFAULT_DESKTOP_PLATFORMS =
endif

DESKTOP_PLATFORMS ?= $(DEFAULT_DESKTOP_PLATFORMS)
CLI_RELEASE_ASSETS = $(RELEASE_DIR)/gpusched-linux-amd64 $(RELEASE_DIR)/gpusched-linux-arm64 $(RELEASE_DIR)/gpusched-darwin-amd64 $(RELEASE_DIR)/gpusched-darwin-arm64
DESKTOP_RELEASE_ASSETS = $(foreach platform,$(DESKTOP_PLATFORMS),$(RELEASE_DIR)/gpusched-desktop-$(subst /,-,$(platform))$(if $(filter darwin/%,$(platform)),.tar.gz,))
RELEASE_ASSETS = $(DESKTOP_RELEASE_ASSETS) $(CLI_RELEASE_ASSETS)

.PHONY: cli cli-mock desktop desktop-release cli-release clean test frontend

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
	cd frontend && pnpm install && pnpm build

desktop: frontend
	cd cmd/desktop && wails build -o ../../$(BUILD_DIR)/$(DESKTOP_BINARY)

desktop-dev:
	cd cmd/desktop && wails dev

desktop-mock:
	cd cmd/desktop && wails dev -tags mock

# --- Release Artifacts ---

desktop-release:
	@if [ -z "$(DESKTOP_PLATFORMS)" ]; then \
		echo "Desktop release builds are supported from macOS or Linux hosts only."; \
		exit 1; \
	fi
	@mkdir -p $(RELEASE_DIR)
	@set -e; \
	for platform in $(DESKTOP_PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		echo "Building desktop app for $$os/$$arch..."; \
		( cd cmd/desktop && wails build -clean -platform $$os/$$arch -o $(DESKTOP_BINARY) ); \
		if [ "$$os" = "darwin" ]; then \
			test -d "cmd/desktop/build/bin/$(DESKTOP_APP_BUNDLE)" || { echo "Missing Wails app bundle: cmd/desktop/build/bin/$(DESKTOP_APP_BUNDLE)"; exit 1; }; \
			tar -czf "$(RELEASE_DIR)/gpusched-desktop-$$os-$$arch.tar.gz" -C cmd/desktop/build/bin $(DESKTOP_APP_BUNDLE); \
		else \
			test -f "cmd/desktop/build/bin/$(DESKTOP_BINARY)" || { echo "Missing Wails desktop binary: cmd/desktop/build/bin/$(DESKTOP_BINARY)"; exit 1; }; \
			cp "cmd/desktop/build/bin/$(DESKTOP_BINARY)" "$(RELEASE_DIR)/gpusched-desktop-$$os-$$arch"; \
			chmod +x "$(RELEASE_DIR)/gpusched-desktop-$$os-$$arch"; \
		fi; \
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
	@$(MAKE) desktop-release
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
