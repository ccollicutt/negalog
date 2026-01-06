# Makefile for Go Project - Consolidated Commands
# Uses variable-based targets for flexibility
#
# This Makefile provides a unified interface for building, testing, linting,
# and quality-checking Go projects. Instead of many individual targets, it uses
# a small number of flexible targets with TYPE= variables to select behavior.
#
# Philosophy:
#   - Fewer targets, more options (use TYPE= to select behavior)
#   - Auto-install missing tools when possible
#   - Verbose output to /tmp/ log files, summaries on stdout
#   - CI-friendly with proper exit codes

# =============================================================================
# CONFIGURATION
# =============================================================================

# Shell configuration - use bash for consistent behavior
SHELL := /bin/bash

# Go configuration
GO := /usr/local/go/bin/go
GOBIN := $(HOME)/go/bin
GOFLAGS :=
# Ensure Go tools can find go binary
export PATH := /usr/local/go/bin:$(GOBIN):$(PATH)

# Project naming
BINARY_NAME := negalog

# Output directories
BUILD_DIR := bin
COVERAGE_DIR := coverage
LOG_DIR := /tmp/negalog-logs

# Version management - read from VERSION file
# Format: MAJOR.MINOR.PATCH (semver)
VERSION_FILE := VERSION
VERSION := $(shell cat $(VERSION_FILE) 2>/dev/null || echo "0.0.0")

# Package path for version injection
VERSION_PKG := negalog/internal/cli/commands

# Ldflags to inject version into binary
LDFLAGS := -ldflags "-X $(VERSION_PKG).Version=$(VERSION)"

# =============================================================================
# PHONY TARGETS
# =============================================================================

.PHONY: help build test clean fmt verify deps run check ci install-tools bump bump-minor bump-major version release push emoji-check publish

# Default target when running just 'make'
.DEFAULT_GOAL := help

# =============================================================================
# HELP
# =============================================================================

# Show available targets and comprehensive usage information.
# Displays all core commands, examples, and variable documentation.
# This is the default target when running 'make' with no arguments.
help: ##@internal Show available targets and usage
	@echo ""
	@echo "Go Project Build System"
	@echo ""
	@echo "CORE COMMANDS:"
	@echo ""
	@echo "  make build TYPE=<type>              Build operations (binary|all|race)"
	@echo "  make test TYPE=<type>               Run tests, linters, and quality checks"
	@echo "  make clean TYPE=<type>              Clean artifacts (build|test|all)"
	@echo "  make fmt                            Format code with gofmt and goimports"
	@echo "  make verify                         Verify module dependencies"
	@echo "  make deps                           Download and verify dependencies"
	@echo ""
	@echo "  make version                        Show current version (auto-bumps on build)"
	@echo ""
	@echo "TEST TYPES:"
	@echo ""
	@echo "  Tests:    unit, integration, e2e, coverage, bench"
	@echo "  Linters:  vet, staticcheck, golangci, shadow, lint (all linters)"
	@echo "  Quality:  gosec, vulncheck, deadcode, quality (all quality checks)"
	@echo "  Combined: all (tests + linters + quality)"
	@echo ""
	@echo "EXAMPLES:"
	@echo ""
	@echo "  make build                          # Build main binary"
	@echo "  make build TYPE=race                # Build with race detector"
	@echo "  make test                           # Run unit tests (default)"
	@echo "  make test TYPE=unit PKG=./pkg/...   # Test specific package"
	@echo "  make test TYPE=coverage             # Run tests with coverage report"
	@echo "  make test TYPE=bench BENCH=.        # Run all benchmarks"
	@echo "  make test TYPE=lint                 # Run all linters"
	@echo "  make test TYPE=golangci FIX=1       # Run golangci-lint with auto-fix"
	@echo "  make test TYPE=quality              # Run all quality checks"
	@echo "  make test TYPE=gosec                # Run security scanner"
	@echo "  make test TYPE=all                  # Run everything"
	@echo "  make clean TYPE=all                 # Clean everything"
	@echo ""
	@echo "VARIABLES:"
	@echo ""
	@echo "  Build variables:"
	@echo "    TYPE      Build type: binary|all|race (default: binary)"
	@echo "    TAGS      Build tags (e.g., TAGS='integration debug')"
	@echo ""
	@echo "  Test variables:"
	@echo "    TYPE      See TEST TYPES above (default: unit)"
	@echo "    PKG       Package path to test (default: ./...)"
	@echo "    TEST      Test name pattern (go test -run)"
	@echo "    BENCH     Benchmark pattern (go test -bench)"
	@echo "    COUNT     Run count for tests (go test -count)"
	@echo "    TIMEOUT   Test timeout (default: 10m)"
	@echo "    VERBOSE   Verbose output (VERBOSE=1)"
	@echo "    RACE      Enable race detector (RACE=1)"
	@echo "    FIX       Auto-fix issues (FIX=1, golangci only)"
	@echo ""
	@echo "  Clean variables:"
	@echo "    TYPE      Clean type: build|test|all (default: build)"
	@echo ""

# =============================================================================
# VERSION MANAGEMENT
# =============================================================================

# Show current version
version: ## Show current version
	@echo "$(VERSION)"

