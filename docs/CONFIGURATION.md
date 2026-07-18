# Configuration Guide

Neru uses TOML for configuration. No config file is required — Neru works out of the box with sensible defaults. Only define the options you want to change; all other defaults are preserved automatically.

> "The daemon" refers to the background process started with `neru launch`.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Config File Location](#config-file-location)
- [Managing Your Config](#managing-your-config)
- [Color Format](#color-format)
- [Hotkeys](#hotkeys)
- [General](#general)
- [Theme](#theme)
- [Hints](#hints)
- [Grid](#grid)
- [Recursive Grid](#recursive_grid)
- [Scroll](#scroll)
- [Monitor Select](#monitor_select)
- [Virtual Pointer](#virtual_pointer)
- [Mouse Action Indicator](#mouse_action_indicator)
- [Mode Indicator](#mode_indicator)
- [Sticky Modifiers](#sticky_modifiers)
- [Per-App Global Hotkey Overrides](#per-app-global-hotkey-overrides)
- [Smooth Cursor](#smooth_cursor)
- [Smooth Scroll](#smooth_scroll)
- [Held Repeat](#held_repeat)
- [Systray](#systray)
- [Logging](#logging)

---

## Quick Start

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]

[scroll]
scroll_step = 50
```

Generate a fully-commented starter file:

```bash
neru config init                          # Creates ~/.config/neru/config.toml
neru config init --force                  # Overwrite existing
neru config init -c /path/to/config.toml  # Custom path
```

---

## Config File Location

> **Recommended:** `~/.config/neru/config.toml`

Loaded in priority order (highest first):

1. `$XDG_CONFIG_HOME/neru/config.toml`
2. `~/.config/neru/config.toml`
3. `~/.neru.toml` (legacy)
4. `neru.toml` (current directory)
5. `config.toml` (current directory)

Override at launch: `neru launch -c /path/to/config.toml`

---

## Managing Your Config

```bash
neru config validate    # Check syntax (no daemon needed)
neru config reload      # Apply changes to running daemon
neru config dump        # Print loaded config as JSON (daemon required)
neru config init        # Create default config file
```

See [CLI.md](CLI.md#configuration-management) for full flag documentation.

---

## Color Format

Colors use hex notation with optional alpha transparency.

| Format      | Example     | Alpha | Notes              |
| ----------- | ----------- | ----- | ------------------ |
| `#AARRGGBB` | `#FF000000` | Yes   | Recommended format |
| `#RRGGBB`   | `#FF0000`   | No    | Fully opaque       |
| `#RGB`      | `#F00`      | No    | Shorthand          |

### Alpha Reference

| Opacity | Hex  | Common Use                  |
| ------- | ---- | --------------------------- |
| 100%    | `FF` | Solid colors, high contrast |
| 95%     | `F2` | Hint labels (default)       |
| 70%     | `B3` | Grid cell backgrounds       |
| 60%     | `99` | Grid borders                |
| 30%     | `4D` | Subtle highlights           |
| 0%      | `00` | Invisible                   |

Calculate: `round(opacity_fraction × 255)` → hex.

### Light / Dark Mode

Colors can be a single string or a dictionary with `light` / `dark` keys:

```toml
# Same for both themes
background_color = "#FF0000AA"

# Per-theme
background_color = { light = "#FF0000AA", dark = "#00FF00AA" }
```

Omitted colors inherit Neru's theme-derived defaults and update in real time when you switch system themes.

---

## Hotkeys

### Global Hotkeys

```toml
[hotkeys]
"Primary+Shift+Space" = "hints"
```

**Syntax:** `"Mod1+Mod2+Key" = "action"`

| Modifier  | Aliases                                 |
| --------- | --------------------------------------- |
| `Cmd`     | `Command`, `Super`, `Meta`              |
| `Ctrl`    | `Control`                               |
| `Alt`     | `Option`                                |
| `Shift`   |                                         |
| `Primary` | `Cmd` on macOS, `Ctrl` on Linux/Windows |

**Available keys** (the `Key` part after modifiers):

| Category   | Keys                                                               |
| ---------- | ------------------------------------------------------------------ |
| Letters    | `a`–`z`, `A`–`Z`                                                   |
| Numbers    | `0`–`9`                                                            |
| Symbols    | `` ` ``, `-`, `=`, `[`, `]`, `\`, `;`, `'`, `,`, `.`, `/`          |
| Named      | `Space`, `Return`, `Enter`, `Escape`, `Tab`, `Delete`, `Backspace` |
| Navigation | `Up`, `Down`, `Left`, `Right`, `Home`, `End`, `PageUp`, `PageDown` |
| Function   | `F1`–`F20`                                                         |

See [CLI.md](CLI.md#feed-keys) for a full key reference with key codes and platform behavior.

Multi-key sequences (e.g. `gg`, `ab`) are supported for per-mode hotkeys with a 500ms timeout.

**Action values** can be a single string or an array:

```toml
[hotkeys]
"Primary+Shift+D" = ["hints", "exec echo 'hints activated'"]
"PageUp"          = ["action go_top", "action page_down"]
```

**Shell commands** use the `exec` prefix: `"Primary+T" = "exec open -a Terminal"`

#### Merging Behavior

| Config                 | Result                    |
| ---------------------- | ------------------------- |
| Section absent         | All defaults used         |
| Section present, empty | All hotkeys disabled      |
| Section has entries    | Merged on top of defaults |

Use `__disabled__` to remove individual defaults:

```toml
[hotkeys]
"Primary+Shift+S" = "__disabled__"   # removes default scroll binding
"Ctrl+Space"      = "hints"          # adds binding; other defaults unchanged
```

When a mode is disabled (`enabled = false`), its default launcher hotkey is removed automatically.

**Mode toggling:** Append `--toggle` to turn a hotkey into a toggle — activates the mode on first press, exits to idle on the second. Works with any mode: `"Ctrl+F" = "grid --toggle"`.

#### Per-App Global Hotkey Overrides

`[[app_configs]]` overrides `[hotkeys]` bindings for specific apps. Use this when you want different launcher hotkeys depending on which app is focused.

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints"

[[app_configs]]
bundle_id = "com.apple.Terminal"
hotkeys = {
    "Cmd+Space" = "hints",
    "Cmd+Shift+Space" = "__disabled__"
}

[[app_configs]]
bundle_id = "com.apple.Safari"
hotkeys = {
    "Cmd+Shift+F" = "hints",
    "Cmd+Shift+Space" = "__disabled__"
}
```

The same [merging rules](#merging-behavior) apply: app hotkeys merge on top of the base `[hotkeys]` bindings, and `__disabled__` removes an inherited binding. When no `[[app_configs]]` entry matches the focused app, the base `[hotkeys]` bindings are used as-is.

### Per-Mode Hotkeys

Each mode can define hotkeys active only while that mode is running. Follows the same [merging rules](#merging-behavior) as global hotkeys.

```toml
[hints.hotkeys]
"Escape"    = "idle"
"Backspace" = "action backspace"
"Shift+L"   = ["action left_click", "idle"]

[scroll.hotkeys]
"gg"                   = "action go_top"      # two-letter sequence
"Primary+Shift+T"      = "exec open -a Terminal"
```

Multi-key alphabetic sequences (e.g. `gg`) use a 500ms timeout.

#### Per-App Hotkey Overrides

Both global and per-mode hotkeys support per-app overrides via `[[app_configs]]` and `[[<mode>.app_configs]]`.

- Global: `[[app_configs]]` overrides `[hotkeys]` bindings for specific apps
- Per-mode: `[[<mode>.app_configs]]` overrides `<mode>.hotkeys` for specific apps

Supported modes for per-mode overrides are `hints`, `grid`, `recursive_grid`, and `scroll`. App hotkeys merge on top of base hotkeys; `__disabled__` removes an inherited binding.

```toml
# Per-app global hotkey overrides (root-level)
[[app_configs]]
bundle_id = "com.apple.Terminal"
hotkeys = {
    "Cmd+Space" = "hints",
    "Cmd+Shift+Space" = "__disabled__"
}

# Per-app mode hotkey overrides
[[hints.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = {
    "Return" = "action left_click",
    "Shift+L" = "__disabled__"
}
```

**Priority order** when a key is pressed while Neru is running:

| Context | Resolution |
|---|---|
| **Idle (no mode active)** | `[hotkeys]` bindings merged with per-app `[[app_configs]]` overrides for the focused app |
| **Inside a mode** | `[<mode>.hotkeys]` merged with per-app `[[<mode>.app_configs]]` overrides, checked before the mode's built-in keys |
| **Global hotkey conflicts with mode hotkey** | Mode hotkey override wins (e.g., a global `Cmd+Shift+F = "hints"` launcher is replaced by `[hints.hotkeys]` `"Cmd+Shift+F" = "recursive_grid"` while hints mode is active) |

Inside a mode, the dispatch order is:

1. Modifier toggle
2. `<mode>.hotkeys` + per-app overrides
3. Mode built-in keys (hint/grid character input)

### Action Reference

All actions available in hotkeys. These also work as `neru action <name>` — see [CLI.md](CLI.md#action-commands) for full flag documentation.

| Category    | Actions                                                                               |
| ----------- | ------------------------------------------------------------------------------------- |
| Click       | `left_click`, `right_click`, `middle_click`                                           |
| Mouse       | `mouse_down`, `mouse_up`, `move_mouse`, `move_mouse_relative`                         |
| Scroll      | `scroll_up`, `scroll_down`, `scroll_left`, `scroll_right`                             |
| Page        | `page_up`, `page_down`, `go_top`, `go_bottom`                                         |
| Keyboard    | `feed`                                                                                |
| Hints       | `search_hints`, `cycle_hint`, `cycle_hint --backward`                                 |
| Delay       | `sleep <duration>` — plain numbers are seconds (`0.5`), explicit units: `500ms`, `1s` |
| Mode        | `reset`, `backspace`                                                                  |
| Composition | `wait_for_mode_exit` (with optional `--bail`), `save_cursor_pos`, `restore_cursor_pos` |

- Use `--bare` (e.g. `"action left_click --bare"`) to target the cursor position instead of the current mode selection (see [CLI.md](CLI.md#clicks))
- `scroll_up` / `scroll_down` support `--steps` (e.g. `"action scroll_down --steps 200"`) to override `scroll_step` (see [CLI.md](CLI.md#scrolling))
- `reset`, `backspace`, `search_hints`, `cycle_hint`, `sleep`, `wait_for_mode_exit`, `save_cursor_pos`, and `restore_cursor_pos` are not valid mode `--action` values — use `neru action ...` or in hotkeys as `"action ..."`

#### Feed Keys

```toml
[hotkeys]
"Primary+Y"       = "action feed h e l l o return"
"Primary+Shift+C" = "action feed ctrl+c"

[hints.hotkeys]
"o"               = ["idle", "action feed o"]

# Feed into Neru's own mode system (--mode)
"Cmd+3"           = [
    "hints --role AXRadioButton --text design --action left_click",
    "action feed --mode a",
]
```

Use `--mode` to route keys through Neru's active mode/action pipeline instead of the OS. See [CLI.md](CLI.md#feed-keys) for syntax, supported key names, and platform behavior.

#### Composition Example

```toml
[hints.hotkeys]
"Enter"  = ["action save_cursor_pos", "idle", "action wait_for_mode_exit", "action restore_cursor_pos"]
"Return" = ["action left_click", "action sleep 0.5", "hints"]

# Bail: abort the chain if the user cancels (Escape) instead of making a selection
"Ctrl+Z" = ["monitor_select", "action wait_for_mode_exit --bail", "recursive_grid"]
```

Use `--bail` to abort the chain when the mode exits without a selection (e.g., user presses Escape). Without `--bail`, `wait_for_mode_exit` always succeeds and the chain continues.

---

## [general]

| Option                                 | Type   | Default       | Description                                                                                       |
| -------------------------------------- | ------ | ------------- | ------------------------------------------------------------------------------------------------- |
| `excluded_apps`                        | array  | `[]`          | Bundle IDs where Neru won't activate                                                              |
| `kb_layout_to_use`                     | string | `""`          | Force keyboard layout InputSourceID bundle ID (auto if empty). E.g. `com.apple.keylayout.Colemak` |
| `hide_overlay_in_screen_share`         | bool   | `false`       | Hide overlay in screen sharing apps                                                               |
| `passthrough_unbounded_keys`           | bool   | `false`       | Let unbound Cmd/Ctrl/Alt shortcuts pass through                                                   |
| `should_exit_after_passthrough`        | bool   | `false`       | Exit mode after a passthrough shortcut                                                            |
| `passthrough_unbounded_keys_blacklist` | array  | `[]`          | Shortcuts to keep consumed when passthrough is on                                                 |
| `exec_shell`                           | string | `"/bin/bash"` | Shell binary used for `exec` hotkey commands                                                      |
| `exec_shell_args`                      | array  | `["-lc"]`     | Shell arguments; command string is appended last                                                  |

Find available `kb_layout_to_use` IDs on macOS:

```bash
# get all enabled input sources
defaults read com.apple.HIToolbox AppleEnabledInputSources

# get the current keyboard layout that is active (e.g. if you use dvorak, it should be `com.apple.keylayout.Dvorak`)
defaults read com.apple.HIToolbox AppleCurrentKeyboardLayoutInputSourceID
```

---

## [theme]

Base colors used to derive all component defaults. Use solid `#RRGGBB` or `#RGB` (no alpha).

| Key             | Role                                                |
| --------------- | --------------------------------------------------- |
| `surface`       | Translucent fills, badges, indicator backgrounds    |
| `accent`        | Borders, lines, primary chrome                      |
| `accent_alt`    | Active/emphasis states, highlights, virtual pointer |
| `on_accent_alt` | Foreground text/icon on `accent_alt` surfaces       |
| `text`          | Foreground text on `surface` backgrounds            |

```toml
[theme.light]
surface       = "#EEF2FF"
accent        = "#465FBC"
accent_alt    = "#0B2377"
on_accent_alt = "#F8FAFF"
text          = "#17327A"

[theme.dark]
surface       = "#0A1338"
accent        = "#6E82D6"
accent_alt    = "#8FA2F0"
on_accent_alt = "#081022"
text          = "#E8EEFF"
```

Explicit component colors override theme derivation. Omitted colors inherit from the palette.

---

## [hints]

Labels clickable UI elements with short overlay labels. By default uses the macOS Accessibility API (`axtree` strategy). Optionally uses Vision Framework (`vision` strategy) for apps with poor AX trees — detects elements via screen capture + text/rectangle recognition scoped to the focused window.

Press `/` to text-search elements. `Space` for multi-word queries. `Return` confirms filtered hints (first is auto-selected). `Escape` cancels search.

Start with search visible: `neru hints --search` (see [CLI.md](CLI.md#hints-mode))

### Options

| Option                             | Type         | Default                 | Description                                                                                                                                                                                                                                                                                                                          |
| ---------------------------------- | ------------ | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `enabled`                          | bool         | `true`                  | Enable/disable hints mode                                                                                                                                                                                                                                                                                                            |
| `strategy`                         | string       | `"axtree"`              | Element detection strategy: `"axtree"` (macOS Accessibility API) or `"vision"` (Vision Framework). Vision mode detects the frontmost window content via screen capture + text/rectangle recognition while still using AX for system elements (menubar, dock, NC). Overridable per-app via `[hints.app_configs]`.                     |
| `hint_characters`                  | string       | `"asdfghjkl"`           | Characters used for labels                                                                                                                                                                                                                                                                                                           |
| `label_direction`                  | string       | `"normal"`              | Hint label algorithm: `"normal"` (default, prefix-avoidance greedy) or `"reverse"` (reverse-order tiers). Empty value defaults to `"normal"`. Overridable per-app via `[hints.app_configs]` and per-activation via the `neru hints --label-direction` CLI flag. See [Choosing a label direction](#choosing-a-label-direction) below. |
| `max_depth`                        | int          | `50`                    | Max accessibility tree depth (0 = unlimited)                                                                                                                                                                                                                                                                                         |
| `include_menubar_hints`            | bool         | `false`                 | Show hints on menubar items                                                                                                                                                                                                                                                                                                          |
| `include_dock_hints`               | bool         | `false`                 | Show hints on Dock items                                                                                                                                                                                                                                                                                                             |
| `include_nc_hints`                 | bool         | `false`                 | Show hints in Notification Center                                                                                                                                                                                                                                                                                                    |
| `include_stage_manager_hints`      | bool         | `false`                 | Show hints in Stage Manager                                                                                                                                                                                                                                                                                                          |
| `include_pip_hints`                | bool         | `false`                 | Show hints on Picture in Picture controls                                                                                                                                                                                                                                                                                            |
| `include_screen_capture_hints`     | bool         | `false`                 | Show hints on Screen Capture controls                                                                                                                                                                                                                                                                                                |
| `detect_mission_control`           | bool         | `false`                 | Enable Mission Control state detection                                                                                                                                                                                                                                                                                               |
| `on_mission_control_activated`     | string/array | `nil`                   | Action(s) to execute when Mission Control opens                                                                                                                                                                                                                                                                                      |
| `on_mission_control_deactivated`   | string/array | `nil`                   | Action(s) to execute when Mission Control closes                                                                                                                                                                                                                                                                                     |
| `additional_menubar_hints_targets` | array        | macOS-specific defaults | Extra menubar bundle IDs                                                                                                                                                                                                                                                                                                             |
| `clickable_roles`                  | array        | macOS-specific defaults | AX roles that generate hints                                                                                                                                                                                                                                                                                                         |
| `ignore_clickable_check`           | bool         | `false`                 | Skip clickability heuristic                                                                                                                                                                                                                                                                                                          |
| `visible_check_enabled`            | bool         | `false`                 | Enable visibility hit-test (slower but fewer noisy hints)                                                                                                                                                                                                                                                                            |

### UI

| Option               | Type   | Default    | Description                                |
| -------------------- | ------ | ---------- | ------------------------------------------ |
| `font_size`          | int    | `10`       | Font size in points                        |
| `font_family`        | string | `""`       | Font family (empty = system default)       |
| `border_radius`      | int    | `-1`       | Corner radius (-1 = auto)                  |
| `padding_x`          | int    | `-1`       | Horizontal padding (-1 = auto)             |
| `padding_y`          | int    | `-1`       | Vertical padding (-1 = auto)               |
| `border_width`       | int    | `1`        | Border width in pixels                     |
| `placement`          | string | `"bottom"` | Label placement: `top`, `center`, `bottom` |
| `background_color`   | color  | derived    | Background color                           |
| `text_color`         | color  | derived    | Text color                                 |
| `matched_text_color` | color  | derived    | Text color for matched characters          |
| `border_color`       | color  | derived    | Border color                               |

```toml
[hints.ui]
font_size = 10
border_radius = -1
padding_x = -1
padding_y = -1
border_width = 1
placement = "bottom"
```

### Boundary Highlight

Optional element outlines for dense layouts. Off by default.

| Option             | Type  | Default | Description                    |
| ------------------ | ----- | ------- | ------------------------------ |
| `enabled`          | bool  | `false` | Draw element boundaries        |
| `border_width`     | int   | `1`     | Stroke width in pixels         |
| `border_radius`    | int   | `-1`    | Corner radius (-1 = auto pill) |
| `background_color` | color | derived | Element fill color             |
| `border_color`     | color | derived | Element stroke color           |

```toml
[hints.boundary_highlight]
enabled = false
border_width = 1
border_radius = -1
```

### Search Input UI

| Option     | Type   | Default           | Description                                                                                             |
| ---------- | ------ | ----------------- | ------------------------------------------------------------------------------------------------------- |
| `position` | string | `"bottom_center"` | Anchor: `top_left`, `top_center`, `top_right`, `center`, `bottom_left`, `bottom_center`, `bottom_right` |
| `x_offset` | int    | `0`               | Horizontal offset from anchor                                                                           |
| `y_offset` | int    | `24`              | Vertical offset from anchor                                                                             |
| `width`    | int    | `320`             | Width in pixels                                                                                         |

Also supports all [hints UI](#ui) visual options except `matched_text_color`.

```toml
[hints.search_input_ui]
position = "bottom_center"
x_offset = 0
y_offset = 24
width = 320
```

### Vision

Tunable settings for Vision-based hint detection (only used when `hints.strategy` or the app-specific `strategy` override is set to `"vision"`).

| Option                             | Type  | Default | Description                                                                                       |
| ---------------------------------- | ----- | ------- | ------------------------------------------------------------------------------------------------- |
| `detect_text`                      | bool  | `true`  | Enable text detection via the Vision framework.                                                   |
| `detect_rectangles`                | bool  | `true`  | Enable rectangle detection via the Vision framework.                                              |
| `request_timeout_ms`               | int   | `5000`  | Timeout in milliseconds for Vision framework analysis requests.                                   |
| `minimum_confidence`               | float | `0.0`   | Minimum confidence score (0.0 to 1.0) for keeping Vision framework observations.                  |
| `merge_iou_threshold`              | float | `0.5`   | Intersection-over-Union (IoU) overlap threshold for merging redundant overlapping bounding boxes. |
| `rectangle_max_candidates`         | int   | `100`   | Maximum number of rectangle candidate observations to evaluate.                                   |
| `rectangle_min_size`               | float | `0.01`  | Minimum normalized size of detected rectangles (e.g. `0.01` is 1% of screen/window dimensions).   |
| `rectangle_min_aspect`             | float | `0.3`   | Minimum aspect ratio (width/height) for rectangle elements.                                       |
| `rectangle_max_aspect`             | float | `10.0`  | Maximum aspect ratio (width/height) for rectangle elements.                                       |
| `button_min_confidence`            | float | `0.3`   | Minimum confidence score threshold for classifying a rectangle as a button.                       |
| `button_min_aspect`                | float | `0.8`   | Minimum aspect ratio for button elements.                                                         |
| `button_max_aspect`                | float | `8.0`   | Maximum aspect ratio for button elements.                                                         |
| `button_icon_max_size`             | int   | `48`    | Maximum width/height in pixels for square button or icon elements.                                |
| `link_min_aspect`                  | float | `5.0`   | Minimum aspect ratio for text link elements.                                                      |
| `link_max_height`                  | int   | `40`    | Maximum height in pixels for text link elements.                                                  |
| `link_min_width`                   | int   | `50`    | Minimum width in pixels for text link elements.                                                   |
| `image_min_size`                   | int   | `48`    | Minimum width/height in pixels for image elements.                                                |
| `checkbox_max_size`                | int   | `32`    | Maximum width/height in pixels for checkbox elements.                                             |
| `generic_clickable_min_confidence` | float | `0.5`   | Minimum confidence threshold for generic clickable elements.                                      |

```toml
[hints.vision]
detect_text = true
detect_rectangles = true
request_timeout_ms = 5000
minimum_confidence = 0.0
merge_iou_threshold = 0.5
rectangle_max_candidates = 100
rectangle_min_size = 0.01
rectangle_min_aspect = 0.3
rectangle_max_aspect = 10.0
button_min_confidence = 0.3
button_min_aspect = 0.8
button_max_aspect = 8.0
button_icon_max_size = 48
link_min_aspect = 5.0
link_max_height = 40
link_min_width = 50
image_min_size = 48
checkbox_max_size = 32
generic_clickable_min_confidence = 0.5
```

### Choosing a label direction

The `label_direction` setting controls how multi-character hint labels are enumerated once the single-character pool is exhausted. With a 4-character alphabet (`asdf`) and 5 hinted elements, the two algorithms produce visibly different label sequences:

| Direction          | Sequence         | Notes                                                                              |
| ------------------ | ---------------- | ---------------------------------------------------------------------------------- |
| `normal` (default) | `A S D FA FS`    | Keeps 3 single-char labels, then expands the 4th alphabet slot into 2-char labels. |
| `reverse`          | `AA SA DA FA AS` | Fills the 2-char tier uniformly from the first alphabet character.                 |

**When to prefer `normal` (default):**

- Most workflows — fewer keystrokes for the common case where 1- or 2-character labels are enough.
- Hint characters are scarce (e.g. a 2- or 3-character alphabet), so single-char labels stay usable longer.

**When to prefer `reverse`:**

- Many hints clustered in one region of the screen. `reverse` spreads the _first_ character of each label evenly across the alphabet, so labels rarely share a prefix and the hint key (the visible character) is less likely to be occluded by another element.
- Workflows that consistently need more than `len(hint_characters)` hints.

You can also mix directions per-app via `[hints.app_configs]` or per-activation via `neru hints --label-direction`. See the [per-app config table](#per-app-config) and [CLI reference](CLI.md#hints-mode).

### Per-App Config

| Field                        | Type   | Description                                                                                                                                                                               |
| ---------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bundle_id`                  | string | App bundle ID                                                                                                                                                                             |
| `strategy`                   | string | Override element detection strategy for this app (`"axtree"` or `"vision"`). Empty string = use global `hints.strategy`.                                                                  |
| `label_direction`            | string | Override hint label algorithm for this app (`"normal"` or `"reverse"`). Empty string = use global `hints.label_direction`. See [Choosing a label direction](#choosing-a-label-direction). |
| `additional_clickable_roles` | array  | Extra AX roles to treat as clickable                                                                                                                                                      |
| `ignore_clickable_check`     | bool   | Skip clickability heuristic for this app                                                                                                                                                  |
| `visible_check_enabled`      | bool   | Enable visibility hit-test for this app                                                                                                                                                   |
| `hotkeys`                    | map    | [per-app hotkey overrides](#per-app-hotkey-overrides)                                                                                                                                     |

```toml
[[hints.app_configs]]
bundle_id = "com.apple.Safari"
strategy = "vision"
label_direction = "reverse"
additional_clickable_roles = ["AXLink"]
ignore_clickable_check = true
visible_check_enabled = true
```

---

## [grid]

Divides the screen into a labelled coordinate grid.

Cursor behavior is chosen per invocation: `neru grid --cursor-selection-mode follow|hold` (see [CLI.md](CLI.md#grid-mode)). Default hotkeys include `` ` `` for `toggle-cursor-follow-selection`.

### Options

| Option              | Type   | Default                       | Description                 |
| ------------------- | ------ | ----------------------------- | --------------------------- |
| `enabled`           | bool   | `true`                        | Enable/disable grid mode    |
| `characters`        | string | `"abcdefghijklmnpqrstuvwxyz"` | Primary grid labels         |
| `sublayer_keys`     | string | same as `characters`          | Subgrid labels              |
| `row_labels`        | string | `""`                          | Custom row labels           |
| `col_labels`        | string | `""`                          | Custom column labels        |
| `live_match_update` | bool   | `true`                        | Highlight cells as you type |
| `hide_unmatched`    | bool   | `true`                        | Hide non-matching cells     |
| `prewarm_enabled`   | bool   | `true`                        | Pre-compute grid on startup |
| `enable_gc`         | bool   | `false`                       | Periodic memory cleanup     |

### UI

| Option                     | Type   | Default | Description                          |
| -------------------------- | ------ | ------- | ------------------------------------ |
| `font_size`                | int    | `10`    | Font size in points                  |
| `font_family`              | string | `""`    | Font family (empty = system default) |
| `border_width`             | int    | `1`     | Border width in pixels               |
| `background_color`         | color  | derived | Cell background                      |
| `text_color`               | color  | derived | Label text                           |
| `matched_text_color`       | color  | derived | Matched cell text                    |
| `matched_background_color` | color  | derived | Matched cell background              |
| `matched_border_color`     | color  | derived | Matched cell border                  |
| `border_color`             | color  | derived | Default cell border                  |

```toml
[grid.ui]
font_size = 10
border_width = 1
```

### Per-App Config

```toml
[[grid.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = { "Return" = "action left_click" }
```

See [per-app hotkey overrides](#per-app-hotkey-overrides).

---

## [recursive_grid]

Narrows the active area with each keypress for precise cursor placement.

Cursor behavior: `neru recursive_grid --cursor-selection-mode follow|hold` (see [CLI.md](CLI.md#recursive-grid-mode)). Default hotkeys include `` ` `` for `toggle-cursor-follow-selection`.

### Options

| Option            | Type   | Default       | Description                                                      |
| ----------------- | ------ | ------------- | ---------------------------------------------------------------- |
| `enabled`         | bool   | `true`        | Enable/disable mode                                              |
| `grid_cols`       | int    | `3`           | Columns (≥ 1; total cells ≥ 2)                                   |
| `grid_rows`       | int    | `3`           | Rows (≥ 1; total cells ≥ 2)                                      |
| `keys`            | string | `"rtyfghvbn"` | Cell selection keys (must be `grid_cols × grid_rows` characters) |
| `min_size_width`  | int    | `1`           | Minimum cell width in pixels                                     |
| `min_size_height` | int    | `1`           | Minimum cell height in pixels                                    |
| `max_depth`       | int    | `10`          | Maximum recursion levels (1–20)                                  |
| `layers`          | array  | `[]`          | Per-depth layout overrides (see below)                           |

#### Layers

Each entry overrides the grid dimensions and keys for a specific depth:

| Field       | Type   | Default        | Description                                 |
| ----------- | ------ | -------------- | ------------------------------------------- |
| `depth`     | int    | required       | Recursion depth to override (0 based index) |
| `grid_cols` | int    | same as parent | Columns at this depth                       |
| `grid_rows` | int    | same as parent | Rows at this depth                          |
| `keys`      | string | same as parent | Selection keys at this depth                |

```toml
[recursive_grid]
layers = [
  { depth = 0, grid_cols = 2, grid_rows = 2, keys = "crtn," },
  { depth = 1, grid_cols = 3, grid_rows = 3, keys = "gcrhtnmwv" },
]
```

### Animation

| Option        | Type | Default | Description                                     |
| ------------- | ---- | ------- | ----------------------------------------------- |
| `enabled`     | bool | `true`  | Native depth transitions on supported platforms |
| `duration_ms` | int  | `50`    | Transition duration in milliseconds             |

### UI

| Option                                | Type   | Default | Description                          |
| ------------------------------------- | ------ | ------- | ------------------------------------ |
| `font_size`                           | int    | `10`    | Font size                            |
| `font_family`                         | string | `""`    | Font family (empty = system default) |
| `line_width`                          | int    | `1`     | Grid line width                      |
| `line_color`                          | color  | derived | Grid line color                      |
| `highlight_color`                     | color  | derived | Selected cell highlight              |
| `text_color`                          | color  | derived | Label text                           |
| `label_background`                    | bool   | `false` | Background behind labels             |
| `label_background_color`              | color  | derived | Label background                     |
| `label_background_padding_x`          | int    | `-1`    | Horizontal label padding (-1 = auto) |
| `label_background_padding_y`          | int    | `-1`    | Vertical label padding (-1 = auto)   |
| `label_background_border_radius`      | int    | `-1`    | Label corner radius (-1 = auto)      |
| `label_background_border_width`       | int    | `1`     | Label border width                   |
| `label_char`                          | string | `""`    | Override all cell labels with a single character (e.g. `·`); empty = use key |
| `label_autohide_multiplier`           | float  | `1.5`   | Hide labels when cell < fontSize × multiplier (0 = disable) |
| `sub_key_preview`                     | bool   | `false` | Show sub-key previews in cells       |
| `sub_key_preview_font_size`           | int    | `8`     | Sub-key preview font size            |
| `sub_key_preview_autohide_multiplier` | float  | `1.5`   | Autohide threshold multiplier        |
| `sub_key_preview_text_color`          | color  | derived | Sub-key preview text color           |
| `sub_key_preview_label_char`          | string | `""`    | Override sub-key labels with a single character (e.g. `·`); empty = use key |

```toml
[recursive_grid.ui]
line_width = 1
font_size = 10
label_background = false
sub_key_preview = false
```

### Per-App Config

```toml
[[recursive_grid.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = { "u" = "action left_click" }
```

See [per-app hotkey overrides](#per-app-hotkey-overrides).

---

## [scroll]

Keyboard-driven scrolling.

### Options

| Option             | Type | Default   | Description                                                                                     |
| ------------------ | ---- | --------- | ----------------------------------------------------------------------------------------------- |
| `scroll_step`      | int  | `50`      | Pixels per line scroll action                                                                   |
| `scroll_step_half` | int  | `500`     | Pixels per half-page action                                                                     |
| `scroll_step_full` | int  | `1000000` | Pixels for top/bottom jump actions                                                              |
| `invert_scroll`    | bool | `false`   | Invert scroll direction (useful when using tools like Mos that reverse synthetic scroll events) |

### Default Hotkeys

```toml
[scroll.hotkeys]
"Escape"  = "idle"
"k"       = "action scroll_up"
"j"       = "action scroll_down"
"h"       = "action scroll_left"
"l"       = "action scroll_right"
"gg"      = "action go_top"
"Shift+G" = "action go_bottom"
"u"       = "action page_up"
"PageUp"  = "action page_up"
"d"       = "action page_down"
"PageDown"= "action page_down"
```

### Per-App Config

| Field              | Type   | Description                                           |
| ------------------ | ------ | ----------------------------------------------------- |
| `bundle_id`        | string | App bundle ID                                         |
| `scroll_step`      | int    | Optional app-specific scroll step override            |
| `scroll_step_half` | int    | Optional app-specific scroll step half override       |
| `scroll_step_full` | int    | Optional app-specific scroll step full override       |
| `hotkeys`          | map    | [per-app hotkey overrides](#per-app-hotkey-overrides) |

```toml
[[scroll.app_configs]]
bundle_id = "com.apple.Safari"
scroll_step = 25
scroll_step_half = 200
scroll_step_full = 1000
hotkeys = { "k" = "action scroll_up", "j" = "action scroll_down" }
```

---

## [monitor_select]

Interactive display picking mode. Shows per-monitor overlay badges labelled with selectable characters. Monitors are sorted in a fixed spatial order (top-to-bottom, left-to-right).

| Option                 | Type   | Default       | Description                                         |
| ---------------------- | ------ | ------------- | --------------------------------------------------- |
| `enabled`              | bool   | `false`       | Enable interactive monitor picking                  |
| `characters`           | string | `"123456789"` | Characters used for monitor labels                  |

### UI

| Key                        | Default       | Description                        |
| -------------------------- | ------------- | ---------------------------------- |
| `font_size`                | `96`          | Badge label font size              |
| `font_family`              | `""` (system) | Badge label font family            |
| `subtitle_font_size`       | `18`          | Monitor name subtitle font size    |
| `subtitle_font_family`     | `""` (system) | Subtitle font family               |
| `border_radius`            | `-1` (auto)   | Badge corner radius                |
| `padding_x`                | `-1` (auto)   | Horizontal padding                 |
| `padding_y`                | `-1` (auto)   | Vertical padding                   |
| `border_width`             | `0`           | Badge border width                 |
| `background_color`         | derived       | Badge fill color                   |
| `text_color`               | derived       | Label text color                   |
| `matched_text_color`       | derived       | Partially-typed label text color   |
| `border_color`             | derived       | Badge border color                 |
| `backdrop_color`           | `""` (none)   | Per-monitor overlay backdrop tint  |
| `subtitle_text_color`      | derived       | Subtitle text color                |

### Hotkeys

| Key      | Default | Description               |
| -------- | ------- | ------------------------- |
| `Escape` | `idle`  | Cancel and return to idle |

```toml
[monitor_select]
enabled = false
characters = "123456789"

[monitor_select.ui]
font_size = 96
font_family = ""
subtitle_font_size = 18
subtitle_font_family = ""
border_radius = -1
padding_x = -1
padding_y = -1
border_width = 0
backdrop_color = ""

[monitor_select.hotkeys]
"Escape" = "idle"
```

---

## [virtual_pointer]

A small dot rendered at the active selection when grid or recursive-grid runs in `--cursor-selection-mode hold`.

| Option    | Type  | Default | Description          |
| --------- | ----- | ------- | -------------------- |
| `enabled` | bool  | `true`  | Enable/disable       |
| `size`    | int   | `3`     | Dot radius in points |
| `color`   | color | derived | Dot color            |

```toml
[virtual_pointer]
enabled = true

[virtual_pointer.ui]
size = 3
```

---

## [mouse_action_indicator]

Transient visual marker at mouse action locations. macOS only; other platforms accept the config and no-op.

| Option    | Type     | Default                                                                   | Description        |
| --------- | -------- | ------------------------------------------------------------------------- | ------------------ |
| `enabled` | bool     | `false`                                                                   | Enable indicators  |
| `actions` | string[] | `["left_click", "right_click", "middle_click", "mouse_down", "mouse_up"]` | Triggering actions |

### UI

| Option             | Type   | Default    | Description          |
| ------------------ | ------ | ---------- | -------------------- |
| `size`             | int    | `36`       | Diameter in points   |
| `border_width`     | int    | `2`        | Border width         |
| `background_color` | color  | derived    | Fill color           |
| `border_color`     | color  | derived    | Stroke color         |
| `shape`            | string | `"circle"` | `circle` or `square` |

### Animation

| Option          | Type   | Default      | Description                                    |
| --------------- | ------ | ------------ | ---------------------------------------------- |
| `duration_ms`   | int    | `260`        | Animation duration in ms                       |
| `start_scale`   | float  | `0.55`       | Starting scale                                 |
| `end_scale`     | float  | `1.35`       | Ending scale                                   |
| `start_opacity` | float  | `0.85`       | Starting opacity                               |
| `end_opacity`   | float  | `0.0`        | Ending opacity                                 |
| `easing`        | string | `"ease_out"` | `linear`, `ease_in`, `ease_out`, `ease_in_out` |

```toml
[mouse_action_indicator]
enabled = false
actions = ["left_click", "right_click"]

[mouse_action_indicator.ui]
size = 36
shape = "circle"

[mouse_action_indicator.animation]
duration_ms = 260
easing = "ease_out"
```

---

## [mode_indicator]

A floating label that follows the cursor and displays the current mode name.

### Per-Mode

| Option             | Type   | Default        | Description                       |
| ------------------ | ------ | -------------- | --------------------------------- |
| `enabled`          | bool   | varies by mode | Show/hide indicator for this mode |
| `text`             | string | varies by mode | Label text                        |
| `background_color` | color  | derived        | Override background color         |
| `text_color`       | color  | derived        | Override text color               |
| `border_color`     | color  | derived        | Override border color             |

```toml
[mode_indicator.scroll]
enabled = true
text = "Scroll"

[mode_indicator.hints]
enabled = false
text = "Hints"

[mode_indicator.grid]
enabled = false
text = "Grid"

[mode_indicator.recursive_grid]
enabled = false
text = "Recursive Grid"

[mode_indicator.monitor_select]
enabled = false
text = "Monitor Select"
```

### UI

| Option               | Type   | Default | Description                             |
| -------------------- | ------ | ------- | --------------------------------------- |
| `font_size`          | int    | `10`    | Font size                               |
| `font_family`        | string | `""`    | Font family (empty = system default)    |
| `background_color`   | color  | derived | Background with alpha                   |
| `text_color`         | color  | derived | Text color                              |
| `border_color`       | color  | derived | Border color                            |
| `border_width`       | int    | `1`     | Border width                            |
| `padding_x`          | int    | `-1`    | Horizontal padding (-1 = auto)          |
| `padding_y`          | int    | `-1`    | Vertical padding (-1 = auto)            |
| `border_radius`      | int    | `-1`    | Corner radius (-1 = auto)               |
| `indicator_x_offset` | int    | `20`    | X offset from cursor (positive = right) |
| `indicator_y_offset` | int    | `20`    | Y offset from cursor (positive = down)  |

```toml
[mode_indicator.ui]
font_size = 10
border_width = 1
padding_x = -1
padding_y = -1
border_radius = -1
indicator_x_offset = 20
indicator_y_offset = 20
```

---

## [sticky_modifiers]

Tap modifiers inside a mode to make them sticky for subsequent actions.

| Option             | Type | Default | Description                                         |
| ------------------ | ---- | ------- | --------------------------------------------------- |
| `enabled`          | bool | `true`  | Enable sticky modifiers                             |
| `tap_max_duration` | int  | `300`   | Max hold (ms) for tap detection (0 = always toggle) |

### UI

| Option               | Type   | Default | Description                            |
| -------------------- | ------ | ------- | -------------------------------------- |
| `font_size`          | int    | `10`    | Font size                              |
| `font_family`        | string | `""`    | Font family (empty = system default)   |
| `background_color`   | color  | derived | Background with alpha                  |
| `text_color`         | color  | derived | Text color                             |
| `border_color`       | color  | derived | Border color                           |
| `border_width`       | int    | `1`     | Border width                           |
| `padding_x`          | int    | `-1`    | Horizontal padding (-1 = auto)         |
| `padding_y`          | int    | `-1`    | Vertical padding (-1 = auto)           |
| `border_radius`      | int    | `-1`    | Corner radius (-1 = auto)              |
| `indicator_x_offset` | int    | `-40`   | X offset from cursor (negative = left) |
| `indicator_y_offset` | int    | `20`    | Y offset from cursor (down)            |

```toml
[sticky_modifiers]
enabled = true
tap_max_duration = 300

[sticky_modifiers.ui]
font_size = 10
indicator_x_offset = -40
indicator_y_offset = 20
```

On Linux, the indicator renders the symbols `❖⇧⌥⌃`. If they appear as `[][][][]`, set `font_family` to a font with those glyphs.

---

## [smooth_cursor]

Animates cursor movement between positions.

| Option               | Type  | Default | Description                        |
| -------------------- | ----- | ------- | ---------------------------------- |
| `move_mouse_enabled` | bool  | `false` | Enable animated mouse movement     |
| `steps`              | int   | `10`    | Number of animation steps          |
| `max_duration`       | int   | `200`   | Max animation duration in ms       |
| `duration_per_pixel` | float | `0.1`   | Ms per pixel for adaptive duration |

```toml
[smooth_cursor]
move_mouse_enabled = false
steps = 10
max_duration = 200
duration_per_pixel = 0.1
```

---

## [smooth_scroll]

Splits scroll deltas into chunked ease-out events for visual feedback. macOS only; other platforms fall back to instant scrolling.

| Option               | Type  | Default | Description                        |
| -------------------- | ----- | ------- | ---------------------------------- |
| `enabled`            | bool  | `false` | Enable smooth scrolling            |
| `steps`              | int   | `20`    | Number of animation steps          |
| `max_duration`       | int   | `180`   | Max animation duration in ms       |
| `duration_per_pixel` | float | `1.0`   | Ms per pixel for adaptive duration |

```toml
[smooth_scroll]
enabled = false
steps = 20
max_duration = 180
duration_per_pixel = 1.0
```

---

## [held_repeat]

Repeatedly dispatches scroll, page, and relative-mouse-move actions while the key is held, with a configurable initial delay and repeat interval. Disable held-key repeat entirely by setting `enabled = false`.

| Option             | Type | Default | Description                              |
| ------------------ | ---- | ------- | ---------------------------------------- |
| `enabled`          | bool | `false` | Master toggle for held-key repeat        |
| `initial_delay_ms` | int  | `50`    | Delay before first repeat fires (ms)     |
| `interval_ms`      | int  | `50`    | Interval between subsequent repeats (ms) |

```toml
[held_repeat]
enabled = false
initial_delay_ms = 50
interval_ms = 50
```

---

## [systray]

| Option    | Type | Default | Description                |
| --------- | ---- | ------- | -------------------------- |
| `enabled` | bool | `true`  | Show/hide the systray icon |

> Changing this option requires a daemon restart.

---

## [logging]

| Option                 | Type   | Default  | Description                                       |
| ---------------------- | ------ | -------- | ------------------------------------------------- |
| `log_level`            | string | `"info"` | Level: `debug`, `info`, `warn`, `error`           |
| `log_file`             | string | `""`     | Custom log file path (empty = default location)   |
| `disable_file_logging` | bool   | `true`   | Console only (no file); file logs always use JSON |
| `max_file_size`        | int    | `10`     | MB before rotation                                |
| `max_backups`          | int    | `5`      | Old log files to keep                             |
| `max_age`              | int    | `30`     | Days to retain old logs                           |

When `log_file` is empty, Neru writes to a platform default location:

| Platform | Default log file                  |
| -------- | --------------------------------- |
| macOS    | `~/Library/Logs/neru/app.log`     |
| Linux    | `~/.local/state/neru/log/app.log` |
| Windows  | `%LOCALAPPDATA%\neru\log\app.log` |

At the default `info` level, logs focus on lifecycle, configuration, mode activation, and actionable operational events. Use `debug` temporarily when investigating key routing, hint generation, accessibility collection, overlay redraws, or IPC action flow. Debug logs intentionally avoid typed UI text, feed-key payloads, exec output, and full configuration values.

---

Use `neru doctor` and runtime logs for troubleshooting configuration issues.
