## Root Makefile — delegates to go/Makefile

GO_DIR := go

.PHONY: build package clean test version bump-version

## Cross-compile Go binaries (linux/darwin/windows)
build:
	$(MAKE) -C $(GO_DIR) build

## Build + package into .mcpb binary bundle (output at project root)
package:
	$(MAKE) -C $(GO_DIR) package
	cp $(GO_DIR)/plannner-connector.mcpb .
	@echo "Ready: plannner-connector.mcpb ($$(du -h plannner-connector.mcpb | cut -f1))"

## Run tests
test:
	$(MAKE) -C $(GO_DIR) test

## Print current version
version:
	$(MAKE) -C $(GO_DIR) version

## Set version in manifest.json
bump-version:
	@test -n "$(V)" || (echo "Usage: make bump-version V=x.y.z" && exit 1)
	$(MAKE) -C $(GO_DIR) bump-version V=$(V)

## Remove build artifacts
clean:
	$(MAKE) -C $(GO_DIR) clean
	rm -f plannner-connector.mcpb