bump: ##@internal Bump patch version (0.1.0 -> 0.1.1)
	@CURRENT=$$(cat $(VERSION_FILE)); \
	MAJOR=$$(echo $$CURRENT | cut -d. -f1); \
	MINOR=$$(echo $$CURRENT | cut -d. -f2); \
	PATCH=$$(echo $$CURRENT | cut -d. -f3); \
	NEW_PATCH=$$((PATCH + 1)); \
	NEW_VERSION="$$MAJOR.$$MINOR.$$NEW_PATCH"; \
	echo "$$NEW_VERSION" > $(VERSION_FILE); \
	echo "Version: $$CURRENT -> $$NEW_VERSION"

bump-minor: ##@internal Bump minor version (0.1.0 -> 0.2.0)
	@CURRENT=$$(cat $(VERSION_FILE)); \
	MAJOR=$$(echo $$CURRENT | cut -d. -f1); \
	MINOR=$$(echo $$CURRENT | cut -d. -f2); \
	NEW_MINOR=$$((MINOR + 1)); \
	NEW_VERSION="$$MAJOR.$$NEW_MINOR.0"; \
	echo "$$NEW_VERSION" > $(VERSION_FILE); \
	echo "Version: $$CURRENT -> $$NEW_VERSION"

bump-major: ##@internal Bump major version (0.1.0 -> 1.0.0)
	@CURRENT=$$(cat $(VERSION_FILE)); \
	MAJOR=$$(echo $$CURRENT | cut -d. -f1); \
	NEW_MAJOR=$$((MAJOR + 1)); \
	NEW_VERSION="$$NEW_MAJOR.0.0"; \
	echo "$$NEW_VERSION" > $(VERSION_FILE); \
	echo "Version: $$CURRENT -> $$NEW_VERSION"

release: ## Push code and create a release tag from VERSION file
	@CURRENT=$$(cat $(VERSION_FILE)); \
	echo "Current version: $$CURRENT"; \
	echo ""; \
	echo "Bump version? [p]atch, [m]inor, [M]ajor, [n]o (use current): "; \
	read -r BUMP; \
	case "$$BUMP" in \
		p|patch) $(MAKE) bump; VERSION=$$(cat $(VERSION_FILE));; \
		m|minor) $(MAKE) bump-minor; VERSION=$$(cat $(VERSION_FILE));; \
		M|major) $(MAKE) bump-major; VERSION=$$(cat $(VERSION_FILE));; \
		n|no|"") VERSION=$$CURRENT;; \
		*) echo "Invalid option"; exit 1;; \
	esac; \
	TAG="v$$VERSION"; \
	echo ""; \
	echo "Releasing $$TAG..."; \
	echo ""; \
	echo "Running tests..."; \
	$(MAKE) test TYPE=all NOBUMP=1 || { echo "FAIL: Tests failed, aborting release"; exit 1; }; \
	echo ""; \
	echo "Pushing code to origin..."; \
	git add -A && git commit -m "Release $$TAG" || true; \
	git push origin main || { echo "FAIL: Push failed"; exit 1; }; \
	echo ""; \
	echo "Creating tag $$TAG..."; \
	git tag -a "$$TAG" -m "Release $$TAG" || { echo "FAIL: Tag creation failed (tag may already exist)"; exit 1; }; \
	echo "Pushing tag $$TAG..."; \
	git push origin "$$TAG" || { echo "FAIL: Tag push failed"; exit 1; }; \
	echo ""; \
	echo "PASS: Released $$TAG"; \
	echo "  GitHub Actions will now build and publish the release."

# =============================================================================
# BUILD
# =============================================================================

