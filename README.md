<div align="center">

<img src="assets/neru-appicon.png" alt="Neru" width="80" />

# Neru

Hints, grids and vim keys for your whole desktop. Free, open source, one binary, one TOML file.

[![Latest Release](https://img.shields.io/github/v/release/y3owk1n/neru?style=flat-square)](https://github.com/y3owk1n/neru/releases)
[![License](https://img.shields.io/github/license/y3owk1n/neru?style=flat-square)](LICENSE)
[![Discord](https://img.shields.io/discord/1502261043701874698?style=flat-square&label=discord)](https://discord.gg/KZwnwr9dz6)
[![Sponsor](https://img.shields.io/badge/sponsor-%E2%9D%A4-30363D?style=flat-square)](https://github.com/sponsors/y3owk1n)

|     macOS 14+     |   Linux (X11, Wayland)   |     Windows 10+     |
| :---------------: | :----------------------: | :-----------------: |
|     Stable ✅     |      Beta, daily 🔵      |    Beta, daily 🔵   |

<sub>Beta means every option, flag and action already works. Stable comes after six clean releases. [What the labels mean](docs/CROSS_PLATFORM.md#what-the-labels-mean)</sub>

[Install](#install) · [Modes](#pick-your-mode) · [Make it yours](#make-it-yours) · [Compare](#how-neru-compares) · [Docs](#documentation)

</div>

---

https://github.com/user-attachments/assets/6b5673e1-7131-4bc0-ad57-41678e9423b9

If you use Vimium in the browser, you already know the feeling. Neru brings it to every app, window, menu bar and dock item on your screen.

```
Cmd+Shift+Space   labels appear on every clickable element
type the label    cursor jumps there
Shift+L           left click
```

## Why Neru

- **Works where accessibility trees don't.** Grid and recursive grid split pixels, not widgets, so they work in canvases, games, remote desktops and Electron apps with thin trees. Hints have three engines: the accessibility tree, on-device OCR, and a pure-Go contour pass that needs no OCR at all.
- **Latency is the product.** The event tap sits on every keystroke. Anything that makes activation or key handling feel slower counts as a bug.
- **No per-app hacks.** Good defaults, then per-app overrides you write yourself: hotkeys, hint engine, scroll step, all re-resolved the instant focus changes.
- **CLI-first, scriptable everywhere.** One executor backs hotkeys, the `neru` CLI and the IPC socket. A sequence that works in a binding works from skhd, Hammerspoon, Raycast or a shell script unchanged.
- **Nothing leaves your machine.** OCR and element detection run on-device. No telemetry, no accounts, and typed text never reaches the log.
- **Fails loudly, recovers cleanly.** A typo'd flag in a binding fails config validation instead of doing nothing when you press the key.

---

## Install

One command on any platform. It installs the binary, man pages, shell completions and a login service, and re-running it updates.

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash
```

```powershell
# Windows
irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1 | iex
```

Pass `--channel nightly` or `--version vX.Y.Z` after `bash -s --` to pick a channel or pin a release. `--uninstall` removes it again.

<details>
<summary>Homebrew (macOS)</summary>

```bash
brew tap y3owk1n/tap
brew install --cask y3owk1n/tap/neru
```

</details>

<details>
<summary>Nix (NixOS, nix-darwin, home-manager)</summary>

Add the flake as an input, then:

```nix
services.neru.enable = true;
services.neru.settings = { /* your config.toml, as Nix */ };
```

Examples for each module are in the [Installation Guide](docs/INSTALLATION.md).

</details>

<details>
<summary>Prebuilt binaries</summary>

Download from [GitHub Releases](https://github.com/y3owk1n/neru/releases/latest):

| Platform | Architecture | File                     |
| :------- | :----------- | :----------------------- |
| macOS    | Apple Silicon| `neru-darwin-arm64.zip`  |
| macOS    | Intel        | `neru-darwin-amd64.zip`  |
| Linux    | x86_64       | `neru-linux-amd64.zip`   |
| Linux    | ARM64        | `neru-linux-arm64.zip`   |
| Windows  | x86_64       | `neru-windows-amd64.zip` |
| Windows  | ARM64        | `neru-windows-arm64.zip` |

Every archive ships a `.sha256` file, and releases since August 2026 carry a signed build provenance attestation:

```bash
gh attestation verify neru-darwin-arm64.zip --repo y3owk1n/neru
```

</details>

<details>
<summary>From source</summary>

```bash
git clone https://github.com/y3owk1n/neru.git && cd neru
just install   # builds, then runs the same installer as the one-liner
```

</details>

### First run

```bash
neru launch    # or open Neru.app on macOS, or the Start Menu entry on Windows
```

Neru offers to write a starter config and asks for what it needs:

| Platform | Needs                                                                 |
| :------- | :-------------------------------------------------------------------- |
| macOS    | Accessibility permission. Screen Recording only for OCR or contour hints. |
| Linux    | Your user in the `input` group and a `/dev/uinput` udev rule. [Linux Setup](docs/LINUX_SETUP.md) |
| Windows  | Nothing beyond the install.                                           |

On Linux there are no default global hotkeys, to avoid colliding with terminal and desktop shortcuts. Bind the modes in `[hotkeys]` or in your compositor. [Global hotkeys on Linux](docs/CONFIGURATION.md#hotkeys)

---

## Pick your mode

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
<sub><b>Hints</b></sub>
</td>
</tr>
</table>

| Mode                  | Default hotkey        | How it works                                                    | Best for                                    |
| :-------------------- | :-------------------- | :-------------------------------------------------------------- | :------------------------------------------ |
| **Recursive Grid** ⭐ | `Primary+Shift+C`     | Halve the screen with home-row keys until the cursor lands      | Anything, including canvases and games      |
| **Hints**             | `Primary+Shift+Space` | Labels every target via accessibility tree, OCR or contour scan | Native apps, Electron, browsers, Figma      |
| **Grid**              | `Primary+Shift+G`     | Type a row and column label to jump to a cell                   | Coarse jumps across big monitors            |
| **Scroll**            | `Primary+Shift+S`     | `j`/`k`, `u`/`d`, `gg`/`G`, `h`/`l` in any scroll view          | Reading without lifting your hands          |
| **Monitor Select**    | unbound               | Labels each display, type one to jump                           | Multi-monitor setups                        |
| **Your own**          | unbound               | A name, an indicator and a key table you declare in config      | Window management layers, app-specific keys |

`Primary` is `Cmd` on macOS and `Ctrl` on Linux and Windows. Every binding is remappable, so Colemak and Dvorak users are covered.

Inside any mode you get the full pointer: left, right and middle click, double and triple click, drag with any button, held-key glide with acceleration, and sticky modifiers you tap instead of hold.

---

## Make it yours

Config is one TOML file at `~/.config/neru/config.toml` (`%APPDATA%\neru\config.toml` on Windows). `neru config reload` applies edits without restarting the daemon, and `neru config set` changes one value at runtime and persists it to an override file.

**Click on select, Vimium style.**

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --action left_click"
```

**Per-app overrides**, re-resolved the moment focus changes.

```toml
[[hints.app_configs]]
bundle_id = "com.brave.Browser"                          # macOS: bundle ID
hotkeys = { "Return" = "action left_click" }

[[hints.app_configs]]
bundle_id = "firefox"                                    # Linux: WM_CLASS or Wayland app_id
hotkeys = { "Return" = "action left_click" }

[[hints.app_configs]]
bundle_id = 'C:\Program Files\Google\Chrome\Application\chrome.exe'  # Windows: exe path
hotkeys = { "Return" = "action left_click" }
```

**Macros**, written once and callable from any key or from the shell.

```toml
[macros]
click_and_exit = ["action left_click --bail-on-error", "idle"]

[hints.hotkeys]
"Enter" = "macro click_and_exit"
```

**A mode of your own**, as a layer of bare-letter bindings.

```toml
[modes.window]
indicator = "Window"

[modes.window.hotkeys]
"h" = "exec yabai -m window --focus west"
"l" = "exec yabai -m window --focus east"

[hotkeys]
"Primary+Shift+W" = "mode window"
```

**Drive it from anywhere.** Everything above is also a command.

```bash
neru hints --role button --text save   # only buttons whose text matches
neru run "action save_cursor_pos" "hints --action left_click" \
         "action wait_for_mode_exit" "action restore_cursor_pos"
neru macro click_and_exit
neru config set hints.strategy contour
neru doctor                            # diagnose permissions and backends
```

Themes, indicators, smooth cursor and scroll, virtual pointer, app exclusions and screen-share hiding are all options too.

[Configuration Reference](docs/CONFIGURATION.md) · [CLI Reference](docs/CLI.md) · [Tips & Tricks](docs/TIPS_TRICKS.md) · [Community configs](docs/CONFIG_SHOWCASES.md)

---

## How Neru compares

| Tool                                                   | Approach                                        | macOS | Linux | Windows |   Price  | Open source       |
| :----------------------------------------------------- | :---------------------------------------------- | :---: | :---: | :-----: | :------: | :---------------: |
| **Neru**                                               | Hints (AX, OCR, contour) + grid + recursive grid + custom modes | ✅ | ✅ | ✅ | **Free** | ✅ |
| [Homerow](https://www.homerow.app/)                    | AX hints + element search                       |  ✅   |       |         | One-time | ❌                |
| [Wooshy](https://wooshy.app)                           | Search-to-click                                 |  ✅   |       |         | Subscription | ❌ |
| [Mouseless](https://mouseless.click/)                  | Grid + mouse keys                               |  ✅   |  ✅   |   ✅    | Subscription or lifetime | ❌ |
| [Shortcat](https://shortcat.app/)                      | Command palette over UI elements                |  ✅   |       |         | Free     | ❌                |
| [Mousemaster](https://github.com/petoncle/mousemaster) | Grid + hints + continuous pointer + remapping   |  ✅   |       |   ✅    | Free     | ✅                |
| [Scoot](https://github.com/mjrusso/scoot)              | AX hints + grid                                 |  ✅   |       |         | Free     | ✅                |
| [Stochos](https://github.com/museslabs/stochos)        | Grid + hints                                    |  ✅   |  ✅   |         | Free     | ✅                |
| [wl-kbptr](https://github.com/moverest/wl-kbptr)       | Grid + contour hints (Wayland)                  |       |  ✅   |         | Free     | ✅                |
| [warpd](https://github.com/rvaiya/warpd)               | Grid + hints + normal pointer                   |  ✅   |  ✅   |   ✅    | Free     | ✅ (dormant)      |
| [Vimac](https://github.com/nchudleigh/vimac)           | AX hints                                        |  ✅   |       |         | Free     | ✅ (superseded by Homerow) |

Neru is the only free tool in the list that runs on all three platforms.

---

## Community

> _"neru: the ACTUAL BEST app for ditching mouse"_

[![Community YouTube Review](https://img.youtube.com/vi/OFnpYTDA2gY/maxresdefault.jpg)](https://www.youtube.com/watch?v=OFnpYTDA2gY)

- **[Neru Dojo](https://bernatgene.github.io/neru-dojo/)**, a browser game the community built to train recursive grid and hints. No install.
- **[HOW-I-USE-NERU.md](HOW-I-USE-NERU.md)**, the author's daily workflow and why grid won over hints.
- **[Discord](https://discord.gg/KZwnwr9dz6)** for questions and sharing setups.

https://github.com/user-attachments/assets/d99328a6-b5f9-402a-a01b-297da2ffb454

---

## Documentation

| Using Neru                                       |                                                          |
| :----------------------------------------------- | :------------------------------------------------------- |
| [Installation](docs/INSTALLATION.md)             | Every install method, permissions, login services        |
| [CLI Reference](docs/CLI.md)                     | Every command, flag and the IPC protocol                 |
| [Configuration Reference](docs/CONFIGURATION.md) | Every option with defaults and platform support          |
| [Tips & Tricks](docs/TIPS_TRICKS.md)             | Worked recipes                                           |
| [Troubleshooting](docs/TROUBLESHOOTING.md)       | Common issues and fixes                                  |

| Platforms                                      |                                                          |
| :--------------------------------------------- | :------------------------------------------------------- |
| [Cross-Platform Guide](docs/CROSS_PLATFORM.md) | The capability matrix, the single source of truth        |
| [Linux Setup](docs/LINUX_SETUP.md)             | Dependencies, permissions, building                      |
| [Linux Desktops](docs/LINUX_DESKTOPS.md)       | Per-compositor setup and known issues                    |

| Working on Neru                                |                                                          |
| :--------------------------------------------- | :------------------------------------------------------- |
| [Contributing](CONTRIBUTING.md)                | How to propose, commit and land a change                 |
| [Development Guide](docs/DEVELOPMENT.md)       | Setup, building, testing, debugging                      |
| [Architecture](docs/ARCHITECTURE.md)           | Layers, boundaries, platform isolation                   |
| [Roadmap](docs/ROADMAP.md)                     | What is next and where help matters most                 |

---

## Contributing

Neru is Go and Objective-C in a hexagonal layout, and the architecture rules are tests. The guardrails in [`internal/architecture/`](internal/architecture/) pin layering, platform isolation and even doc-link integrity, and each failure message says how to fix it.

```bash
just ci   # the same recipes CI gates on
```

Linux and Windows bug reports currently outrank any new feature. [Contributing Guide](CONTRIBUTING.md)

---

## Support the project

Neru is built by one person in spare time. If it has earned a place in your workflow, [sponsor it](https://github.com/sponsors/y3owk1n).

Only need window management? [mimi](https://github.com/y3owk1n/mimi) is the window and space pieces of Neru as a standalone tool.

## License

MIT. See [LICENSE](LICENSE).

<div align="center">
<br/>

**The mouse is now optional.**

Made with ❤️ by <a href="https://github.com/y3owk1n">y3owk1n</a>

</div>
