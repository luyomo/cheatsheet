VERSION = $(shell git describe --tags 2>/dev/null || echo "0.0.1-dev")
BINARY_NAME = "dm-toolkit"
ARCHIVE_NAME = "$(VERSION).tar.gz"

build:
	go build -o bin/$(BINARY_NAME)
fmt:
	find . -name "*.go" -exec go fmt {} \;
release-linux: build
	mkdir -p release/$(VERSION)
	cp bin/$(BINARY_NAME) release/$(VERSION)/
	tar -czf $(ARCHIVE_NAME) -C release $(VERSION)

github-release: release
	gh release create $(VERSION) $(ARCHIVE_NAME) --title "$(VERSION)" --notes "Release notes here"
