# Rotta build/test/release helpers
.PHONY: build install run test test-ci test-verbose test-coverage test-critical-path-statement-coverage test-changed-module-mutation fmt fmt-check lint verify verify-ci cross clean tidy deps release release-check hooks-install help

GOPATH := $(shell go env GOPATH)
GOTESTSUM := $(GOPATH)/bin/gotestsum
LEFTHOOK := $(GOPATH)/bin/lefthook
GOLANGCI_LINT_VERSION ?= v2.5.0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -s -w -X main.version=$(VERSION)

BINARY = rotta
BUILD_DIR = bin
MAIN_PATH = ./cmd/rotta

build:
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(MAIN_PATH)

install: build
	@mkdir -p "$${GOPATH:-$(HOME)/go}/bin"
	@cp $(BUILD_DIR)/$(BINARY) "$${GOPATH:-$(HOME)/go}/bin/$(BINARY)"

run:
	@go run $(MAIN_PATH)

fmt:
	@go fmt ./...

fmt-check:
	@UNFORMATTED=$$(find . -name '*.go' -not -path './vendor/*' -exec gofmt -l {} +); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "The following files need gofmt:"; \
		echo "$$UNFORMATTED"; \
		exit 1; \
	fi

test:
	@go test ./...

test-ci:
	@echo "Running tests..."
	@if command -v $(GOTESTSUM) >/dev/null 2>&1; then \
		$(GOTESTSUM) --format testdox -- -race -coverprofile=coverage.out ./...; \
	else \
		go test -v -race -coverprofile=coverage.out ./...; \
	fi

test-verbose:
	@go test -v ./...

test-coverage:
	@$(MAKE) test-ci
	@go tool cover -html=coverage.out -o coverage.html

test-critical-path-statement-coverage:
	@go test ./... -coverpkg=./... -coverprofile=critical-path.out -count=1
	@python3 scripts/critical_path_statement_coverage.py --inventory .rotta/critical-path-coverage.json --profile critical-path.out

test-changed-module-mutation:
	@python3 scripts/changed_module_mutation.py --timeout 900

lint:
	@go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

verify: fmt-check lint test-ci build

verify-ci: verify

cross:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-arm64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe $(MAIN_PATH)

tidy:
	@go mod tidy

deps:
	@go mod download

release-check:
	@echo "Checking release prerequisites..."
	@if [ -z "$(shell git status --porcelain)" ]; then \
		echo "✓ Working directory is clean"; \
	else \
		echo "✗ Working directory has uncommitted changes"; \
		exit 1; \
	fi
	@if git describe --exact-match --tags HEAD >/dev/null 2>&1; then \
		echo "✓ Current commit is tagged"; \
	else \
		echo "✗ Current commit is not tagged"; \
		exit 1; \
	fi

release: release-check test lint
	@echo "Release checks passed; push tag $(VERSION) to trigger CI release pipeline."

clean:
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

hooks-install:
	@go install github.com/evilmartians/lefthook@latest
	@$(LEFTHOOK) install

help:
	@echo "Available targets:"
	@echo "  build          - Build rotta"
	@echo "  install        - Install binary to $${GOPATH}/bin"
	@echo "  run            - Run the binary"
	@echo "  fmt            - Format Go files"
	@echo "  fmt-check      - Check gofmt/goimports formatting"
	@echo "  test           - Run unit tests"
	@echo "  test-ci        - Run tests with race + coverage for CI"
	@echo "  test-verbose   - Run tests verbose"
	@echo "  test-coverage  - Generate coverage report"
	@echo "  test-critical-path-statement-coverage - Verify named critical functions with Go statement coverage"
	@echo "  test-changed-module-mutation - Run bounded isolated critical changed-module mutations"
	@echo "  lint           - Run golangci-lint"
	@echo "  cross          - Build all supported OS/arch variants"
	@echo "  verify         - Run fmt-check, lint, test-ci, build"
	@echo "  release-check  - Verify clean tree and tagged commit"
	@echo "  release        - Verify and announce release trigger"
	@echo "  clean          - Remove build artifacts"
	@echo "  deps           - Download dependencies"
	@echo "  hooks-install  - Install lefthook git hooks"
