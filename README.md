<div align="center">

<img src="assets/neru-appicon.png" alt="Neru Logo" width="128" />

# 練る · Neru

**Mouse-free cross platform OS navigation. Free, open-source, endlessly customisable.**

[![License](https://img.shields.io/github/license/y3owk1n/neru)](LICENSE)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)
![Go Version](https://img.shields.io/github/go-mod/go-version/y3owk1n/neru)
[![Latest Release](https://img.shields.io/github/v/release/y3owk1n/neru)](https://github.com/y3owk1n/neru/releases)

[**Get Started**](#-get-started) · [**Features**](#-features) · [**Configuration**](#%EF%B8%8F-configuration) · [**Docs**](#-documentation) · [**Contributing**](#-contributing)

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

> [!WARNING]
> Neru claims that it can be cross platform, but currently only supports macOS.
> Linux and Windows support foundations are laid out, but for the community to implement together.
> The author does not have the resources to support Windows and Linux.
> See [linux support discussion here](https://github.com/y3owk1n/neru/discussions/559)

## What is Neru?

**Neru (練る)** — Japanese for _"to refine and master through practice"_ — lets you navigate, click, and scroll anywhere on your screen using only your keyboard. No mouse. No trackpad. No limits.

It's the free, open-source alternative to [Homerow](https://www.homerow.app/), [Mouseless](https://mouseless.click/), and [Wooshy](https://wooshy.app) — with zero paywalls, zero subscriptions, and everything configurable down to the last pixel.

> Want to see how the author actually uses Neru day-to-day? [Read the full story →](HOW-I-USE-NERU.md)

---

Before you dive in, let's see if Neru fits your workflow.

**You'll probably love Neru if you are:**

- Uses a split keyboard
- A Neovim/Vim user
- Already using tools like Vimium, Homerow, Shortcat, Mouseless, etc, but looking for open source alternatives
- The kind of person who remaps Caps Lock
- Interested in tiling window managers or keyboard-driven UI
- Someone who enjoys optimising tiny workflow inefficiencies
- Wanted a dotfile based config
- Like to customise hell out of everything
- Like to have your own workflows

**You'll probably not care about Neru if you:**

- Prefer using the mouse for everything
- Never thought about keyboard-driven workflows
- Are perfectly happy switching between mouse and keyboard every few seconds
- Prefer traditional desktop interaction over power-user tooling
- In love with GUIs

Neru is built for power users who want to stay on the keyboard and move fast.

If that's you — welcome.

---

## ✨ Features

### 🎯 3 Navigation Modes

**Recursive Grid Mode** _(recommended)_ — divide your screen into a coordinate grid and drill into any region with `u`/`i`/`j`/`k` until you land exactly where you want. Works in every app, every window, every corner of macOS — no exceptions, no app-specific setup.

**Grid Mode** — same coordinate-based approach without the recursive subdivision, great for quick coarse navigation.

**Hints Mode** — labels appear on every clickable element for instant accessibility-driven navigation. Requires accessibility tree support, so some apps may need additional configuration.

> Recursive Grid is recommended as your daily driver: it's precise, predictable, works everywhere out-of-the-box, and never needs app-specific tweaks.

### 🖱️ Direct Mouse Actions

In any navigation mode, trigger mouse actions without switching modes — left click, right click, double click, middle click, and drag & drop via direct action keybindings.

### 📜 Vim-Style Scrolling

Scroll anywhere with familiar `j`/`k`, `gg`/`G`, and `Ctrl+D`/`Ctrl+U` bindings — all fully remappable.

### 🚫 Per-App Exclusions

Disable Neru in specific apps where you don't need it.

### 💬 CLI & Scripting

Full IPC-based CLI lets you control Neru programmatically, integrate with other tools, or build your own automation workflows.

---

## ⚙️ Configuration

Neru is configured entirely through a single **TOML file** — no GUI required. Every keybinding, every color, every behavior is yours to define.

**Why TOML over a settings UI?**

- Version-control your config in your dotfiles
- Edit faster than clicking through preference panels
- No UI = less code to maintain = more stability
- Shareable, reproducible, scriptable

See the full [Configuration Reference →](docs/CONFIGURATION.md)

Want to get inspired? Check out other neru users' configs [here](https://github.com/y3owk1n/neru/discussions/542)

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

### 2. Grant Accessibility Permission

1. Open **System Settings → Privacy & Security → Accessibility**
2. Enable **Neru**

### 3. Launch & Try

```bash
# Start Neru
open -a Neru

# Auto-start on login
neru services install
```

| Default Hotkey    | Action                                  |
| ----------------- | --------------------------------------- |
| `Cmd+Shift+C`     | **Recursive grid mode** _(recommended)_ |
| `Cmd+Shift+G`     | Grid mode                               |
| `Cmd+Shift+Space` | Hints mode                              |
| `Cmd+Shift+S`     | Scroll mode                             |
| `Shift+L`         | Left click (in any mode)                |
| `Shift+R`         | Right click (in any mode)               |

> These are just subset of available keys. All hotkeys and actions are fully remappable.

Full setup walkthrough: [Installation Guide →](docs/INSTALLATION.md)

---

## ✅ Works Everywhere

| Category     | Apps                                                |
| ------------ | --------------------------------------------------- |
| Native macOS | Finder, Safari, Mail, System Settings, Terminal     |
| Electron     | VS Code, Cursor, Slack, Spotify, Obsidian, Discord  |
| Browsers     | Chrome, Firefox, Safari, Arc, Brave, Zen            |
| Creative     | Figma, Adobe Illustrator, Photoshop                 |
| System UI    | Menubar, Dock, Mission Control, Notification Center |

Grid mode works universally. Hints mode works where the accessibility tree is exposed — see the [Troubleshooting Guide](docs/TROUBLESHOOTING.md) for app-specific notes.

---

## 🆚 Free Alternative To

| Tool                                         | Status       | Neru          |
| -------------------------------------------- | ------------ | ------------- |
| [Homerow](https://www.homerow.app/)          | Paid         | ✅ Free       |
| [Mouseless](https://mouseless.click/)        | Paid         | ✅ Free       |
| [Wooshy](https://wooshy.app)                 | Paid         | ✅ Free       |
| [Shortcat](https://shortcat.app/)            | Discontinued | ✅ Active     |
| [Vimac](https://github.com/dexterleng/vimac) | Unmaintained | ✅ Maintained |

---

## 📚 Documentation

- [Installation Guide](docs/INSTALLATION.md) — Homebrew, Nix, source builds
- [Configuration Reference](docs/CONFIGURATION.md) — every TOML option
- [CLI Usage](docs/CLI.md) — IPC commands and scripting
- [Troubleshooting](docs/TROUBLESHOOTING.md) — common issues and app-specific fixes
- [Development](docs/DEVELOPMENT.md) — architecture and build instructions
- [Cross-Platform Architecture](docs/ARCHITECTURE_CROSS_PLATFORM.md) — porting guide for Linux/Windows contributors

---

## 🤝 Contributing

Contributions are welcome. The project is small and the codebase is approachable.

```bash
git checkout -b feature/your-idea
just test && just lint
# open a PR
```

Follow the [Coding Standards](docs/CODING_STANDARDS.md) and keep PRs focused on a single change. See [Development Guide](docs/DEVELOPMENT.md) for architecture details.

**Good first contributions:**

- New navigation mechanisms
- Additional mouse action types
- Config examples for common setups
- Demo videos
- Performance improvements
- Bug fixes

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
