VERSION ?= $(shell jq -r .version manifest.json)
LDFLAGS := -s -w
BIN     := server

.PHONY: build package clean test bump-version version

build:
	mkdir -p $(BIN)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BIN)/plannner-connector ./cmd/plannner-connector
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BIN)/plannner-connector-darwin ./cmd/plannner-connector
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BIN)/plannner-connector.exe ./cmd/plannner-connector

package: build
	rm -f plannner-connector.mcpb
	zip -j plannner-connector.mcpb manifest.json
	zip -r plannner-connector.mcpb $(BIN)/

test:
	go test ./...

clean:
	rm -rf $(BIN) plannner-connector.mcpb

bump-version:
	@test -n "$(V)" || (echo "Usage: make bump-version V=x.y.z" && exit 1)
	jq '.version = "$(V)"' manifest.json > tmp.json && mv tmp.json manifest.json
	@echo "Version set to $(V)"

version:
	@echo $(VERSION)
