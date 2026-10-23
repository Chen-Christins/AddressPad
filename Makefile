VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell git log -1 --format=%cI)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build run test lint snapshot
build:
	go build -ldflags "$(LDFLAGS)" -o bin/apad ./
run:
	go run -ldflags "$(LDFLAGS)" .
test:
	go test ./...
lint:
	go vet ./...
snapshot:
	goreleaser release --snapshot --clean
