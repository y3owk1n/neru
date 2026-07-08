<div align="center">

<img src="assets/neru-appicon.png" alt="Neru Logo" width="80" />

# Neru

**Navigate your entire screen without touching the mouse.**

_Built for the person who already remapped Caps Lock._

![Go Version](https://img.shields.io/github/go-mod/go-version/y3owk1n/neru?style=flat-square&logo=go)
[![Latest Release](https://img.shields.io/github/v/release/y3owk1n/neru?style=flat-square)](https://github.com/y3owk1n/neru/releases)
[![Discord](https://img.shields.io/discord/1502261043701874698?style=flat-square)](https://discord.gg/KZwnwr9dz6)
[![License](https://img.shields.io/github/license/y3owk1n/neru?style=flat-square)](LICENSE)

**Free and open-source.** No subscription. No trial. No catch.

|      macOS       |     Linux     |    Windows    |
| :--------------: | :-----------: | :-----------: |
| Full featured ✅ | Core modes ✅ | Core modes ✅ |

**練る** _(neru)_ — "to refine through practice." The more you use it, the faster you get.

</div>

---

If you use Vimium in the browser, you already know the feeling. **Neru brings keyboard-driven navigation to your entire operating system** — every app, every window, every menu bar item.

```
Cmd+Shift+Space  →  labels appear on every clickable element
type the label   →  cursor jumps there
Shift+L          →  left click
```

The mouse is now optional.

Unlike most tools in this space, Neru is **CLI-first** — every action is a shell command, every setting is a plain TOML file, everything is scriptable. No GUI preferences window. It lives in your dotfiles where it belongs.

---

## Quick Start

### 1. Install

```bash
brew tap y3owk1n/tap
brew install --cask y3owk1n/tap/neru
```

Other methods: [Homebrew nightly](docs/INSTALLATION.md#homebrew-nightly), [Nix flake](docs/INSTALLATION.md#nix-flake), [build from source](docs/INSTALLATION.md#build-from-source).

### 2. Grant Permissions

Open **System Settings → Privacy & Security → Accessibility**, click **+**, and add `/Applications/Neru.app`.

> Neru needs Accessibility access to read UI elements, simulate clicks, and capture keyboard events. All processing stays local — no data leaves your machine. See [Security](SECURITY.md).

### 3. Start the Daemon

```bash
neru launch
neru status      # Should show "running"
```

### 4. Use It

```
Cmd+Shift+Space        → labels appear on every clickable element
as                     → cursor jumps to element labelled "as"
l                      → left-click
Shift+L                → right-click
Escape                 → exit hints mode
```

### 5. Make It Auto-Start

```bash
neru services install
```

---

## Pick Your Mode

<table>
<tr>
<td align="center" width="33%">
<img src="https://github.com/user-attachments/assets/0d46fa7c-345a-45ee-ad44-7a601c2b7cb1" alt="Recursive Grid Mode" /><br/>
<sub><b>Recursive Grid</b> · start here</sub>
</td>
<td align="center" width="33%">
<img src="https://github.com/user-attachments/assets/d452f972-ce23-4798-955b-6dbfa8435504" alt="Grid Mode" /><br/>
<sub><b>Grid</b></sub>
</td>
<td align="center" width="33%">
<img src="https://github.com/user-attachments/assets/71b13850-1b87-40b5-9ac0-93cff1f2e89b" alt="Hints Mode" /><br/>
<sub><b>Hints (AX + Vision OCR)</b></sub>
</td>
</tr>
</table>

| Mode                  | Hotkey            | How It Works                                            | Best For                                         |
| :-------------------- | :---------------- | :------------------------------------------------------ | :----------------------------------------------- |
| **Recursive Grid** ⭐ | `Cmd+Shift+C`     | Divides screen into cells; narrow with home-row keys    | Everything — any app, canvas, or game            |
| **Hints**             | `Cmd+Shift+Space` | Labels every clickable element via AX API or Vision OCR | Native apps, Electron (VS Code, Slack), Figma    |
| **Grid**              | `Cmd+Shift+G`     | Jump to a cell by row + column label                    | Fast coarse movement across large monitors       |
| **Scroll**            | `Cmd+Shift+S`     | Vim-style `j`/`k` scrolling, `u`/`d` for half pages     | Reading docs and code without lifting your hands |

All bindings are fully remappable. → [Configuration Guide](docs/CONFIGURATION.md)

---

## CLI-First, By Design

```bash
neru hints             # trigger hints mode
neru grid              # trigger grid mode
neru recursive_grid    # trigger recursive grid mode
neru scroll            # trigger scroll mode
neru config reload     # hot-reload config without restarting
neru status            # check daemon state and permissions
```

Config lives at `~/.config/neru/config.toml` — plain text, version-controllable, hot-reloadable. Every action is available over IPC, so Neru composes naturally with skhd, Raycast, Alfred, Hammerspoon, or any shell script.

```bash
neru config init       # generate a fully-commented starter config
neru config validate   # check for errors before applying
neru config reload     # apply changes without restarting
```

→ [CLI Guide](docs/CLI.md) · [Configuration Guide](docs/CONFIGURATION.md)

---

## How Neru Compares

| Tool                                            | Approach                                      |  Price   |    Open Source    |
| :---------------------------------------------- | :-------------------------------------------- | :------: | :---------------: |
| **Neru**                                        | Grid + Recursive Grid + AX Hints + Vision OCR | **Free** |        ✅         |
| [Homerow](https://www.homerow.app/)             | AX-Tree Hints                                 |   Paid   |        ❌         |
| [Wooshy](https://wooshy.app)                    | AX-Tree Search-to-click                       |   Paid   |        ❌         |
| [Mouseless](https://mouseless.click/)           | Grid pointer control                          |   Paid   |        ❌         |
| [Scoot](https://github.com/mjrusso/scoot)       | AX Hints + Grid                               |   Free   |        ✅         |
| [Glyphlow](https://github.com/blindFS/Glyphlow) | Hints + vim text editing                      |   Free   |        ✅         |
| [Stochos](https://github.com/museslabs/stochos) | Keyboard driven mouse control                 |   Free   |        ✅         |
| [Vimac](https://github.com/dexterleng/vimac)    | AX Hints                                      |   Free   | ✅ (unmaintained) |
| [warpd](https://github.com/rvaiya/warpd)        | Grid + Hints + Normal Pointer                 |   Free   |        ✅         |

More modes, more engines, more platforms — and it's free.

---

## Platform Support

| Capability                | macOS | Linux | Windows |
| :------------------------ | :---: | :---: | :-----: |
| Recursive Grid            |  ✅   |  ✅   |   ✅    |
| Grid                      |  ✅   |  ✅   |   ✅    |
| Vim-Style Scroll          |  ✅   |  ✅   |   ✅    |
| Hints (Accessibility API) |  ✅   |  ◻️   |   ✅    |
| Hints (Vision OCR)        |  ✅   |  ◻️   |   ◻️    |
| Direct Mouse Injection    |  ✅   |  ✅   |   ✅    |
| Global Hotkeys            |  ✅   |  ✅   |   ✅    |
| Native Overlays           |  ✅   |  ✅   |   ✅    |

→ [Cross-Platform Details](docs/CROSS_PLATFORM.md)

---

## Community

- **Neru Dojo** — browser-based training game: [bernatgene.github.io/neru-dojo](https://bernatgene.github.io/neru-dojo/)
- **Discord** — share setups, ask questions: [discord.gg/KZwnwr9dz6](https://discord.gg/KZwnwr9dz6)
- **Author's setup** — [HOW-I-USE-NERU.md](HOW-I-USE-NERU.md)

### Community Configs

These community members share their Neru configs:

| Author                                 | Config                                                                                                           |
| :------------------------------------- | :--------------------------------------------------------------------------------------------------------------- |
| [@y3owk1n](https://github.com/y3owk1n) | [nix-system-config-v2](https://github.com/y3owk1n/nix-system-config-v2/blob/main/home-manager/packages/neru.nix) |
| [@y9san9](https://github.com/y9san9)   | [y9san9.neru](https://github.com/y9san9/y9san9.neru)                                                             |

Add yours: submit a PR with a link to your config.

---

## Documentation Index

| Guide                                          | What's Inside                                             |
| :--------------------------------------------- | :-------------------------------------------------------- |
| [Installation](docs/INSTALLATION.md)           | Homebrew, Nix, source build, Linux setup, permissions     |
| [CLI](docs/CLI.md)                             | Every command and flag                                    |
| [Configuration](docs/CONFIGURATION.md)         | Keybindings, themes, overlays, all settings + recipes     |
| [Troubleshooting](docs/TROUBLESHOOTING.md)     | Common issues and fixes                                   |
| [Contributing](CONTRIBUTING.md)                | Development setup, coding standards, testing, PR workflow |
| [Architecture](docs/ARCHITECTURE.md)           | System design, ports/adapters, cross-platform model       |
| [Cross-Platform Guide](docs/CROSS_PLATFORM.md) | Linux/Windows contribution guide                          |

---

## What people are saying

> _"neru: the ACTUAL BEST app for ditching mouse"_

[![Community YouTube Review](https://img.youtube.com/vi/OFnpYTDA2gY/maxresdefault.jpg)](https://www.youtube.com/watch?v=OFnpYTDA2gY)

https://github.com/user-attachments/assets/d99328a6-b5f9-402a-a01b-297da2ffb454

---

## Roadmap

### Near Term

- Strengthen macOS reliability around startup, config reloads, and mode transitions
- Expand contract tests around ports, adapters, and reload behavior
- Linux AT-SPI accessibility integration
- Linux notifications and active app detection

### Platform Status

| Backend                 | Status        | Priority Gaps                          |
| :---------------------- | :------------ | :------------------------------------- |
| Linux X11               | Supported     | AT-SPI, notifications                  |
| Linux Wayland (wlroots) | Supported     | AT-SPI, notifications                  |
| Linux Wayland (KDE)     | Supported     | AT-SPI, notifications                  |
| Linux Wayland (GNOME)   | Not supported | Input injection, overlay, hotkeys      |
| Windows                 | Basic         | Notifications, dark mode, app watching |

### Contributor Priorities

1. Platform adapters in `internal/core/infra/platform/`
2. Linux AT-SPI accessibility integration
3. Notifications and app detection (Linux + Windows)
4. Config reload regression coverage
5. GNOME support (input injection + overlay)

---

## Contributing

Neru is written in Go and Objective-C with a clean, modular architecture. Pull requests are welcome.

```bash
git checkout -b feature/your-feature
just test && just lint
# open a pull request
```

→ [Contributing Guide](CONTRIBUTING.md) · [Architecture](docs/ARCHITECTURE.md)

---

## Support the Project

Built and maintained by one person, entirely in spare time.

👉 [GitHub Sponsors](https://github.com/sponsors/y3owk1n)

---

## Just Need Window Management?

[mimi](https://github.com/y3owk1n/mimi) is the window and space management pieces extracted into a standalone tool.

```bash
brew install --cask y3owk1n/tap/mimi
```

---

## License

MIT — see [LICENSE](LICENSE).

<div align="center">
<br/>

**Stop reaching for the mouse. It takes one command.**

```bash
brew install --cask y3owk1n/tap/neru
```

<br/>
Welcome to the mouseless life.<br/><br/>
Made with ❤️ by <a href="https://github.com/y3owk1n">y3owk1n</a>

</div>
