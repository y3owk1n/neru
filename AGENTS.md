# AGENTS.md - Neru Development Guide

Neru is a keyboard-driven navigation tool for macOS built with Go and Objective-C.

## Domain Concepts

- **Mode**: Navigation context (hints, grid, scroll, action)
- **Bridge**: Objective-C macOS integration layer
- **Adapter**: Port implementation for external systems

## Documentation

Documentation is progressively disclosed. Start here, then navigate to detailed docs:

- [Development Guide](./docs/DEVELOPMENT.md) - Build, testing, architecture
- [Coding Standards](./docs/CODING_STANDARDS.md) - Go & Objective-C conventions
- [CLI Usage](./docs/CLI.md) - Command-line interface
- [Configuration](./docs/CONFIGURATION.md) - Configuration reference

## Resources

- [Go](https://golang.org/doc/) | [Just](https://github.com/casey/just) | [Cobra](https://github.com/spf13/cobra)

> **Tip**: Docs may become outdated. When in doubt, read the code directly.
