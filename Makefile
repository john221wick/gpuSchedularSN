BUILD_DIR = build
BINARY = gpusched
MOCK_BINARY = gpusched_mock

.PHONY: run mock clean

run: $(BUILD_DIR)/$(BINARY)
	./$(BUILD_DIR)/$(BINARY)

mock: $(BUILD_DIR)/$(MOCK_BINARY)
	./$(BUILD_DIR)/$(MOCK_BINARY)

$(BUILD_DIR)/$(BINARY):
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/

$(BUILD_DIR)/$(MOCK_BINARY):
	@mkdir -p $(BUILD_DIR)
	go build -tags mock -o $(BUILD_DIR)/$(MOCK_BINARY) ./cmd/

clean:
	rm -rf $(BUILD_DIR)
