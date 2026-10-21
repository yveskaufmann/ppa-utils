SHELL := /usr/bin/env bash

# Command-line arguments passed to the binaries during run or test targets
ARGS ?=
PREFIX ?= /usr/local

##################################
# Build - Variables
##################################

# Semantic version passed into binaries via -ldflags (see internal/version.Version).
VERSION ?= 0.0.1
VERSION_LDFLAGS := -X yv35.com/ppa/ppa-utils/internal/version.Version=$(VERSION)

# Go environment and architecture settings
GO_BIN_DIR := $(shell GOBIN=$$(go env GOBIN); if [ -n "$$GOBIN" ]; then echo "$$GOBIN"; else go env GOPATH | cut -d: -f1 | sed 's|$$|/bin|'; fi)
GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)

# Main entry points for each binary
CLI_ENTRY_DIR := ./cmd/ppautil

# Output directory for built binaries
BUILD_OUTPUT_DIR := bin/

##################################
# BUILD - TARGETS
##################################

.PHONY: build
build: build-cli   # Build all application binaries

.PHONY: build-cli
build-cli: # Build cli application binary
	go build -ldflags "$(VERSION_LDFLAGS)" -o $(BUILD_OUTPUT_DIR) $(CLI_ENTRY_DIR)

.PHONY: clean
clean: # Clean build artifacts
	rm -f $(BUILD_OUTPUT_DIR)/*

.PHONY: clean-cache
clean-cache: # Clean Go build cache
	go clean -cache -modcache

##################################
# RUN & DEV - TARGETS
##################################

.PHONY: run
run: # Run the CLI application (development)
	@APP_ENV=dev go run -ldflags "$(VERSION_LDFLAGS)" $(CLI_ENTRY_DIR) $(ARGS)

##################################
# TESTING - TARGETS
##################################

.PHONY: test
test:  # Run unit tests
	go test ./internal/... -v $(ARGS)

##################################
# UTILITY - TARGETS
##################################

.PHONY: help
help: # Show this help
	@printf "Available targets:\n\n"
	@awk 'BEGIN {FS = ":.*#"} /^[a-zA-Z0-9_-]+:.*#/ {printf "  \033[1;37m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
