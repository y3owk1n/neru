# Neru Build System
# Version information (can be overridden)

VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
GIT_COMMIT := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
BUILD_DATE := `date -u +"%Y-%m-%dT%H:%M:%SZ"`

# Ldflags for version injection

LDFLAGS := "-s -w -X github.com/y3owk1n/neru/internal/cli.Version=" + VERSION + " -X github.com/y3owk1n/neru/internal/cli.GitCommit=" + GIT_COMMIT + " -X github.com/y3owk1n/neru/internal/cli.BuildDate=" + BUILD_DATE

# Default build
default: build

# Build the application (development)
build:
    @echo "Building Neru..."
    @echo "Version: {{ VERSION }}"
    CGO_ENABLED=1 go build -ldflags="{{ LDFLAGS }}" -o bin/neru ./cmd/neru
    @echo "✓ Build complete: bin/neru"

# Build with optimizations for release
release:
    @echo "Building release version..."
    @echo "Version: {{ VERSION }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    CGO_ENABLED=1 go build -ldflags="{{ LDFLAGS }}" -trimpath -o bin/neru ./cmd/neru
    @echo "✓ Release build complete: bin/neru"

# Build with custom version
build-version VERSION_OVERRIDE:
    @echo "Building Neru with custom version..."
    CGO_ENABLED=1 go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru ./cmd/neru
    @echo "✓ Build complete: bin/neru (version: {{ VERSION_OVERRIDE }})"

# Bundle the application
bundle: release
    @echo "Bundling Neru..."
    mkdir -p build/Neru.app/Contents/{MacOS,Resources}

    cp -r bin/neru build/Neru.app/Contents/MacOS/Neru

    sed "s/VERSION/{{ VERSION }}/g" resources/Info.plist.template > build/Neru.app/Contents/Info.plist

    @echo "✓ Bundle complete: build/Neru.app"

# Run tests
test:
    @echo "Running tests..."
    go test -v ./...

# Run with race detection
test-race:
    @echo "Running tests with race detection..."
    go test -race -v ./...

test-integration:
    @echo "Running integration tests..."
    @echo "Note: Only runs tests tagged with //go:build integration"
    go test -tags=integration -v ./...

test-coverage:
    @echo "Running tests with coverage..."
    go test -coverprofile=coverage.out -covermode=atomic ./...

test-coverage-html:
    @echo "Running tests with coverage (HTML)..."
    go test -coverprofile=coverage.out -covermode=atomic ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report generated: coverage.html"

test-coverage-summary:
    @echo "Running tests with coverage summary..."
    go test -coverprofile=coverage.out -covermode=atomic ./...
    go tool cover -func=coverage.out | tail -1

test-coverage-detailed:
    @echo "Running tests with detailed coverage analysis..."
    go test -coverprofile=coverage.out -covermode=atomic ./...
    @echo "=== Coverage by Function ==="
    go tool cover -func=coverage.out
    @echo ""
    @echo "=== Coverage by Package ==="
    go tool cover -func=coverage.out | grep -E "^[^/]+/" | awk '{print $1, $3}' | sort -k2 -nr
    @echo ""
    @echo "=== Uncovered Functions ==="
    go tool cover -func=coverage.out | grep "0.0%"

test-flaky-check:
    @echo "Running tests multiple times to check for flakiness..."
    @for i in 1 2 3; do \
        echo "Run $$i:"; \
        go test -v ./... 2>&1 | grep -E "(FAIL|PASS|SKIP)" | head -1; \
    done

test-quality-check:
    @echo "Running test quality checks..."
    @echo "=== Test Files Count ==="
    @find . -name "*_test.go" -type f | wc -l
    @echo ""
    @echo "=== Benchmark Count ==="
    @grep -r "func Benchmark" --include="*_test.go" . | wc -l
    @echo ""
    @echo "=== Fuzz Test Count ==="
    @grep -r "func Fuzz" --include="*_test.go" . | wc -l
    @echo ""
    @echo "=== Integration Test Count ==="
    @find . -name "*integration*test.go" -type f | wc -l

# Complete test suite with detailed analysis (slower, more comprehensive)
test-full-suite: test test-race test-integration test-coverage-detailed test-quality-check
    @echo "🎉 Full test suite completed successfully!"

test-race-integration:
    @echo "Running integration tests with race detection..."
    go test -race -tags=integration -v ./...



# Check if files are formatted correctly
fmt-check:
    #!/usr/bin/env bash
    echo "Not checking formatting for go files... It will be checked in lint"
    echo "Checking Objective-C file formatting..."
    EXIT_CODE=0
    while IFS= read -r -d '' file; do
        OUTPUT=$(clang-format --dry-run -Werror --style=file --assume-filename=file.m "$file" 2>&1)
        RESULT=$?
        # Filter out the "does not support C++" warnings
        FILTERED=$(echo "$OUTPUT" | grep -v "Configuration file(s) do(es) not support C++")
        if [ -n "$FILTERED" ]; then
            echo "$FILTERED"
        fi
        if [ $RESULT -ne 0 ] && [ -n "$FILTERED" ]; then
            EXIT_CODE=1
        fi
    done < <(find internal/infra/bridge \( -name "*.h" -o -name "*.m" \) -print0)
    if [ $EXIT_CODE -ne 0 ]; then
        echo "Some Objective-C files are not properly formatted. Run 'just fmt' to fix them."
        exit 1
    fi
    echo "✓ All Objective-C files are properly formatted"

# Run benchmarks
bench:
    @echo "Running benchmarks..."
    go test -bench=. -benchmem ./...

# Clean build artifacts
clean:
    @echo "Cleaning build artifacts..."
    rm -rf bin/
    rm -rf build/
    rm -rf *.app
    @echo "✓ Clean complete"

# Format code
fmt:
    @echo "Formatting Go files..."
    golangci-lint fmt
    @echo "Formatting Objective-C files..."
    @find internal/infra/bridge \( -name "*.h" -o -name "*.m" \) -exec clang-format -i --style=file --assume-filename=file.m {} \;
    @echo "✓ Format complete"

# Lint code
lint:
    @echo "Linting code..."
    golangci-lint run
    @echo "Linting Objective-C files..."
    echo "Skipping Objective-C linting due to header issues"
    @echo "✓ Lint complete"

# Vet
vet:
    @echo "Vetting code..."
    go vet ./...
    @echo "✓ Vet complete"

# Download dependencies
deps:
    @echo "Downloading dependencies..."
    go mod download
    go mod tidy
    @echo "✓ Dependencies updated"

# Verify dependencies
verify:
    @echo "Verifying dependencies..."
    go mod verify
    @echo "✓ Dependencies verified"