# Build Go binaries with various configurations.
# Supports building a single binary, all packages, or with race detection enabled.
# Build metadata (version, commit, build time) is automatically injected via ldflags.
#
# Variables:
#   TYPE=binary|all|race  - Build configuration (default: binary)
#     binary  - Build the main binary only to bin/<name>
#     all     - Build all packages (verify compilation) then build main binary
#     race    - Build with race detector enabled (slower but catches data races)
#   TAGS=<tags>  - Space-separated build tags (e.g., TAGS='integration debug')
#
# Examples:
#   make build                    # Build main binary
#   make build TYPE=race          # Build with race detector for testing
#   make build TYPE=all           # Verify all packages compile
#   make build TAGS=integration   # Build with integration tag
#
# Output: bin/<binary-name>
build: ## Build Go binaries. Variables: TYPE=binary|all|race (default: binary). binary=build main binary to bin/<name>, all=build all packages then main binary, race=build with race detector enabled. TAGS=<tags> for build tags. Examples: make build, make build TYPE=race, make build TAGS=integration. Output: bin/<binary-name>
	@mkdir -p $(BUILD_DIR) $(LOG_DIR)
	@TYPE=$(or $(TYPE),binary); \
	LOGFILE="$(LOG_DIR)/build-$$(date +%Y%m%d-%H%M%S).log"; \
	TAGS_FLAG=""; \
	if [ -n "$(TAGS)" ]; then TAGS_FLAG="-tags '$(TAGS)'"; fi; \
	CURRENT=$$(cat $(VERSION_FILE)); \
	if [ "$(NOBUMP)" = "1" ]; then \
		NEW_VERSION="$$CURRENT"; \
	else \
		MAJOR=$$(echo $$CURRENT | cut -d. -f1); \
		MINOR=$$(echo $$CURRENT | cut -d. -f2); \
		PATCH=$$(echo $$CURRENT | cut -d. -f3); \
		NEW_PATCH=$$((PATCH + 1)); \
		NEW_VERSION="$$MAJOR.$$MINOR.$$NEW_PATCH"; \
		echo "$$NEW_VERSION" > $(VERSION_FILE); \
	fi; \
	LDFLAGS="-X $(VERSION_PKG).Version=$$NEW_VERSION"; \
	case "$$TYPE" in \
		binary) \
			echo "Building $(BINARY_NAME) v$$NEW_VERSION..."; \
			$(GO) build $(GOFLAGS) -ldflags "$$LDFLAGS" $$TAGS_FLAG -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/cli > "$$LOGFILE" 2>&1; \
			if [ $$? -eq 0 ]; then echo "PASS: Built $(BUILD_DIR)/$(BINARY_NAME)"; else echo "FAIL: Build failed"; cat "$$LOGFILE"; exit 1; fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		race) \
			echo "Building $(BINARY_NAME) v$$NEW_VERSION with race detector..."; \
			$(GO) build $(GOFLAGS) -ldflags "$$LDFLAGS" $$TAGS_FLAG -race -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/cli > "$$LOGFILE" 2>&1; \
			if [ $$? -eq 0 ]; then echo "PASS: Built $(BUILD_DIR)/$(BINARY_NAME) (race)"; else echo "FAIL: Build failed"; cat "$$LOGFILE"; exit 1; fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		all) \
			echo "Building all packages v$$NEW_VERSION..."; \
			$(GO) build $(GOFLAGS) $$TAGS_FLAG ./... > "$$LOGFILE" 2>&1; \
			$(GO) build $(GOFLAGS) -ldflags "$$LDFLAGS" $$TAGS_FLAG -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/cli >> "$$LOGFILE" 2>&1; \
			if [ $$? -eq 0 ]; then echo "PASS: All packages built"; else echo "FAIL: Build failed"; cat "$$LOGFILE"; exit 1; fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		*) \
			echo "Unknown build type: $$TYPE"; \
			echo "Valid types: binary, all, race"; \
			exit 1; \
			;; \
	esac

# =============================================================================
# TEST
# =============================================================================

