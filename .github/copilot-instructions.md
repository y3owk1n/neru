# Copilot instructions

The full cross-agent project contract lives in [AGENTS.md](../AGENTS.md) at the
repository root (newer Copilot versions read it natively). Follow it — in
particular the One Rule (non-darwin code never imports
`internal/adapter/platform/darwin`), the coordinate convention (global
top-left origin, Y down), `derrors.CodeNotSupported` for platform stubs, and
the `just fmt && just lint && just test && just build` pre-commit gate.
