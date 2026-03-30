## Root Makefile — delegates to go/Makefile

GO_DIR := go

.PHONY: build package clean test version bump-version

## Cross-compile Go binaries (linux/darwin/windows)
build:
	$(MAKE) -C $(GO_DIR) build

## Build + package into .mcpb binary bundle
package:
	$(MAKE) -C $(GO_DIR) package

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
