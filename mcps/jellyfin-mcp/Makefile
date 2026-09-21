VERSION := $(shell cat npm/VERSION | tr -d '[:space:]')
BINARY := jellyfin-mcp
NPM_DIR := npm
BIN_TARGET := $(NPM_DIR)/jellyfin-mcp/bin/$(BINARY)
GO_SRC := $(shell find internal -name '*.go') main.go go.mod go.sum

GO_BUILD_FLAGS := -trimpath -ldflags="-s -w -X github.com/jaredtrent/jellyfin-mcp/internal/server.version=$(VERSION)"

.PHONY: build build-local test vet lint check list-binaries version-sync npm-publish release \
        clean version-bump

## build: Compile Go binary for linux/amd64 (npm package / MetaMCP target)
build: $(BIN_TARGET)

$(BIN_TARGET): $(GO_SRC)
	mkdir -p $(dir $(BIN_TARGET))
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GO_BUILD_FLAGS) -o $(BIN_TARGET) .

## build-local: Compile Go binary for native platform (dev testing)
build-local:
	mkdir -p build
	go build $(GO_BUILD_FLAGS) -o build/$(BINARY) .

## test: Run all tests
test:
	go test ./...

## vet: Static analysis
vet:
	go vet ./...

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## check: Run vet, lint, and tests together
check: vet lint test

## list-binaries: Show built binaries
list-binaries:
	@echo "NPM binary:"
	@ls -lh $(BIN_TARGET) 2>/dev/null || echo "  (not built — run 'make build')"
	@echo "Local binary:"
	@ls -lh build/$(BINARY) 2>/dev/null || echo "  (not built — run 'make build-local')"

## version-sync: Update package.json to match npm/VERSION
version-sync:
	@echo "Syncing version $(VERSION)..."
	@jq --arg v "$(VERSION)" '.version = $$v' $(NPM_DIR)/jellyfin-mcp/package.json > $(NPM_DIR)/jellyfin-mcp/package.json.tmp && \
		mv $(NPM_DIR)/jellyfin-mcp/package.json.tmp $(NPM_DIR)/jellyfin-mcp/package.json
	@echo "Done. Package at version $(VERSION)."

## npm-publish: Publish the package to npmjs.com
npm-publish:
	@if [ ! -f $(NPM_DIR)/.npmrc ]; then \
		echo "Error: npm/.npmrc not found. Copy npm/.npmrc.template to npm/.npmrc and add your token."; \
		exit 1; \
	fi
	@echo "Publishing @jaredtrent/jellyfin-mcp v$(VERSION)..."
	cd $(NPM_DIR)/jellyfin-mcp && npm publish --userconfig=../.npmrc
	@echo "Published."

## release: Full workflow — check, clean, sync version, build, tag, publish
release: check clean version-sync build npm-publish
	git tag -a "v$(VERSION)" -m "Release $(VERSION)"
	git push origin "v$(VERSION)"

## clean: Remove built binaries
clean:
	rm -f $(BIN_TARGET) build/$(BINARY)

## version-bump: Bump to today's date (CalVer YYYY.MMDD.#), incrementing build # if same day
version-bump:
	@current=$$(cat $(NPM_DIR)/VERSION | tr -d '[:space:]'); \
	today_year=$$(date +%Y); \
	today_mmdd=$$(date +%-m%d); \
	cur_year=$$(echo $$current | cut -d. -f1); \
	cur_mmdd=$$(echo $$current | cut -d. -f2); \
	cur_build=$$(echo $$current | cut -d. -f3); \
	if [ "$$today_year" = "$$cur_year" ] && [ "$$today_mmdd" = "$$cur_mmdd" ]; then \
		new="$$today_year.$$today_mmdd.$$((cur_build + 1))"; \
	else \
		new="$$today_year.$$today_mmdd.1"; \
	fi; \
	echo "$$new" > $(NPM_DIR)/VERSION; \
	echo "Version bumped: $$current -> $$new"
	@$(MAKE) version-sync
