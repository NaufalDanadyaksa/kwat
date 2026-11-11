# KWAT CLI Agent Makefile

BINARY_NAME=kwat
BUILD_DIR=bin
VERSION?=0.1.0-dev
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)"

.PHONY: all build test clean run lint

all: test build

build:
	@mkdir -p $(BUILD_DIR)
	@export PATH="/usr/local/bin:$$PATH" && go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kwat
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

test:
	@export PATH="/usr/local/bin:$$PATH" && go test -v ./...

clean:
	@rm -rf $(BUILD_DIR)
	@echo "Cleaned build artifacts"

run:
	@export PATH="/usr/local/bin:$$PATH" && go run ./cmd/kwat
