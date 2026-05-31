BUILD_DIR = build
CLI_BINARY = gpusched
CLI_MOCK_BINARY = gpusched_mock
DESKTOP_BINARY = gpusched-desktop

.PHONY: cli cli-mock desktop clean test frontend

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


# --- Test ---

test:
	go test $(shell go list ./... | grep -v cmd/desktop)

test-mock:
	go test -tags mock ./...

# --- Clean ---

clean:
	rm -rf $(BUILD_DIR)
	rm -rf frontend/.svelte-kit frontend/build
	rm -rf release

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
	@echo "Building binaries for v$(NEW_VERSION)..."
	@rm -rf release && mkdir -p release
	@GOOS=linux GOARCH=amd64 go build -tags mock -o release/gpusched-linux-amd64 ./cmd/
	@GOOS=linux GOARCH=arm64 go build -tags mock -o release/gpusched-linux-arm64 ./cmd/
	@GOOS=darwin GOARCH=amd64 go build -tags mock -o release/gpusched-darwin-amd64 ./cmd/
	@GOOS=darwin GOARCH=arm64 go build -tags mock -o release/gpusched-darwin-arm64 ./cmd/
	@echo "Committing version bump..."
	@git add $(VERSION_FILE)
	@git commit -m "Bump version to v$(NEW_VERSION)"
	@git tag v$(NEW_VERSION)
	@git push && git push --tags
	@echo "Creating GitHub release..."
	@gh release create v$(NEW_VERSION) \
		--title "v$(NEW_VERSION)" \
		--notes "Release v$(NEW_VERSION)" \
		release/gpusched-linux-amd64 \
		release/gpusched-linux-arm64 \
		release/gpusched-darwin-amd64 \
		release/gpusched-darwin-arm64
	@echo "✓ Released v$(NEW_VERSION)"
