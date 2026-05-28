<div align="center">

<img src="assets/neru-appicon.png" alt="Neru Logo" width="108" />

# 練る · Neru

**Navigate your entire screen without touching the mouse.**

![macOS Support](https://img.shields.io/badge/macOS-Stable-blue?style=flat-square&logo=apple)
[![Linux Support](https://img.shields.io/badge/Linux-X11_&_Wayland-blue?style=flat-square&logo=linux)](docs/LINUX_SETUP.md)
![Windows Support](https://img.shields.io/badge/Windows-Foundations-lightgrey?style=flat-square&logo=windows)
![Go Version](https://img.shields.io/github/go-mod/go-version/y3owk1n/neru?style=flat-square&logo=go)
[![Latest Release](https://img.shields.io/github/v/release/y3owk1n/neru?style=flat-square)](https://github.com/y3owk1n/neru/releases)
[![License](https://img.shields.io/github/license/y3owk1n/neru?style=flat-square)](LICENSE)

</div>

---

**Neru** _(練る — "to refine through practice")_ puts your cursor anywhere on the screen using only your keyboard. Click, scroll, drag, and select text — all without leaving the home row.

It is a free, blazing-fast, and open-source alternative to paid/subscription utilities like [Homerow](https://www.homerow.app/), [Mouseless](https://mouseless.click/), and [Wooshy](https://wooshy.app). It is fully configurable, lightweight, and respects your privacy.

> [!TIP]
> See how the author uses Neru day-to-day → [HOW-I-USE-NERU.md](HOW-I-USE-NERU.md) and watch the `recursive_grid` demo:
>
> https://github.com/user-attachments/assets/6b5673e1-7131-4bc0-ad57-41678e9423b9

---

## 🧭 Navigation Modes

Neru offers four distinct modes tailored for every navigation requirement.

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
<sub><b>Hints (Dual-Engine)</b><br/>AX-Tree & Vision OCR</sub>
</td>
</tr>
</table>

| Mode                    | Core Mechanism                                                                                       | Best For                                                                    |
| :---------------------- | :--------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------------- |
| **Recursive Grid** ⭐   | Divides screen into cells; narrow recursively using home-row keys (`u`/`i`/`j`/`k`).                 | **Everything** — works flawlessly in any app, web browser, canvas, or game. |
| **Hints (Dual-Engine)** | Places labels on clickable elements using either OS **Accessibility APIs** or **Native Vision OCR**. | Standard macOS apps, Electron apps, and complex design tools (e.g. Figma).  |
| **Grid**                | Standard coordinate grid; jump directly by row + column label.                                       | Rapid, coarse pointer adjustments across large monitors.                    |
| **Scroll**              | Vim-style keyboard scrolling (`j`/`k` to scroll, `u`/`d` for half pages).                            | Reading docs, articles, and code without lifting your hands.                |

---

## 🚀 Features

- **Full Pointer Control** — Bind left, right, middle clicks, double clicks, and drag & drop to home-row keys.
- **Sticky Modifiers** — Tap `Shift` or `Cmd` once to apply them to your next mouse click without holding.
- **Universal Application Support** — Works in native AppKit/SwiftUI, Electron (VS Code, Slack, Obsidian), Web Browsers (Safari, Chrome, Firefox), Creative Suites (Figma), and system components (Dock, Menu Bar, Mission Control).
- **CLI & Automation** — Rich IPC-based command line interface for custom shell scripting, hammerspoon integration, and keyboard managers (e.g. Raycast, Alfred).
- **TOML Configuration** — Easily version-control your entire setup (keybindings, theme, overlays) in a single plain text file.

---

## 📥 Installation

### macOS (Homebrew — Recommended)

> [!NOTE]
> The Homebrew tap is maintained at [y3owk1n/homebrew-tap](https://github.com/y3owk1n/homebrew-tap).

```bash
brew tap y3owk1n/tap
brew install --cask y3owk1n/tap/neru
```

### macOS / Linux (Nix Flake)

```bash
# Inputs: inputs.neru.url = "github:y3owk1n/neru";
# Modules: nix-darwin (macOS) · nixosModules (Linux) · home-manager (both)
# See docs/INSTALLATION.md for comprehensive setup.
```

### Building From Source

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru && just release
```

### Post-Installation Setup

1. **macOS Accessibility:** Grant access in **System Settings → Privacy & Security → Accessibility → enable Neru**.
2. **Daemon Management:**

```bash
open -a Neru              # Launch the app (one off launching)
neru services install     # Auto-start on login (highly recommended) and restart on exit
```

For display-server requirements on Linux, see the [Linux Setup Guide](docs/LINUX_SETUP.md).

---

## ⌨️ Default Hotkeys

All hotkeys are remappable to suit your custom keyboard layouts (Colemak, Dvorak, etc.).

| Hotkey              | Action                         |
| :------------------ | :----------------------------- |
| `Cmd+Shift+C`       | Activate **Recursive Grid** ⭐ |
| `Cmd+Shift+Space`   | Activate **Hints**             |
| `Cmd+Shift+G`       | Activate **Grid**              |
| `Cmd+Shift+S`       | Activate **Scroll**            |
| `Shift+L` (in mode) | Execute Left Click             |
| `Shift+R` (in mode) | Execute Right Click            |

> Read the [Configuration Reference](docs/CONFIGURATION.md#hotkeys) to customize bindings.

---

## ⚙️ Configuration

Your configuration lives at `~/.config/neru/config.toml`. It's human-readable, dotfile-friendly, and changes are applied instantly without restarting.

```bash
neru config init      # Generate a fully-commented starter configuration
neru config validate  # Validate your TOML file structure and types
neru config reload    # Hot-reload configurations into the running daemon
```

For all settings, see the [Configuration Guide](docs/CONFIGURATION.md) and explore user showcases in [Config Showcases](docs/CONFIG_SHOWCASES.md).

---

## ⚖️ How Neru Compares

### macOS Ecosystem

Neru is unique in offering both coordinate-based grids and a dual-engine (AX + Vision OCR) hints mode in a single free, open-source tool.

| Tool                                            | Engine Approach                                              |  Price   | Open Source | Active / Maintained |
| :---------------------------------------------- | :----------------------------------------------------------- | :------: | :---------: | :-----------------: |
| **Neru** 練る                                   | **Grid + Recursive Grid + AX-Tree Hints + Vision OCR Hints** | **Free** |     ✅      |      ✅ Active      |
| [Homerow](https://www.homerow.app/)             | AX-Tree Hints (with fuzzy search)                            |   Paid   |     ❌      |      ✅ Active      |
| [Wooshy](https://wooshy.app)                    | AX-Tree Search-to-click                                      |   Paid   |     ❌      |      ✅ Active      |
| [Mouseless](https://mouseless.click/)           | Grid pointer control                                         |   Paid   |     ❌      |      ✅ Active      |
| [Scoot](https://github.com/mjrusso/scoot)       | AX Hints + Grid                                              |   Free   |     ✅      |   🔲 Low Activity   |
| [Vimac](https://github.com/dexterleng/vimac)    | AX Hints                                                     |   Free   |     ✅      |   ❌ Unmaintained   |
| [warpd](https://github.com/rvaiya/warpd)        | Grid + Hints + Normal Pointer                                |   Free   |     ✅      |   🔲 Low Activity   |
| [Shortcat](https://shortcat.app/)               | Hints (fuzzy search)                                         |   Free   |     ❌      |   ❌ discontinued   |
| [Glyphlow](https://github.com/blindFS/Glyphlow) | Hints + vim text editing                                     |   Free   |     ✅      |      ✅ Active      |

### Browser Extensions

If you use keyboard navigators on the web, Neru brings that exact comfort to your entire operating system:

- [Vimium](https://github.com/philc/vimium) — Standard hints-based web navigation.
- [Vimium C](https://github.com/gdh1995/vimium-c) — Extended fast Vimium branch.
- [Tridactyl](https://github.com/tridactyl/tridactyl) — Complete Vim environment inside Firefox.

---

## 💻 Platform Support & Capabilities

macOS is fully featured and stable. Support for Linux (X11 & Wayland) is actively developing.

| Capability                     | macOS | Linux | Windows |
| :----------------------------- | :---: | :---: | :-----: |
| **Recursive Grid**             |  ✅   |  ✅   |   🔲    |
| **Grid**                       |  ✅   |  ✅   |   🔲    |
| **Vim-Style Scroll**           |  ✅   |  ✅   |   🔲    |
| **Direct Mouse Injection**     |  ✅   |  ✅   |   🔲    |
| **Global Hotkeys**             |  ✅   |  ✅   |   🔲    |
| **Accessibility Hints (AX)**   |  ✅   |  🔲   |   🔲    |
| **Vision-Powered Hints (OCR)** |  ✅   |  🔲   |   🔲    |
| **Native High-Perf Overlays**  |  ✅   |  ✅   |   🔲    |

> Detailed backend documentation and roadmap plans live in [docs/ROADMAP.md](docs/ROADMAP.md) and [docs/CROSS_PLATFORM.md](docs/CROSS_PLATFORM.md).

---

## 🤝 Contributing

Neru has a modular, clean hexagonal architecture (Ports & Adapters) written in Go and Objective-C, making it exceptionally easy to add features or platform adapters.

```bash
git checkout -b feature/amazing-feature
just test && just lint
# Open a pull request!
```

Refer to [Development Guide](docs/DEVELOPMENT.md) and [Coding Standards](docs/CODING_STANDARDS.md) for more details.

---

## 💖 Sponsoring

Neru is built and maintained entirely in spare time. If Neru has streamlined your day-to-day workflow, improved your ergonomics, or saved you money, please consider sponsoring:

👉 [**GitHub Sponsors**](https://github.com/sponsors/y3owk1n)

---

## 🎓 Acknowledgments

We stand on the shoulders of giants. Our deepest gratitude goes to the creators of [Homerow](https://www.homerow.app/), [Vimac](https://github.com/dexterleng/vimac), [Vimium](https://github.com/philc/vimium), [Mouseless](https://mouseless.click/), and [Shortcat](https://shortcat.app/).

---

## 📄 License

Distributed under the MIT License. See [LICENSE](LICENSE) for more details.

<div align="center">

**Made with ❤️ by [y3owk1n](https://github.com/y3owk1n)**

⭐ Star this repository if Neru makes your workflow better!

</div>