# Run tests, linters, and quality checks - unified interface.
# Supports unit tests, integration tests, e2e tests, coverage, benchmarks,
# static analysis linters, and security/quality scanners.
#
# Variables:
#   TYPE=unit|integration|e2e|coverage|bench|vet|staticcheck|golangci|shadow|lint|gosec|vulncheck|deadcode|quality|all
#     unit        - Run unit tests only (uses -short flag, skips slow tests)
#     integration - Run integration tests (uses -tags integration)
#     e2e         - Run end-to-end tests (uses -tags e2e)
#     coverage    - Run tests with coverage reporting, generates HTML report
#     bench       - Run benchmarks with memory allocation stats
#     vet         - Built-in Go static analyzer, catches common mistakes
#     staticcheck - Advanced static analyzer, finds bugs and performance issues
#     golangci    - Meta-linter running 50+ linters, highly configurable
#     shadow      - Detects variable shadowing which can cause subtle bugs
#     lint        - Run all linters (vet + staticcheck + golangci)
#     gosec       - Security scanner, finds SQL injection, XSS, hardcoded creds
#     vulncheck   - Official Go vulnerability scanner, checks deps against Go vuln DB
#     deadcode    - Finds unreachable/unused code that can be safely removed
#     quality     - Run all quality checks (gosec + vulncheck + deadcode)
#     all         - Run everything: unit, integration, coverage, lint, quality
#   PKG=<path>      - Package path to test (default: ./...)
#   TEST=<pattern>  - Filter tests by name pattern (maps to go test -run)
#   BENCH=<pattern> - Benchmark pattern for TYPE=bench (default: .)
#   COUNT=<n>       - Run tests n times (useful for flaky test detection)
#   TIMEOUT=<dur>   - Test timeout duration (default: 10m)
#   VERBOSE=1       - Enable verbose output (shows each test name)
#   RACE=1          - Enable race detector during tests
#   FIX=1           - Auto-fix issues where possible (golangci only)
#
# Examples:
#   make test                           # Run unit tests
#   make test TYPE=unit TEST=TestFoo    # Run tests matching "TestFoo"
#   make test TYPE=coverage             # Generate coverage report
#   make test TYPE=bench BENCH=BenchmarkSort  # Run specific benchmark
#   make test TYPE=lint                 # Run all linters
#   make test TYPE=golangci FIX=1       # Run golangci-lint with auto-fix
#   make test TYPE=quality              # Run all security/quality checks
#   make test TYPE=all                  # Run full test suite + lint + quality
#
# Output: $(LOG_DIR)/<type>-<timestamp>.log, coverage/coverage.html (for coverage)
test: ## Run tests, linters, and quality checks. TYPE=unit|integration|e2e|coverage|bench|vet|staticcheck|golangci|shadow|lint|gosec|vulncheck|deadcode|quality|all (default: unit). PKG=<path> package to test, TEST=<pattern> filter by name, FIX=1 auto-fix (golangci only). Output: $(LOG_DIR)/<type>.log
	@mkdir -p $(LOG_DIR) $(COVERAGE_DIR)
	@TYPE=$(or $(TYPE),unit); \
	PKG=$(or $(PKG),./...); \
	TIMEOUT=$(or $(TIMEOUT),10m); \
	TEST_FLAGS="-timeout $$TIMEOUT"; \
	if [ -n "$(TEST)" ]; then TEST_FLAGS="$$TEST_FLAGS -run '$(TEST)'"; fi; \
	if [ -n "$(COUNT)" ]; then TEST_FLAGS="$$TEST_FLAGS -count $(COUNT)"; fi; \
	if [ "$(VERBOSE)" = "1" ]; then TEST_FLAGS="$$TEST_FLAGS -v"; fi; \
	if [ "$(RACE)" = "1" ]; then TEST_FLAGS="$$TEST_FLAGS -race"; fi; \
	LOGFILE="$(LOG_DIR)/$$TYPE-$$(date +%Y%m%d-%H%M%S).log"; \
	case "$$TYPE" in \
		unit) \
			echo "Running unit tests..."; \
			$(GO) test $$TEST_FLAGS -short ./cmd/... ./internal/... ./pkg/... > "$$LOGFILE" 2>&1; \
			EXIT_CODE=$$?; \
			PASSED=$$(grep -c "^ok" "$$LOGFILE" || echo 0); \
			FAILED=$$(grep -c "^FAIL" "$$LOGFILE" || echo 0); \
			if [ $$EXIT_CODE -eq 0 ]; then \
				echo "PASS: $$PASSED packages passed"; \
			else \
				echo "FAIL: $$FAILED packages failed (see log)"; \
				grep "^FAIL\|^---" "$$LOGFILE" | head -20; \
			fi; \
			echo "Log: $$LOGFILE"; \
			exit $$EXIT_CODE; \
			;; \
		integration) \
			echo "Running integration tests..."; \
			$(GO) test $$TEST_FLAGS -tags integration $$PKG > "$$LOGFILE" 2>&1; \
			EXIT_CODE=$$?; \
			PASSED=$$(grep -c "^ok" "$$LOGFILE" || echo 0); \
			if [ $$EXIT_CODE -eq 0 ]; then echo "PASS: $$PASSED packages passed"; else echo "FAIL: Tests failed"; fi; \
			echo "Log: $$LOGFILE"; \
			exit $$EXIT_CODE; \
			;; \
		e2e) \
			echo "Running e2e tests..."; \
			$(MAKE) build TYPE=binary NOBUMP=$(NOBUMP) > /dev/null 2>&1 || { echo "FAIL: Build failed"; exit 1; }; \
			$(GO) test $$TEST_FLAGS ./test/... > "$$LOGFILE" 2>&1; \
			EXIT_CODE=$$?; \
			PASSED=$$(grep -c "^ok" "$$LOGFILE" || echo 0); \
			if [ $$EXIT_CODE -eq 0 ]; then echo "PASS: $$PASSED packages passed"; else echo "FAIL: Tests failed"; cat "$$LOGFILE"; fi; \
			echo "Log: $$LOGFILE"; \
			exit $$EXIT_CODE; \
			;; \
		coverage) \
			echo "Running tests with coverage..."; \
			mkdir -p $(COVERAGE_DIR)/e2e; \
			echo "Building binary with coverage instrumentation..."; \
			$(GO) build -cover -covermode=atomic -o $(BUILD_DIR)/$(BINARY_NAME)-cov ./cmd/cli > "$$LOGFILE" 2>&1; \
			echo "Running unit tests with coverage..."; \
			$(GO) test $$TEST_FLAGS -coverprofile=$(COVERAGE_DIR)/unit.out -covermode=atomic ./cmd/... ./internal/... ./pkg/... >> "$$LOGFILE" 2>&1; \
			echo "Running e2e tests with coverage..."; \
			GOCOVERDIR=$(COVERAGE_DIR)/e2e PATH="$(shell pwd)/$(BUILD_DIR):$$PATH" \
				$(GO) test $$TEST_FLAGS ./test/... >> "$$LOGFILE" 2>&1 || true; \
			if [ -d "$(COVERAGE_DIR)/e2e" ] && [ "$$(ls -A $(COVERAGE_DIR)/e2e 2>/dev/null)" ]; then \
				echo "Merging coverage data..."; \
				$(GO) tool covdata textfmt -i=$(COVERAGE_DIR)/e2e -o=$(COVERAGE_DIR)/e2e.out 2>/dev/null || true; \
				if [ -f "$(COVERAGE_DIR)/e2e.out" ]; then \
					$(GO) tool covdata merge -i=$(COVERAGE_DIR)/e2e -o=$(COVERAGE_DIR)/merged 2>/dev/null || true; \
					$(GO) tool covdata textfmt -i=$(COVERAGE_DIR)/merged -o=$(COVERAGE_DIR)/e2e-merged.out 2>/dev/null || true; \
				fi; \
			fi; \
			if [ -f "$(COVERAGE_DIR)/e2e.out" ] && [ -f "$(COVERAGE_DIR)/unit.out" ]; then \
				echo "mode: atomic" > $(COVERAGE_DIR)/coverage.out; \
				grep -v "^mode:" $(COVERAGE_DIR)/unit.out >> $(COVERAGE_DIR)/coverage.out 2>/dev/null || true; \
				grep -v "^mode:" $(COVERAGE_DIR)/e2e.out >> $(COVERAGE_DIR)/coverage.out 2>/dev/null || true; \
			else \
				cp $(COVERAGE_DIR)/unit.out $(COVERAGE_DIR)/coverage.out 2>/dev/null || true; \
			fi; \
			$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html 2>/dev/null || true; \
			TOTAL=$$($(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out 2>/dev/null | tail -1); \
			echo "$$TOTAL"; \
			echo "Coverage report: $(COVERAGE_DIR)/coverage.html"; \
			echo "Log: $$LOGFILE"; \
			;; \
		bench) \
			echo "Running benchmarks..."; \
			BENCH_PATTERN=$(or $(BENCH),.); \
			$(GO) test $$TEST_FLAGS -bench="$$BENCH_PATTERN" -benchmem $$PKG > "$$LOGFILE" 2>&1; \
			grep "^Benchmark" "$$LOGFILE" | head -10 || echo "No benchmarks found"; \
			echo "Log: $$LOGFILE"; \
			;; \
		vet) \
			echo "Running go vet..."; \
			$(GO) vet ./... > "$$LOGFILE" 2>&1; \
			EXIT_CODE=$$?; \
			ISSUES=$$(wc -l < "$$LOGFILE" | tr -d ' '); \
			if [ $$ISSUES -eq 0 ]; then echo "PASS: No issues"; else echo "FAIL: $$ISSUES issues found"; cat "$$LOGFILE"; fi; \
			echo "Log: $$LOGFILE"; \
			exit $$EXIT_CODE; \
			;; \
		staticcheck) \
			echo "Running staticcheck..."; \
			if ! command -v staticcheck &> /dev/null; then \
				echo "Installing staticcheck..."; \
				$(GO) install honnef.co/go/tools/cmd/staticcheck@latest; \
			fi; \
			staticcheck ./... > "$$LOGFILE" 2>&1 || true; \
			ISSUES=$$(wc -l < "$$LOGFILE" | tr -d ' '); \
			if [ $$ISSUES -eq 0 ]; then echo "PASS: No issues"; else echo "$$ISSUES issues found"; cat "$$LOGFILE"; fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		golangci) \
			echo "Running golangci-lint..."; \
			if ! command -v golangci-lint &> /dev/null; then \
				echo "Installing golangci-lint..."; \
				$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
			fi; \
			FIX_FLAG=""; \
			if [ "$(FIX)" = "1" ]; then FIX_FLAG="--fix"; fi; \
			golangci-lint run $$FIX_FLAG ./... > "$$LOGFILE" 2>&1 || true; \
			ISSUES=$$(grep -c "^[^[:space:]].*:[0-9]" "$$LOGFILE" || echo 0); \
			if [ $$ISSUES -eq 0 ]; then echo "PASS: No issues"; else echo "$$ISSUES issues found"; head -20 "$$LOGFILE"; fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		shadow) \
			echo "Running shadow..."; \
			if ! command -v shadow &> /dev/null; then \
				echo "Installing shadow..."; \
				$(GO) install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest; \
			fi; \
			shadow ./... > "$$LOGFILE" 2>&1 || true; \
			ISSUES=$$(wc -l < "$$LOGFILE" | tr -d ' '); \
			if [ $$ISSUES -eq 0 ]; then echo "PASS: No shadowing issues"; else echo "$$ISSUES issues"; fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		lint) \
			echo "Running all linters..."; \
			$(GO) vet ./... > "$$LOGFILE.vet" 2>&1 || true; \
			VET_ISSUES=$$(wc -l < "$$LOGFILE.vet" | tr -d ' '); \
			if command -v staticcheck &> /dev/null; then \
				staticcheck ./... > "$$LOGFILE.staticcheck" 2>&1 || true; \
				SC_ISSUES=$$(wc -l < "$$LOGFILE.staticcheck" | tr -d ' '); \
			else SC_ISSUES=0; fi; \
			if command -v golangci-lint &> /dev/null; then \
				golangci-lint run ./... > "$$LOGFILE.golangci" 2>&1 || true; \
				GC_ISSUES=$$(grep -c "^[^[:space:]].*:[0-9]" "$$LOGFILE.golangci" || echo 0); \
			else GC_ISSUES=0; fi; \
			cat "$$LOGFILE.vet" "$$LOGFILE.staticcheck" "$$LOGFILE.golangci" > "$$LOGFILE" 2>/dev/null || true; \
			TOTAL=$$((VET_ISSUES + SC_ISSUES + GC_ISSUES)); \
			echo "vet: $$VET_ISSUES, staticcheck: $$SC_ISSUES, golangci: $$GC_ISSUES"; \
			if [ $$TOTAL -eq 0 ]; then echo "PASS: All linters passed"; else echo "Total: $$TOTAL issues"; fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		gosec) \
			echo "Running gosec..."; \
			if ! command -v gosec &> /dev/null; then \
				echo "Installing gosec..."; \
				$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest; \
			fi; \
			gosec -fmt=text -quiet ./... > "$$LOGFILE" 2>&1 || true; \
			ISSUES=$$(grep -c "^\[" "$$LOGFILE" || echo 0); \
			NOSEC=$$(grep "Nosec" "$$LOGFILE" | grep -o "[0-9]*" | tail -1 || echo 0); \
			if [ $$ISSUES -eq 0 ]; then echo "PASS: No security issues ($$NOSEC nosec)"; else echo "$$ISSUES security issues found"; fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		vulncheck) \
			echo "Running govulncheck..."; \
			if ! command -v govulncheck &> /dev/null; then \
				echo "Installing govulncheck..."; \
				$(GO) install golang.org/x/vuln/cmd/govulncheck@latest; \
			fi; \
			govulncheck ./... > "$$LOGFILE" 2>&1 || true; \
			if grep -q "No vulnerabilities found" "$$LOGFILE"; then \
				echo "PASS: No vulnerabilities found"; \
			else \
				VULNS=$$(grep -c "^Vulnerability" "$$LOGFILE" || echo 0); \
				echo "$$VULNS vulnerabilities found"; \
			fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		deadcode) \
			echo "Running deadcode..."; \
			if ! command -v deadcode &> /dev/null; then \
				echo "Installing deadcode..."; \
				$(GO) install golang.org/x/tools/cmd/deadcode@latest; \
			fi; \
			deadcode ./... > "$$LOGFILE" 2>&1 || true; \
			ISSUES=$$(wc -l < "$$LOGFILE" | tr -d ' '); \
			if [ $$ISSUES -eq 0 ]; then echo "PASS: No dead code"; else echo "$$ISSUES unreachable functions"; fi; \
			echo "Log: $$LOGFILE"; \
			;; \
		quality) \
			echo "Running quality checks..."; \
			GOSEC_LOG="$$LOGFILE.gosec"; VULN_LOG="$$LOGFILE.vuln"; DEAD_LOG="$$LOGFILE.dead"; \
			if command -v gosec &> /dev/null; then \
				gosec -fmt=text -quiet ./... > "$$GOSEC_LOG" 2>&1 || true; \
				GOSEC_N=$$(grep -c "^\[" "$$GOSEC_LOG" || echo 0); \
			else GOSEC_N="n/a"; fi; \
			if command -v govulncheck &> /dev/null; then \
				govulncheck ./... > "$$VULN_LOG" 2>&1 || true; \
				if grep -q "No vulnerabilities" "$$VULN_LOG"; then VULN_N=0; else VULN_N=$$(grep -c "^Vulnerability" "$$VULN_LOG" || echo 0); fi; \
			else VULN_N="n/a"; fi; \
			if command -v deadcode &> /dev/null; then \
				deadcode ./... > "$$DEAD_LOG" 2>&1 || true; \
				DEAD_N=$$(wc -l < "$$DEAD_LOG" | tr -d ' '); \
			else DEAD_N="n/a"; fi; \
			cat "$$GOSEC_LOG" "$$VULN_LOG" "$$DEAD_LOG" > "$$LOGFILE" 2>/dev/null || true; \
			echo "gosec: $$GOSEC_N, vulncheck: $$VULN_N, deadcode: $$DEAD_N"; \
			echo "Log: $$LOGFILE"; \
			;; \
		emoji) \
			echo "Running emoji check..."; \
			$(MAKE) emoji-check; \
			;; \
		all) \
			echo "Running full test suite..."; \
			FAILED=0; \
			echo "=== Build ==="; \
			$(MAKE) build TYPE=binary NOBUMP=$(NOBUMP) || FAILED=1; \
			echo "=== Unit Tests ==="; \
			$(GO) test $$TEST_FLAGS -short ./cmd/... ./internal/... ./pkg/... > "$$LOGFILE.unit" 2>&1 || FAILED=1; \
			if [ "$$CI" = "true" ]; then cat "$$LOGFILE.unit"; fi; \
			UNIT_PASS=$$(grep -c "^ok" "$$LOGFILE.unit" 2>/dev/null | tr -d '\n' || echo 0); \
			UNIT_FAIL=$$(grep -c "^FAIL" "$$LOGFILE.unit" 2>/dev/null | tr -d '\n' || echo 0); \
			if [ "$${UNIT_FAIL:-0}" -eq 0 ]; then echo "PASS: $$UNIT_PASS packages passed"; else echo "FAIL: $$UNIT_FAIL packages failed"; fi; \
			echo "=== E2E Tests ==="; \
			$(GO) test $$TEST_FLAGS ./test/... > "$$LOGFILE.e2e" 2>&1 || FAILED=1; \
			if [ "$$CI" = "true" ]; then cat "$$LOGFILE.e2e"; fi; \
			E2E_PASS=$$(grep -c "^ok" "$$LOGFILE.e2e" 2>/dev/null | tr -d '\n' || echo 0); \
			E2E_FAIL=$$(grep -c "^FAIL" "$$LOGFILE.e2e" 2>/dev/null | tr -d '\n' || echo 0); \
			if [ "$${E2E_FAIL:-0}" -eq 0 ]; then echo "PASS: $$E2E_PASS packages passed"; else echo "FAIL: $$E2E_FAIL packages failed"; fi; \
			echo "=== Coverage ==="; \
			$(GO) test $$TEST_FLAGS -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./cmd/... ./internal/... ./pkg/... > "$$LOGFILE.cov" 2>&1 || true; \
			if [ "$$CI" = "true" ]; then cat "$$LOGFILE.cov"; fi; \
			COV=$$($(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out 2>/dev/null | tail -1 | awk '{print $$3}'); \
			echo "Coverage: $$COV"; \
			echo "=== Vet ==="; \
			$(GO) vet ./... > "$$LOGFILE.vet" 2>&1 || FAILED=1; \
			if [ "$$CI" = "true" ] && [ -s "$$LOGFILE.vet" ]; then cat "$$LOGFILE.vet"; fi; \
			VET_N=$$(wc -l < "$$LOGFILE.vet" | tr -d ' '); \
			if [ $$VET_N -eq 0 ]; then echo "PASS: No issues"; else echo "FAIL: $$VET_N issues"; fi; \
			echo "=== Lint ==="; \
			LINT_TOTAL=0; SC_N=0; GC_N=0; \
			if command -v staticcheck &> /dev/null; then \
				staticcheck ./... > "$$LOGFILE.staticcheck" 2>&1 || true; \
				SC_N=$$(wc -l < "$$LOGFILE.staticcheck" | tr -d ' \n'); \
				LINT_TOTAL=$$((LINT_TOTAL + SC_N)); \
			fi; \
			if command -v golangci-lint &> /dev/null; then \
				golangci-lint run ./... > "$$LOGFILE.golangci" 2>&1 || true; \
				GC_N=$$(grep -c "^[^[:space:]].*:[0-9]" "$$LOGFILE.golangci" 2>/dev/null); GC_N=$${GC_N:-0}; \
				LINT_TOTAL=$$((LINT_TOTAL + GC_N)); \
			fi; \
			echo "staticcheck: $$SC_N, golangci: $$GC_N"; \
			if [ $$LINT_TOTAL -eq 0 ]; then echo "PASS: No lint issues"; else echo "WARN: $$LINT_TOTAL lint issues"; fi; \
			echo "=== Security ==="; \
			SECURITY_ISSUES=0; GOSEC_N=0; VULN_N=0; \
			if command -v gosec &> /dev/null; then \
				gosec -fmt=text -quiet ./... > "$$LOGFILE.gosec" 2>&1 || true; \
				GOSEC_N=$$(grep -c "^\[" "$$LOGFILE.gosec" 2>/dev/null); GOSEC_N=$${GOSEC_N:-0}; \
				SECURITY_ISSUES=$$((SECURITY_ISSUES + GOSEC_N)); \
			fi; \
			if command -v govulncheck &> /dev/null; then \
				govulncheck ./... > "$$LOGFILE.vuln" 2>&1 || true; \
				if grep -q "No vulnerabilities" "$$LOGFILE.vuln"; then VULN_N=0; else VULN_N=$$(grep -c "^Vulnerability" "$$LOGFILE.vuln" 2>/dev/null); VULN_N=$${VULN_N:-0}; fi; \
				SECURITY_ISSUES=$$((SECURITY_ISSUES + VULN_N)); \
			fi; \
			echo "gosec: $$GOSEC_N, vulncheck: $$VULN_N"; \
			if [ $$SECURITY_ISSUES -eq 0 ]; then echo "PASS: No security issues"; else echo "WARN: $$SECURITY_ISSUES security issues"; fi; \
			echo "=== Quality ==="; \
			DEAD_N=0; \
			if command -v deadcode &> /dev/null; then \
				deadcode ./... > "$$LOGFILE.deadcode" 2>&1 || true; \
				DEAD_N=$$(wc -l < "$$LOGFILE.deadcode" | tr -d ' \n'); \
			fi; \
			echo "deadcode: $$DEAD_N"; \
			if [ $$DEAD_N -eq 0 ]; then echo "PASS: No dead code"; else echo "INFO: $$DEAD_N unreachable functions"; fi; \
			cat "$$LOGFILE.unit" "$$LOGFILE.e2e" "$$LOGFILE.cov" "$$LOGFILE.vet" "$$LOGFILE.staticcheck" "$$LOGFILE.golangci" "$$LOGFILE.gosec" "$$LOGFILE.vuln" "$$LOGFILE.deadcode" > "$$LOGFILE" 2>/dev/null || true; \
			echo ""; \
			if [ $$FAILED -eq 1 ]; then echo "FAIL: Some checks failed"; exit 1; else echo "PASS: All checks passed"; fi; \
			;; \
		*) \
			echo "Unknown test type: $$TYPE"; \
			echo "Valid types: unit, integration, e2e, coverage, bench, vet, staticcheck, golangci, shadow, lint, gosec, vulncheck, deadcode, quality, emoji, all"; \
			exit 1; \
			;; \
	esac

