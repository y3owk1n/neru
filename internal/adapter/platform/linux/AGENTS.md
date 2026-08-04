# Linux backends — native-boundary contracts

Read `../AGENTS.md` first (slots, factory, stubs, generated `wlr_protocol/`). This file covers what it omits. These rules are scattered across file-head comments; violating them fails at link or run time, not compile time.

- **Backend selection is runtime, not build-tag**: KDE and wlroots are identical at compile time (`linux && wayland`); dispatch on `platform.DetectLinuxBackend`, never add a compositor build tag.
- **Every cgo symbol has a `_nocgo.go` twin** returning `derrors.CodeNotSupported` ("requires CGO-enabled Linux builds"); pinned by `system_stub_contract_test.go` — a supported capability must never answer `CodeNotSupported` or return nil.
- **The `.c` files compile in exactly one unit** (`cgo.go`). Packages that call bridge symbols must blank-import `platform/linux` — and `wlr_protocol` separately — or the linker fails with undefined symbols.
- **`doc.go` is tagged `linux`, not `linux && cgo` — deliberately.** The `.c` files already gate cgo builds; the broader tag keeps analysis working on other hosts. Don't "fix" it.
- **C→Go signaling is fd-based** (self-pipe from a native thread); there are zero `//export` callbacks here by design. The monitor owns its fd: treat it read-only, never close it; monitors run for the daemon's lifetime.
- **Never block on the eventtap goroutine.** Mid-action native calls (e.g. libei lazy connect) run on the goroutine holding the keyboard grab — a slow call freezes global hotkeys. Timeouts stay short; session establishment belongs to warm-up.
- **Capability probes**: one capped goroutine per probe, never restarted while wedged — native calls can't be canceled, and probes are reached from long-lived IPC handlers. Per-feature isolation is pinned by `capability_probes_test.go`.
- **uinput/evdev**: writes to the device fd must be contiguous (never interleave with a concurrent feed); releasing our modifier must not clear one the user physically holds.
- Naming: `neru_<subsystem>_*` snake_case, one `.c`/`.h` pair per subsystem, `#cgo pkg-config` on the consuming Go file.
