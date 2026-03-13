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

# Uses CGO on macOS (required for Objective-C bridge), CGO_ENABLED=0 elsewhere
build:
    @echo "Building Neru..."
    @echo "Version: {{ VERSION }}"
    {{ if os() == "macos" { "CGO_ENABLED=1" } else { "CGO_ENABLED=0" } }} go build -ldflags="{{ LDFLAGS }}" -o bin/neru{{ if os() == "windows" { ".exe" } else { "" } }} ./cmd/neru
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

# Build release artifacts for CI (cross-platform)
release-ci VERSION_OVERRIDE:
    @echo "Building release artifacts for CI..."
    @echo "Version: {{ VERSION_OVERRIDE }}"
    @echo "Commit: {{ GIT_COMMIT }}"
    @echo "Date: {{ BUILD_DATE }}"
    mkdir -p bin
    @echo "Building darwin-arm64..."
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-darwin-arm64 ./cmd/neru
    @echo "Building darwin-amd64..."
    CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-darwin-amd64 ./cmd/neru
    @echo "Building linux-amd64..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-linux-amd64 ./cmd/neru
    @echo "Building windows-amd64..."
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-windows-amd64.exe ./cmd/neru
    @echo "Building windows-arm64..."
    CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-windows-arm64.exe ./cmd/neru
    @echo "Building linux-arm64..."
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version={{ VERSION_OVERRIDE }} -X github.com/y3owk1n/neru/internal/cli.GitCommit={{ GIT_COMMIT }} -X github.com/y3owk1n/neru/internal/cli.BuildDate={{ BUILD_DATE }}" -trimpath -o bin/neru-linux-arm64 ./cmd/neru
    @echo "✓ Release artifacts built successfully"

# Bundle the application
bundle: release
    @echo "Bundling Neru..."
    mkdir -p build/Neru.app/Contents/{MacOS,Resources}

    cp -r bin/neru build/Neru.app/Contents/MacOS/Neru

    cp resources/icon.icns build/Neru.app/Contents/Resources/icon.icns

    sed "s/VERSION/{{ VERSION }}/g" resources/Info.plist.template > build/Neru.app/Contents/Info.plist

    codesign --force --deep --sign - build/Neru.app

    @echo "✓ Bundle complete: build/Neru.app"

# Run tests

# Run all tests (unit + integration)
test: test-unit test-integration
    @echo "Running all tests..."

# Run unit tests
test-unit:
    @echo "Running unit tests..."
    go test -v ./...

# Run integration tests
test-integration:
    @echo "Running integration tests..."
    go test -tags=integration -v ./...

# Run with race detection
test-race: test-race-unit test-race-integration
    @echo "Running tests with race detection..."

# Run unit tests with race detection
test-race-unit:
    @echo "Running unit tests with race detection..."
    go test -race -v ./...

# Run integration tests with race detection
test-race-integration:
    @echo "Running integration tests with race detection..."
    go test -tags=integration -race -v ./...

test-all: test test-race

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
    done < <(find internal/core/infra \( -name "*.h" -o -name "*.m" \) -print0)
    if [ $EXIT_CODE -ne 0 ]; then
        echo "Some Objective-C files are not properly formatted. Run 'just fmt' to fix them."
        exit 1
    fi
    echo "✓ All Objective-C files are properly formatted"

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
    @find internal/core/infra \( -name "*.h" -o -name "*.m" \) -exec clang-format -i --style=file --assume-filename=file.m {} \;
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

# Generate icon.icns from a source PNG (e.g., just generate-icns icon-1024.png)
generate-icns SOURCE:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Generating icon.icns from {{ SOURCE }}..."
    ICONSET="icon.iconset"
    mkdir -p "$ICONSET"
    sips -z 16 16     "{{ SOURCE }}" --out "$ICONSET/icon_16x16.png"      >/dev/null
    sips -z 32 32     "{{ SOURCE }}" --out "$ICONSET/icon_16x16@2x.png"   >/dev/null
    sips -z 32 32     "{{ SOURCE }}" --out "$ICONSET/icon_32x32.png"      >/dev/null
    sips -z 64 64     "{{ SOURCE }}" --out "$ICONSET/icon_32x32@2x.png"   >/dev/null
    sips -z 128 128   "{{ SOURCE }}" --out "$ICONSET/icon_128x128.png"    >/dev/null
    sips -z 256 256   "{{ SOURCE }}" --out "$ICONSET/icon_128x128@2x.png" >/dev/null
    sips -z 256 256   "{{ SOURCE }}" --out "$ICONSET/icon_256x256.png"    >/dev/null
    sips -z 512 512   "{{ SOURCE }}" --out "$ICONSET/icon_256x256@2x.png" >/dev/null
    sips -z 512 512   "{{ SOURCE }}" --out "$ICONSET/icon_512x512.png"    >/dev/null
    sips -z 1024 1024 "{{ SOURCE }}" --out "$ICONSET/icon_512x512@2x.png" >/dev/null
    iconutil -c icns "$ICONSET" -o resources/icon.icns
    rm -rf "$ICONSET"
    echo "✓ Generated resources/icon.icns"

# Generate systray tray icon PNGs from source PNGs
# Resizes to 44×44 pixels (22pt @2x retina for macOS menu bar)

# Usage: just generate-tray-icons active.png disabled.png
generate-tray-icons ACTIVE DISABLED:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Generating tray icons..."
    TRAY_DIR="internal/app/components/systray/resources"
    mkdir -p "$TRAY_DIR"
    sips -z 44 44 "{{ ACTIVE }}"   --out "$TRAY_DIR/tray-icon.png"          >/dev/null
    sips -z 44 44 "{{ DISABLED }}" --out "$TRAY_DIR/tray-icon-disabled.png"  >/dev/null
    echo "✓ Generated $TRAY_DIR/tray-icon.png (44×44, 22pt @2x)"
    echo "✓ Generated $TRAY_DIR/tray-icon-disabled.png (44×44, 22pt @2x)"

# Generate all icons from source PNGs

# Usage: just generate-icons app-icon.png tray-active.png tray-disabled.png
generate-icons APP_ICON TRAY_ACTIVE TRAY_DISABLED:
    just generate-icns {{ APP_ICON }}
    just generate-tray-icons {{ TRAY_ACTIVE }} {{ TRAY_DISABLED }}
    @echo "✓ All icons generated"