# =============================================================================
# FORMAT
# =============================================================================

# Auto-format Go source code to conform to standard Go style.
# Uses gofmt (built-in) and goimports (if available) to format code
# and organize imports according to Go conventions.
#
# This target modifies files in-place. Run before committing code.
#
# Tools used:
#   - gofmt: Standard Go formatter (always available)
#   - goimports: Formats code AND manages import statements (optional but recommended)
#
# Examples:
#   make fmt    # Format all Go code in the project
#
# No variables - always formats all code in the project.
# Safe to run anytime - idempotent operation.
fmt: ## Format Go code with gofmt and goimports. Modifies files in-place. Uses gofmt (built-in) for standard formatting and goimports (if installed) to organize imports. Safe to run anytime - idempotent. Run before committing code.
	@echo "Formatting code..."
	@$(GO) fmt ./...
	@if command -v goimports &> /dev/null; then \
		echo "Running goimports..."; \
		goimports -w .; \
	else \
		echo "goimports not installed (run: go install golang.org/x/tools/cmd/goimports@latest)"; \
	fi
	@echo "PASS: Code formatted"

# =============================================================================
# VERIFY & DEPS
# =============================================================================

# Verify that module dependencies are correct and consistent.
# Runs go mod verify to check that dependencies haven't been tampered with,
# then runs go mod tidy to clean up go.mod and go.sum.
# Finally checks if there are uncommitted changes to go.mod/go.sum.
#
# This is useful in CI to ensure reproducible builds and detect
# when someone forgot to run 'go mod tidy' before committing.
#
# Examples:
#   make verify    # Verify deps are correct and tidy
#
# Exit code: Non-zero if verification fails or go.mod/go.sum changed.
verify: ## Verify module dependencies are correct and tidy. Runs go mod verify to check deps haven't been tampered with, go mod tidy to clean up, then checks for uncommitted go.mod/go.sum changes. Useful in CI to ensure reproducible builds. Exit code non-zero if verification fails.
	@echo "Verifying dependencies..."
	@$(GO) mod verify
	@$(GO) mod tidy
	@echo "Checking for uncommitted go.mod/go.sum changes..."
	@git diff --exit-code go.mod go.sum || (echo "go.mod or go.sum has uncommitted changes"; exit 1)
	@echo "PASS: Dependencies verified"

