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

# --- Test ---

test:
	go test $(shell go list ./... | grep -v cmd/desktop)

test-mock:
	go test -tags mock ./...

# --- Clean ---

clean:
	rm -rf $(BUILD_DIR)
	rm -rf frontend/.svelte-kit frontend/build
