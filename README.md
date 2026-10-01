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

<sub>Beta means every option, flag and action already works. Stable comes after six clean releases. [What the labels mean](docs/reference/platform-support.md#what-the-labels-mean)</sub>

[Install](#install) · [Modes](#pick-your-mode) · [Make it yours](#make-it-yours) · [Docs](#documentation)

</div>

---

https://github.com/user-attachments/assets/f4c86753-0109-47c9-a94e-775af2546a17

Neru does what Vimium does in the browser, for every app, window, menu bar and dock item on your screen.

```
Cmd+Shift+Space   labels appear on every clickable element
type the label    cursor jumps there
Shift+L           left click
```

## Why Neru

- **Works where accessibility trees don't.** Grid and recursive grid split pixels, not widgets, so they work in canvases, games, remote desktops and Electron apps with thin trees. Hints have three engines: the accessibility tree, on-device OCR, and a pure-Go contour pass that needs no OCR at all.
- **Latency is the product.** The event tap sits on every keystroke. Anything that makes activation or key handling slower counts as a bug.
- **No per-app hacks.** Good defaults, then per-app overrides you write yourself: hotkeys, hint engine, scroll step, all re-resolved the instant focus changes.
- **CLI-first, scriptable everywhere.** One executor backs hotkeys, the `neru` CLI and the IPC socket. A sequence that works in a binding works from skhd, Hammerspoon, Raycast or a shell script unchanged.
- **Nothing leaves your machine.** OCR and element detection run on-device. No telemetry, no accounts, and typed text never reaches the log.
- **Fails loudly, recovers cleanly.** A typo'd flag in a binding fails config validation instead of doing nothing when you press the key.

---

## Install

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash
```

```powershell
# Windows
irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1 | iex
```

The script installs the binary, man pages and shell completions, offers a login service, and updates in place when you run it again. Homebrew, Nix, prebuilt binaries, building from source and uninstalling are in the [Installation guide](docs/guide/installation.md).

Then run `neru launch` and follow [Getting started](docs/guide/getting-started.md) for permissions, your config file and your first hotkeys.

Prefer an agent? Two skills for Claude Code, Codex and Cursor answer questions and write your config. See [Set up with an agent](docs/guide/installation.md#set-up-with-an-agent).

---

## Pick your mode

<table>
<tr>
<td align="center" width="50%">
<img src="https://github.com/user-attachments/assets/d82e14fa-0ac4-4081-9a0a-f99243d1da5b" alt="Recursive Grid Mode" /><br/>
<sub><b>Recursive Grid</b></sub>
</td>
<td align="center" width="50%">
<img src="https://github.com/user-attachments/assets/a19ef869-6400-4b5f-b3c6-92af69c2a76b" alt="Hints Mode" /><br/>
<sub><b>Hints</b></sub>
</td>
</tr>
<tr>
<td align="center" width="50%">
<img src="https://github.com/user-attachments/assets/913e9d15-7f42-470d-b5cf-7a55065789fe" alt="Bisect Mode" /><br/>
<sub><b>Bisect</b></sub>
</td>
<td align="center" width="50%">
<img src="https://github.com/user-attachments/assets/392fcfa1-9779-464c-8bac-a05a6afc73c3" alt="Grid Mode" /><br/>
<sub><b>Grid</b></sub>
</td>
</tr>
</table>

| Mode                  | Default hotkey        | How it works                                                    | Best for                                    |
| :-------------------- | :-------------------- | :-------------------------------------------------------------- | :------------------------------------------ |
| **Recursive Grid** ⭐ | `Primary+Shift+C`     | Halve the screen with home-row keys until the cursor lands      | Anything, including canvases and games      |
| **Bisect**            | `Primary+Shift+B`     | `hjkl` keep a half, `yubn` a quadrant, until the cursor lands   | Anything, without reading labels            |
| **Hints**             | `Primary+Shift+Space` | Labels every target via accessibility tree, OCR or contour scan | Native apps, Electron, browsers, Figma      |
| **Grid**              | `Primary+Shift+G`     | Type a row and column label to jump to a cell                   | Coarse jumps across big monitors            |
| **Scroll**            | `Primary+Shift+S`     | `j`/`k`, `u`/`d`, `gg`/`G`, `h`/`l` in any scroll view          | Reading without lifting your hands          |
| **Monitor Select**    | unbound               | Labels each display, type one to jump                           | Multi-monitor setups                        |
| **Your own**          | unbound               | A name, an indicator and a key table you declare in config      | Window management layers, app-specific keys |

`Primary` is `Cmd` on macOS and `Ctrl` on Linux and Windows. Linux ships without these global defaults, so they never collide with terminal shortcuts ([bind your own](docs/guide/getting-started.md#binding-your-first-hotkeys)). Every binding is remappable, including for Colemak and Dvorak layouts.

Inside any mode you get the full pointer: left, right and middle click, double and triple click, drag with any button, held-key glide with acceleration, and sticky modifiers you tap instead of hold.

---

## Make it yours

Everything is one TOML file, reloaded with `neru config reload` and changeable one value at a time with `neru config set`. [Getting started](docs/guide/getting-started.md) covers where it lives and how it layers.

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

[Configuration reference](docs/reference/configuration.md) · [CLI reference](docs/reference/cli.md) · [Recipes](docs/guide/recipes.md) · [Community configs](docs/project/showcases.md)

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

**Using Neru**

- [Installation](docs/guide/installation.md) and [Getting started](docs/guide/getting-started.md)
- [Recipes](docs/guide/recipes.md) and [Troubleshooting](docs/guide/troubleshooting.md)
- [Linux setup](docs/guide/linux.md) and [Linux desktops](docs/guide/linux-desktops.md)

**Reference**

- [Configuration](docs/reference/configuration.md), every option with its default
- [CLI](docs/reference/cli.md), every command and flag
- [Scripting and IPC](docs/reference/scripting.md)
- [Platform support](docs/reference/platform-support.md), what works where

**Working on Neru**

- [Contributing](CONTRIBUTING.md), [Development](docs/contributing/development.md), [Architecture](docs/contributing/architecture.md)
- [Roadmap](docs/project/roadmap.md)

---

## Contributing

Bug reports, especially on Linux and Windows, and pull requests are welcome. Start with the [Contributing guide](CONTRIBUTING.md).

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
