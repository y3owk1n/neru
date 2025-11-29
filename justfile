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

# Run all tests (unit + integration)
test:
    @echo "Running all tests..."
    just test-unit
    just test-integration

# Run unit tests
test-unit:
    @echo "Running unit tests..."
    go test -tags=unit -v ./...

# Run with race detection
test-race:
    @echo "Running tests with race detection..."
    just test-race-unit
    just test-race-integration

# Run unit tests with race detection
test-race-unit:
    @echo "Running unit tests with race detection..."
    go test -tags=unit -race -v ./...

# Run integration tests with race detection
test-race-integration:
    @echo "Running integration tests with race detection..."
    go test -tags=integration -race -v ./...

# Run integration tests
test-integration:
    @echo "Running integration tests..."
    go test -tags=integration -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -tags=unit -coverprofile=coverage-unit.out ./...
	go test -tags=integration -coverprofile=coverage-integration.out ./...
	@head -1 coverage-unit.out > coverage.txt
	@tail -n +2 coverage-unit.out >> coverage.txt
	@tail -n +2 coverage-integration.out >> coverage.txt

test-coverage-html:
    @echo "Running tests with coverage (HTML)..."
    just test-coverage
    go tool cover -html=coverage.txt -o coverage.html

test-all: test-unit test-race-unit test-race-integration test-integration test-coverage

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
    done < <(find internal/core/infra/bridge \( -name "*.h" -o -name "*.m" \) -print0)
    if [ $EXIT_CODE -ne 0 ]; then
        echo "Some Objective-C files are not properly formatted. Run 'just fmt' to fix them."
        exit 1
    fi
    echo "✓ All Objective-C files are properly formatted"

# Run benchmarks
bench:
    @echo "Running all benchmarks..."
    just bench-unit
    just bench-integration

# Run unit benchmarks
bench-unit:
    @echo "Running unit benchmarks..."
    go test -tags=unit -bench=. -benchmem ./...

# Run integration benchmarks
bench-integration:
    @echo "Running integration benchmarks..."
    go test -tags=integration -bench=. -benchmem ./...

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
    golangci-lint run --fix
    @echo "Formatting Objective-C files..."
    @find internal/core/infra/bridge \( -name "*.h" -o -name "*.m" \) -exec clang-format -i --style=file --assume-filename=file.m {} \;
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