# Download all module dependencies and verify their checksums.
# This is useful to pre-populate the module cache before building,
# especially in CI environments or Docker builds.
#
# Examples:
#   make deps    # Download and verify all dependencies
#
# This target is idempotent - safe to run multiple times.
deps: ## Download and verify all module dependencies. Pre-populates module cache before building. Useful in CI/Docker builds. Idempotent - safe to run multiple times.
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod verify
	@echo "PASS: Dependencies ready"

# =============================================================================
# CLEAN
# =============================================================================

# Clean up build artifacts, test caches, and generated files.
# Supports selective cleaning to avoid removing everything unnecessarily.
#
# Variables:
#   TYPE=build|test|all  - What to clean (default: build)
#     build - Remove compiled binaries from bin/ and run go clean
#     test  - Remove coverage reports, test logs, and clear test cache
#     all   - Remove everything: binaries, coverage, logs, and all caches
#
# Examples:
#   make clean              # Clean build artifacts only
#   make clean TYPE=test    # Clean test artifacts and cache
#   make clean TYPE=all     # Nuclear option - clean everything
#
# Safe to run anytime - idempotent operations.
clean: ## Clean build artifacts and caches. Variables: TYPE=build|test|all (default: build). build=remove bin/ and run go clean, test=remove coverage/ and logs, all=remove everything. Examples: make clean, make clean TYPE=test, make clean TYPE=all. Safe to run anytime - idempotent.
	@TYPE=$(or $(TYPE),build); \
	case "$$TYPE" in \
		build) \
			echo "Cleaning build artifacts..."; \
			rm -rf $(BUILD_DIR); \
			$(GO) clean; \
			echo "PASS: Build artifacts cleaned"; \
			;; \
		test) \
			echo "Cleaning test artifacts..."; \
			rm -rf $(COVERAGE_DIR); \
			rm -rf $(LOG_DIR); \
			$(GO) clean -testcache; \
			echo "PASS: Test artifacts cleaned"; \
			;; \
		all) \
			echo "Cleaning all artifacts..."; \
			rm -rf $(BUILD_DIR); \
			rm -rf $(COVERAGE_DIR); \
			rm -rf $(LOG_DIR); \
			$(GO) clean -cache -testcache; \
			echo "PASS: All artifacts cleaned"; \
			;; \
		*) \
			echo "Unknown clean type: $$TYPE"; \
			echo "Valid types: build, test, all"; \
			exit 1; \
			;; \
	esac

