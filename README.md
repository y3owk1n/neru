<div align="center">

<img src="assets/neru-appicon.png" alt="Neru Logo" width="128" />

# 練る · Neru

**Mouse-free OS navigation. Free, open-source, endlessly customisable.**

[![License](https://img.shields.io/github/license/y3owk1n/neru)](LICENSE)
![Platform](<https://img.shields.io/badge/platform-macOS%20(stable)%20%7C%20Linux%20%26%20Windows%20(WIP)-lightgrey>)
![Go Version](https://img.shields.io/github/go-mod/go-version/y3owk1n/neru)
[![Latest Release](https://img.shields.io/github/v/release/y3owk1n/neru)](https://github.com/y3owk1n/neru/releases)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/y3owk1n/neru)

[**Features**](#-features) · [**Get Started**](#-get-started) · [**Docs**](#-documentation) · [**Platform Support**](#-platform-support)

---

<table>
<tr>
<td align="center" width="33%">
<img src="https://github.com/user-attachments/assets/0d46fa7c-345a-45ee-ad44-7a601c2b7cb1" alt="Recursive Grid Mode" /><br/>
<sub><b>Recursive Grid</b> · recommended</sub>
</td>
<td align="center" width="33%">
<img src="https://github.com/user-attachments/assets/d452f972-ce23-4798-955b-6dbfa8435504" alt="Grid Mode" /><br/>
<sub><b>Grid</b></sub>
</td>
<td align="center" width="33%">
<img src="https://github.com/user-attachments/assets/71b13850-1b87-40b5-9ac0-93cff1f2e89b" alt="Hints Mode" /><br/>
<sub><b>Hints</b></sub>
</td>
</tr>
</table>

</div>

---

## What is Neru?

**Neru (練る)** — Japanese for _"to refine and master through practice"_ — lets you navigate, click, and scroll anywhere on your screen using only your keyboard. No mouse. No trackpad. No limits.

It's the free, open-source alternative to [Homerow](https://www.homerow.app/), [Mouseless](https://mouseless.click/), and [Wooshy](https://wooshy.app) — with zero paywalls, zero subscriptions, and everything configurable down to the last pixel.

> Want to see how the author actually uses Neru day-to-day? [Read the full story →](HOW-I-USE-NERU.md)

---

## ✨ Features

### Navigation modes

| Mode                  | How it works                                                      | Best for                                       |
| --------------------- | ----------------------------------------------------------------- | ---------------------------------------------- |
| **Recursive Grid** ⭐ | Divide screen into cells, narrow recursively with `u`/`i`/`j`/`k` | Universal — works in every app, every window   |
| **Grid**              | Coordinate grid, select by row+column label                       | Quick coarse navigation                        |
| **Hints**             | Labels appear on every clickable UI element                       | Standard macOS apps with accessibility support |
| **Scroll**            | Vim-style `j`/`k`, `gg`/`G`, `d`/`u` scrolling                    | Scrolling without touching the mouse           |

> Recursive Grid is recommended as your daily driver — it's precise, predictable, and works everywhere out-of-the-box with no app-specific setup.

### Everything else

- **Direct mouse actions** — left click, right click, middle click, drag & drop, all bound to keys you define
- **Sticky modifiers** — tap `Shift` or `Cmd` to apply it to your next click, no holding required
- **Per-app exclusions** — disable Neru in specific apps by bundle ID
- **Full CLI & scripting** — IPC-based CLI for automation, hotkey managers, and shell scripts
- **TOML configuration** — every keybinding, color, and behavior in a single file you can version-control

---

## 🚀 Get Started

### 1. Install

```bash
# Homebrew (recommended)
brew tap y3owk1n/tap
brew install --cask y3owk1n/tap/neru

# Nix Flake
# inputs.neru.url = "github:y3owk1n/neru";
# See docs/INSTALLATION.md for nix-darwin / home-manager setup

# Build from source
git clone https://github.com/y3owk1n/neru.git
cd neru && just release
```

### 2. Grant accessibility permission

1. Open **System Settings → Privacy & Security → Accessibility**
2. Enable **Neru**

### 3. Launch

```bash
# Start Neru
open -a Neru

# Auto-start on login
neru services install
```

### Default hotkeys

| Hotkey            | Action                               |
| ----------------- | ------------------------------------ |
| `Cmd+Shift+C`     | Recursive Grid mode ⭐               |
| `Cmd+Shift+G`     | Grid mode                            |
| `Cmd+Shift+Space` | Hints mode                           |
| `Cmd+Shift+S`     | Scroll mode                          |
| `Shift+L`         | Left click (in any navigation mode)  |
| `Shift+R`         | Right click (in any navigation mode) |

All hotkeys are fully remappable. See the [Configuration Reference →](docs/CONFIGURATION.md#hotkeys)

> **Note:** Defining any custom hotkey in your config replaces all defaults. Include every hotkey you want to keep.

Full setup walkthrough: [Installation Guide →](docs/INSTALLATION.md)

---

## ✅ Works everywhere

| Category     | Apps                                                |
| ------------ | --------------------------------------------------- |
| Native macOS | Finder, Safari, Mail, System Settings, Terminal     |
| Electron     | VS Code, Cursor, Slack, Spotify, Obsidian, Discord  |
| Browsers     | Chrome, Firefox, Safari, Arc, Brave, Zen            |
| Creative     | Figma, Adobe Illustrator, Photoshop                 |
| System UI    | Menubar, Dock, Mission Control, Notification Center |

Grid and Recursive Grid modes work universally — no accessibility support needed. Hints mode works where the accessibility tree is exposed. See the [Troubleshooting Guide](docs/TROUBLESHOOTING.md) for app-specific notes.

---

## ⚙️ Configuration

Neru is configured entirely through a single **TOML file** — no GUI required. Every keybinding, color, and behavior is yours to define.

```bash
neru config init      # Generate a fully-commented starter config
neru config validate  # Check for errors
neru config reload    # Apply changes to a running daemon
```

Config file location: `~/.config/neru/config.toml`

**Why TOML over a settings UI?**

- Version-control your config in your dotfiles
- Edit faster than clicking through preference panels
- Shareable, reproducible, scriptable

See the full [Configuration Reference →](docs/CONFIGURATION.md)

Want inspiration? Check out other users' configs in the [community discussion →](https://github.com/y3owk1n/neru/discussions/542)

---

## 🆚 How Neru compares

### macOS tools

| Tool                                         | Approach                               | Price    | Status          | Open Source |
| -------------------------------------------- | -------------------------------------- | -------- | --------------- | ----------- |
| **Neru**                                     | Hints + Grid + Recursive Grid + Scroll | **Free** | ✅ Active       | ✅ Yes      |
| [Scoot](https://github.com/mjrusso/scoot)    | Hints + Grid + Freestyle               | Free     | ✅ Active       | ✅ Yes      |
| [Homerow](https://www.homerow.app/)          | Hints (fuzzy search + labels)          | Paid     | ✅ Active       | ❌ No       |
| [Wooshy](https://wooshy.app)                 | Hints (search-to-click, minimalist UI) | Paid     | ✅ Active       | ❌ No       |
| [Mouseless](https://mouseless.click/)        | Grid-based pointer control             | Paid     | ✅ Active       | ❌ No       |
| [Shortcat](https://shortcat.app/)            | Hints (fuzzy search)                   | Free     | ❌ Discontinued | ❌ No       |
| [Vimac](https://github.com/dexterleng/vimac) | Hints + Grid (Homerow's predecessor)   | Free     | ⚠️ Unmaintained | ✅ Yes      |
| [warpd](https://github.com/rvaiya/warpd)     | Hints + Grid + Normal mode             | Free     | ⚠️ Low activity | ✅ Yes      |

### Cross-platform / Linux / Windows tools

| Tool                                                   | Platform      | Approach                   | Price | Status          |
| ------------------------------------------------------ | ------------- | -------------------------- | ----- | --------------- |
| [warpd](https://github.com/rvaiya/warpd)               | macOS + Linux | Hints + Grid + Normal mode | Free  | ⚠️ Low activity |
| [mousemaster](https://github.com/petoncle/mousemaster) | Windows       | Hints + Grid + Normal mode | Free  | ✅ Active       |

### Browser extensions

| Tool                                                          | Approach                           | Price |
| ------------------------------------------------------------- | ---------------------------------- | ----- |
| [Vimium](https://github.com/philc/vimium) (Chrome/Firefox)    | Hints-based link navigation        | Free  |
| [Vimium C](https://github.com/gdh1995/vimium-c) (Chrome)      | Extended Vimium with more features | Free  |
| [Tridactyl](https://github.com/tridactyl/tridactyl) (Firefox) | Full Vim emulation in browser      | Free  |

---

## 📚 Documentation

- [Installation Guide](docs/INSTALLATION.md) — Homebrew, Nix, source builds
- [Configuration Reference](docs/CONFIGURATION.md) — every TOML option
- [CLI Usage](docs/CLI.md) — IPC commands and scripting
- [Troubleshooting](docs/TROUBLESHOOTING.md) — common issues and app-specific fixes
- [Development Guide](docs/DEVELOPMENT.md) — architecture and build instructions
- [System Architecture](docs/ARCHITECTURE.md) — comprehensive architecture guide and porting instructions

---

## 🤝 Contributing

Contributions are welcome. The project is small and the codebase is approachable.

```bash
git checkout -b feature/your-idea
just test && just lint
# open a PR
```

Follow the [Coding Standards](docs/CODING_STANDARDS.md) and keep PRs focused on a single change. See [Development Guide](docs/DEVELOPMENT.md) for architecture details.

---

## 💻 Platform support

Neru uses a **Hexagonal Architecture (Ports and Adapters)** that isolates OS-specific logic from core business rules, making it straightforward to port to new platforms.

### Current status

| Platform    | Status                                                          |
| ----------- | --------------------------------------------------------------- |
| **macOS**   | ✅ Fully supported — all features stable                        |
| **Linux**   | 🔲 Foundations ready — native implementation needs contributors |
| **Windows** | 🔲 Foundations ready — native implementation needs contributors |

### Compatibility matrix

| Capability                | macOS | Linux | Windows |
| :------------------------ | :---: | :---: | :-----: |
| Recursive Grid Mode       |  ✅   |  🔲   |   🔲    |
| Grid Mode                 |  ✅   |  🔲   |   🔲    |
| Hints Mode                |  ✅   |  🔲   |   🔲    |
| Vim-Style Scrolling       |  ✅   |  🔲   |   🔲    |
| Direct Mouse Actions      |  ✅   |  🔲   |   🔲    |
| Global Hotkeys            |  ✅   |  🔲   |   🔲    |
| Accessibility Integration |  ✅   |  🔲   |   🔲    |
| Native Overlays           |  ✅   |  🔲   |   🔲    |

> ✅ = Fully Supported | 🔲 = Contributor Needed (Stub Implementation)

### Roadmap

- **Phase 1 — macOS (current)**
  - [x] Stable core architecture
  - [x] High-performance native macOS bridge
  - [x] Comprehensive feature set
- **Phase 2 — Linux**
  - [ ] AT-SPI accessibility integration
  - [ ] X11/Wayland event capture
  - [ ] Native Linux overlays
- **Phase 3 — Windows**
  - [ ] UI Automation (UIA) integration
  - [ ] Windows Hooks for event capture
  - [ ] Win32/WinUI overlays

**Looking to contribute?** Check issues labeled [`cross-platform`](https://github.com/y3owk1n/neru/issues?q=is%3Aopen+is%3Aissue+label%3Across-platform) or join the [Linux Support Discussion](https://github.com/y3owk1n/neru/discussions/559).

---

## 🙏 Acknowledgments

Built on the shoulders of:
[Homerow](https://www.homerow.app/) · [Vimac](https://github.com/dexterleng/vimac) · [Vimium](https://github.com/philc/vimium) · [Mouseless](https://mouseless.click/) · [Shortcat](https://shortcat.app/)

---

## 📄 License

MIT — see [LICENSE](LICENSE) for details.

<div align="center">

**Made with ❤️ by [y3owk1n](https://github.com/y3owk1n)**

⭐ Star this repo if Neru makes your workflow better

</div>
