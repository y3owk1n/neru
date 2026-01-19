---
name: lint
description: Format and lint the Neru codebase
license: MIT
compatibility: opencode
metadata:
  audience: developers
  workflow: ci
---

## What I do

Format and lint the Neru codebase following project conventions.

## Commands

- `just fmt` - Format code (Go + Objective-C)
- `just fmt-check` - Check formatting only
- `just lint` - Run linters
- `just vet` - Vet code
- `just deps` - Download and tidy dependencies
- `just verify` - Verify dependencies

## When to use me

Use this skill before committing code to ensure it meets project standards.
