VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell git log -1 --format=%cI)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
EXE :=
ifeq ($(OS),Windows_NT)
EXE := .exe
endif

.PHONY: build run test lint snapshot
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/apad$(EXE) ./
run:
	go run -ldflags "$(LDFLAGS)" .
test:
	go test ./...
lint:
	go vet ./...
snapshot:
	goreleaser release --snapshot --clean
