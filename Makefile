NAME     := plannner-connector
DIST     := dist
MCPB     := $(NAME).mcpb
TMPDIR   := $(shell mktemp -d)

VERSION  ?= $(shell node -p "require('./manifest.json').version")

.PHONY: build package clean version bump-version

## Build TypeScript → dist/
build:
	npm run build

## Set version in manifest.json and package.json
bump-version:
	@test -n "$(V)" || (echo "Usage: make bump-version V=1.2.3" && exit 1)
	node -e "\
	  const fs = require('fs'); \
	  for (const f of ['manifest.json','package.json']) { \
	    const j = JSON.parse(fs.readFileSync(f,'utf8')); \
	    j.version = '$(V)'; \
	    fs.writeFileSync(f, JSON.stringify(j, null, 2) + '\n'); \
	  }"
	@echo "Version set to $(V)"

## Build + package into .mcpb (production deps only)
package: build
	rm -f $(MCPB)
	cp -r $(DIST) manifest.json package.json $(TMPDIR)/
	cd $(TMPDIR) && npm install --omit=dev --ignore-scripts --no-audit --no-fund 2>&1
	cd $(TMPDIR) && zip -r $(CURDIR)/$(MCPB) manifest.json package.json $(DIST)/ node_modules/
	rm -rf $(TMPDIR)
	@echo "Packaged $(MCPB) v$(VERSION) ($$(du -h $(MCPB) | cut -f1))"

## Print current version
version:
	@echo $(VERSION)

## Remove build artifacts
clean:
	rm -rf $(DIST) $(MCPB)