# =============================================================================
# INSTALL GO
# =============================================================================

GO_VERSION := 1.25.5
GO_INSTALL_DIR := /usr/local

install-go: ## Install Go $(GO_VERSION) from go.dev/dl
	@echo "Installing Go $(GO_VERSION)..."
	@if command -v go &> /dev/null && go version | grep -q "go$(GO_VERSION)"; then \
		echo "Go $(GO_VERSION) is already installed"; \
		go version; \
		exit 0; \
	fi
	@echo "Downloading Go $(GO_VERSION)..."
	@curl -fsSL "https://go.dev/dl/go$(GO_VERSION).linux-amd64.tar.gz" -o /tmp/go$(GO_VERSION).tar.gz
	@echo "Removing old Go installation (if any)..."
	@sudo rm -rf $(GO_INSTALL_DIR)/go
	@echo "Extracting to $(GO_INSTALL_DIR)..."
	@sudo tar -C $(GO_INSTALL_DIR) -xzf /tmp/go$(GO_VERSION).tar.gz
	@rm /tmp/go$(GO_VERSION).tar.gz
	@echo ""
	@echo "PASS: Go $(GO_VERSION) installed to $(GO_INSTALL_DIR)/go"
	@echo ""
	@echo "Add to your PATH if not already present:"
	@echo "  export PATH=\$$PATH:$(GO_INSTALL_DIR)/go/bin"
	@echo ""
	@$(GO_INSTALL_DIR)/go/bin/go version
