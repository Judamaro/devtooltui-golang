VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

FUZZTIME ?= 30s

.PHONY: build test golden-update lint bench fuzz docs man release-dry

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/devtooltui ./cmd/devtooltui

test:
	go test -race ./...

golden-update:
	UPDATE_GOLDEN=1 go test ./internal/... ./cmd/...

lint:
	golangci-lint run && govulncheck ./...

bench:
	go test -run '^$$' -bench . -benchmem ./...

# Runs every Fuzz* target for FUZZTIME each (go test -fuzz takes one at a time).
fuzz:
	@set -e; for pkg in $$(go list ./...); do \
		for t in $$(go test -list '^Fuzz' $$pkg | grep '^Fuzz'); do \
			echo "== $$pkg $$t"; \
			go test -run '^$$' -fuzz "^$$t$$" -fuzztime $(FUZZTIME) $$pkg; \
		done; \
	done

# docs/index.md and docs/tools/*.md, from the registry and the goldens.
docs:
	go run ./cmd/gendocs -site docs

# Man pages in man/ (GoReleaser ships them; not committed).
man:
	go run ./cmd/gendocs -man man

release-dry:
	goreleaser release --snapshot --clean
