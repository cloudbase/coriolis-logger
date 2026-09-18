.PHONY: all
all: build-release

.PHONY: fmt
fmt: ## Format the code.
	go fmt ./...

.PHONY: vet
vet: ## Run static code analysis.
	go vet ./...

COVER_OUTDIR ?= bin
COVER_OUTFILE_RAW ?= $(COVER_OUTDIR)/coverage.raw
COVER_OUTFILE_HTML ?= $(COVER_OUTDIR)/coverage.html
# Filter executed tests by the "TEST_RE" regex
TEST_RE ?= .*
# Skip tests that match the "TEST_SKIP_RE" regex
TEST_SKIP_RE ?=
TEST_CMD = go test ./... \
	-run "$(TEST_RE)" \
	-coverpkg=coriolis-logger/... \
	-coverprofile=$(COVER_OUTFILE_RAW)
ifneq ($(strip $(TEST_SKIP_RE)),)
TEST_CMD += -skip "$(TEST_SKIP_RE)"
endif

.PHONY: test-unit
test-unit: fmt vet ## Run coriolis-logger unit tests.
	mkdir -p $(COVER_OUTDIR)
	$(TEST_CMD)
	go tool cover -html=$(COVER_OUTFILE_RAW) -o=$(COVER_OUTFILE_HTML)

.PHONY: test-unit-verbose
test-unit-verbose: fmt vet ## Run coriolis-logger unit tests in verbose mode.
	mkdir -p $(COVER_OUTDIR)
	$(TEST_CMD) -test.v
	go tool cover -html=$(COVER_OUTFILE_RAW) -o=$(COVER_OUTFILE_HTML)

.PHONY: test
test: test-unit ## Run all coriolis-logger tests.

.PHONY: build-dev
build-dev: fmt vet ## Generate coriolis-logger dev build.
	# Dev build, meant to build fast and run on the dev machine:
	#   * use the host architecture
	#   * avoid rebuilding unmodified components
	mkdir -p bin
	go build \
		-o bin/coriolis-logger \
		./cmd/coriolis-logger

.PHONY: build-dev-dbg
build-dev-dbg: fmt vet ## Generate coriolis-logger dev build, disabling compiler optimizations.
	mkdir -p bin
	go build -gcflags="all=-N -l" \
		-o bin/coriolis-logger \
		./cmd/coriolis-logger

.PHONY: build-release
build-release: fmt vet ## Generate coriolis-logger release build.
	# Release build, meant to be deployed as the Coriolis logging service:
	#   * strip debug symbols
	#   * build for Linux x86_64
	#   * rebuild everything
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a \
		-ldflags="-w -s" \
		-o bin/coriolis-logger \
		./cmd/coriolis-logger

CONFIG ?= testdata/config.toml

.PHONY: run
run: ## Run coriolis-logger.
	bin/coriolis-logger -config "$(CONFIG)"

# We'll reuse the "help" generator from operator-sdk (Apache-2).
.DEFAULT_GOAL := help
.PHONY: help
help: ## Show this help screen.
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
