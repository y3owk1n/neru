# Neru

**Master your keyboard, refine your workflow**

Navigate macOS without touching your mouse - keyboard-driven productivity at its finest 🖱️⌨️

<div align="center">

[![License](https://img.shields.io/github/license/y3owk1n/neru)](LICENSE)
![Platform](https://img.shields.io/badge/platform-macOS-lightgrey)
![Go Version](https://img.shields.io/github/go-mod/go-version/y3owk1n/neru)
[![Latest Release](https://img.shields.io/github/v/release/y3owk1n/neru)](https://github.com/y3owk1n/neru/releases)

</div>

<p align="center">
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#features">Features</a> •
  <a href="#documentation">Docs</a> •
  <a href="#contributing">Contributing</a>
</p>

---

## ✨ What is Neru?

**Neru (練る)** - a Japanese word meaning "to refine, polish, and master through practice" - is a free, open-source keyboard navigation tool for macOS. Navigate, click, and scroll anywhere on your screen without ever touching your mouse.

### 🎯 Why Choose Neru?

- 🆓 **Always free** - No paywalls, no subscriptions, no "upgrade to pro"
- 🎬 **Universal compatibility** - Works with native macOS apps, Electron apps, and all browsers
- ⚡ **Lightning fast** - Native performance with instant response
- 🛠️ **Power-user friendly** - Text-based config for version control and dotfile management
- 🤝 **Community-owned** - Your contributions shape the project
- 🔧 **Scriptable** - CLI commands enable automation and integration

### 🆚 Free Alternative To

Neru is a capable **free and open-source** replacement for:

- [Homerow](https://www.homerow.app/) - Modern keyboard navigation (paid)
- [Shortcat](https://shortcat.app/) - Keyboard productivity tool (discontinued)
- [Vimac](https://github.com/dexterleng/vimac) - Vim-style navigation (unmaintained)
- [Mouseless](https://mouseless.click/) - Grid based keyboard navigation (paid)

---

## 🚀 Get Started

### Install

```bash
# Homebrew (recommended)
brew tap y3owk1n/tap
brew install --cask y3owk1n/tap/neru

# Nix Flake
# Add to flake.nix: inputs.neru.url = "github:y3owk1n/neru";
# See docs/INSTALLATION.md for nix-darwin/home-manager setup

# Or build from source
git clone https://github.com/y3owk1n/neru.git
cd neru && just release
```

### Grant Permissions

1. Open **System Settings**
2. Navigate to **Privacy & Security → Accessibility**
3. Enable **Neru**

### Try It

```bash
# Start Neru
open -a Neru

# Try default hotkeys:
# Cmd+Shift+Space - Hint mode
# Cmd+Shift+G     - Grid mode
# Cmd+Shift+S     - Scroll
```

See [Installation Guide](docs/INSTALLATION.md) for detailed setup instructions.

---

## 🎯 Core Features

<div align="center">

| Feature                   | Description                                                 |
| ------------------------- | ----------------------------------------------------------- |
| 🎯 **Hint Labels**        | Click any visible element using keyboard labels             |
| 🎬 **Action Mode**        | Choose click type: left, right, double, middle, drag & drop |
| 📜 **Vim Scrolling**      | Scroll anywhere with `j`/`k`, `gg`/`G`, Ctrl+D/U            |
| 🌐 **Universal Support**  | Native apps, Electron, Chrome, Firefox, system UI           |
| ⚡ **Native Performance** | Built with Objective-C and Go for instant response          |
| 🛠️ **TOML Config**        | Highly customizable with text-based configuration           |
| 🚫 **App Exclusion**      | Disable Neru in specific applications                       |
| 💬 **CLI Control**        | IPC commands for scripting and automation                   |

</div>

### 🎮 How It Works

**Four Navigation Modes:**

1. **Hints Mode** - Accessibility-based labels on clickable elements
2. **Grid Mode** - Universal coordinate-based navigation (works everywhere!)
3. **Scroll Mode** - Vim-style scrolling at cursor position
4. **Action Mode** - Interactive mouse actions at cursor position

<div align="center">

![hints-preview](https://github.com/user-attachments/assets/71b13850-1b87-40b5-9ac0-93cff1f2e89b)
![grid-preview](https://github.com/user-attachments/assets/d452f972-ce23-4798-955b-6dbfa8435504)

[Hints Demo](demos/hints.md) • [Grid Demo](demos/grid.md)

</div>

## 📚 Documentation

- **[Installation Guide](docs/INSTALLATION.md)** - Homebrew, Nix, source builds
- **[Configuration](docs/CONFIGURATION.md)** - Complete TOML reference
- **[CLI Usage](docs/CLI.md)** - Command-line interface
- **[Troubleshooting](docs/TROUBLESHOOTING.md)** - Common issues & solutions
- **[Development](docs/DEVELOPMENT.md)** - Building & contributing

### ⚙️ Configuration

Neru uses TOML configuration with sensible defaults. Customize everything from hotkeys to visual styling.

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints"
"Cmd+Shift+G" = "grid"

[hints]
hint_characters = "asdfghjkl"
background_color = "#FFD700"
```

See [Configuration Guide](docs/CONFIGURATION.md) for all options.

## 🤝 Contributing

We welcome contributions! Here's how to get started:

1. **Fork & Clone** the repository
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Make your changes** following our [Coding Standards](docs/CODING_STANDARDS.md)
4. **Test thoroughly** (`just test && just lint`)
5. **Commit conventionally** and open a Pull Request

**Guidelines:**

- Keep PRs focused on a single change
- Add tests for new features
- Update documentation
- Follow existing code style

See [Development Guide](docs/DEVELOPMENT.md) for build instructions and architecture details.

### 🏗️ Design Philosophy

**Why TOML config over GUI?**

- ⚡ Faster editing than clicking through settings
- 📝 Version control friendly (dotfiles, git)
- 🔧 More powerful than UI constraints
- 🛠️ Reduces maintenance burden

**Why grid-based navigation?**

- ✅ Works everywhere (native apps, Electron, browsers, system UI)
- ⚡ Fast and reliable (no accessibility tree traversal)
- 🎯 Always accurate (clicks at exact coordinates)
- 🔧 Simple maintenance (no app-specific workarounds)

### 📊 Project Status

**Actively maintained** with community contributions. PRs welcome!

**Future Ideas:**

- Service management commands
- Improved app icons

**Known Issues:**

- Hold/unhold actions don't work in Finder (help appreciated!)

## ✅ Compatibility

Neru works with **everything**:

- **Native macOS Apps** - Finder, Safari, System Settings, Mail, etc.
- **Electron Apps** - VS Code, Cursor, Slack, Spotify, Obsidian, Discord
- **Browsers** - Chrome, Firefox, Safari, Arc, Brave, Zen
- **Creative Apps** - Adobe Illustrator, Photoshop, Figma
- **System UI** - Menubar, Dock, Mission Control, Notification Center

See [Troubleshooting Guide](docs/TROUBLESHOOTING.md) for app-specific issues.

---

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

Inspired by these excellent projects:

- [Homerow](https://www.homerow.app/) - Modern keyboard navigation
- [Vimac](https://github.com/dexterleng/vimac) - Vim-style navigation
- [Shortcat](https://shortcat.app/) - Keyboard productivity tool
- [Vimium](https://github.com/philc/vimium) - Browser Vim bindings
- [Mouseless](https://mouseless.click/) - Grid navigation

## 💬 Support

- 📖 [Troubleshooting Guide](docs/TROUBLESHOOTING.md) for common issues
- 🐛 [Open an issue](https://github.com/y3owk1n/neru/issues) for bugs
- 💬 [Discussions](https://github.com/y3owk1n/neru/discussions) for questions
- ⭐ Star this repo if you find Neru useful!

<div align="center">

**Made with ❤️ by [y3owk1n](https://github.com/y3owk1n)**

</div>
