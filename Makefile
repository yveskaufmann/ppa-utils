SHELL := /usr/bin/env bash

# Command-line arguments passed to the binaries during run or test targets
ARGS ?=
PREFIX ?= /usr/local

##################################
# Build - Variables
##################################

# Semantic version passed into binaries via -ldflags (see internal/version.Version).
VERSION ?= 0.0.1
VERSION_LDFLAGS := -X yv35.com/ppa-utils/internal/version.Version=$(VERSION)

# Go environment and architecture settings
GO_BIN_DIR := $(shell GOBIN=$$(go env GOBIN); if [ -n "$$GOBIN" ]; then echo "$$GOBIN"; else go env GOPATH | cut -d: -f1 | sed 's|$$|/bin|'; fi)
GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)

# Main entry points for each binary
CLI_ENTRY_DIR := ./cmd/ppa-utils

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
# FORMAT & HOOKS - TARGETS
##################################

.PHONY: fmt
fmt: # Format all Go files with gofmt
	gofmt -l -w cmd internal

.PHONY: hooks
hooks: # Install the git hooks from .githooks (formats staged Go files on commit)
	git config core.hooksPath .githooks

##################################
# RELEASE - TARGETS
##################################

# Set YES=1 to skip the confirmation prompt of the release target
YES ?=

.PHONY: release
release: # Trigger the release-tag workflow on GitHub (asks for confirmation, YES=1 skips it)
	@command -v gh >/dev/null || { echo "gh is required"; exit 1; }
	@[ "$$(git branch --show-current)" = "main" ] || { echo "release must be cut from main"; exit 1; }
	@git diff --quiet && git diff --cached --quiet || { echo "working tree is not clean"; exit 1; }
	@git fetch origin main --tags --quiet
	@[ "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" ] || { echo "main is not in sync with origin/main"; exit 1; }
	@last=$$(git describe --tags --abbrev=0 2>/dev/null); \
	echo "Commits since $${last:-the beginning}:"; \
	git log --oneline $${last:+$$last..}HEAD
	@if [ -z "$(YES)" ]; then \
		read -r -p "Trigger release from main? [y/N] " answer; \
		case "$$answer" in y|Y) ;; *) echo "aborted"; exit 1;; esac; \
	fi
	gh workflow run release-tag.yml --ref main
	@sleep 3
	gh run watch $$(gh run list --workflow=release-tag.yml --limit 1 --json databaseId -q '.[0].databaseId')

.PHONY: release-status
release-status: # Show the latest release-build workflow runs
	gh run list --workflow=release-build.yml --limit 5

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
