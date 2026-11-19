VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build test golden-update lint bench release-dry

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/devtooltui ./cmd/devtooltui

test:
	go test -race ./...

golden-update:
	UPDATE_GOLDEN=1 go test ./internal/...

lint:
	golangci-lint run && govulncheck ./...

bench:
	go test -run '^$$' -bench . -benchmem ./...

release-dry:
	goreleaser release --snapshot --clean
