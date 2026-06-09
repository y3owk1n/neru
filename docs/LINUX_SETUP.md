# Linux Setup & Testing Guide

Neru provides native Linux support through these display server backends:

- **X11** — works with any X11-based session (XOrg, i3, etc.)
- **Wayland (wlroots)** — works with wlroots-based compositors (Sway, Hyprland, niri, River)
- **Wayland (KDE Plasma)** — KWin exposes `zwlr_layer_shell_v1` and `zxdg_output_manager_v1` (overlays and screen geometry use the shared wlroots client) but **not** `zwlr_virtual_pointer_v1`. Input (move/click/scroll/modifier) is injected through `libei` via the `org.freedesktop.portal.RemoteDesktop` portal instead, which requires approving a one-time "Remote Control" consent prompt per session. See [Measured Compositor Protocol Support](#measured-compositor-protocol-support).

> **GNOME Wayland** is not yet supported. GNOME Shell uses its own private protocols instead of the wlr protocols. See the placeholder files in `internal/core/infra/platform/linux/wayland_gnome/` for contribution guidance.

---

## Table of Contents

- [Supported Compositors & Backends](#supported-compositors--backends)
- [Install-Time Environment Adjustments (Non-Code)](#install-time-environment-adjustments-non-code)
- [Wayland Keyboard Capture Permissions](#wayland-keyboard-capture-permissions)
- [Using nix home manager](#using-nix-home-manager)
- [Build Dependencies](#build-dependencies)
- [Building](#building)
- [Under The Hood: Wayland (wlroots) Architecture Details for Testers](#under-the-hood-wayland-wlroots-architecture-details-for-testers)
- [Validation & Setup Guide](#validation--setup-guide)
- [Known Limitations](#known-limitations)
- [Troubleshooting](#troubleshooting)

---

## Supported Compositors & Backends

| Compositor | Backend         | Status           | Notes                                                   |
| ---------- | --------------- | ---------------- | ------------------------------------------------------- |
| Sway       | wayland-wlroots | ✅ Supported     | Full virtual-pointer and layer-shell support            |
| Hyprland   | wayland-wlroots | ✅ Supported     | Full virtual-pointer and layer-shell support            |
| niri       | wayland-wlroots | ✅ Supported     | Full virtual-pointer and layer-shell support            |
| River      | wayland-wlroots | ✅ Supported     | Full virtual-pointer and layer-shell support            |
| X11 / XOrg | x11             | ✅ Supported     | XTest for input, XRandR for screens                     |
| i3         | x11             | ✅ Supported     | Runs under X11                                          |
| KDE Plasma | wayland-kde     | ✅ Supported     | Overlay + screen geometry via the wlroots client; input via `libei` (RemoteDesktop portal). Needs a one-time per-session consent prompt |
| GNOME      | wayland-gnome   | 🔲 Not Supported | Needs libei + GNOME Shell extension; see PLACEHOLDER.md |

> **KDE input goes through `libei`, not the wlroots virtual pointer (confirmed,
> not assumed).** KWin 6.6.4 does not expose `zwlr_virtual_pointer_v1` (measured
> directly with `wayland-info`; see
> [Measured Compositor Protocol Support](#measured-compositor-protocol-support)),
> so Neru injects input through `libei` via the
> `org.freedesktop.portal.RemoteDesktop` portal. The overlay and screen geometry
> still use the shared wlroots client because `zwlr_layer_shell_v1` and
> `zxdg_output_manager_v1` are present. The portal shows a one-time "Remote
> Control" consent prompt that must be approved before the cursor can move or
> click.

---

## Install-Time Environment Adjustments (Non-Code)

These are changes to the **host environment**, not to Neru's code, that a Linux
install must account for. A from-source build, a Homebrew formula, or a distro
package each needs to either perform these or clearly tell the user to. Keep this
list current as new environment requirements are discovered.

| #   | Adjustment                                                  | Why it is needed                                                                 | Backends affected      | Persists across reboot?      |
| --- | ----------------------------------------------------------- | -------------------------------------------------------------------------------- | ---------------------- | ---------------------------- |
| 1   | Install build dependencies (see [Build Dependencies](#build-dependencies)) | Compile the native CGO backends; a prebuilt binary still needs the matching runtime shared libs present | All Linux              | Yes (packages stay installed) |
| 2   | Add the user to the `input` group: `sudo usermod -aG input "$USER"` | `evdev` keyboard capture for reliable modified clicks and sticky modifiers; see [Wayland Keyboard Capture Permissions](#wayland-keyboard-capture-permissions) | Wayland (wlroots, KDE) | Yes, but requires a re-login to take effect |
| 3   | Bind `neru <mode>` in the compositor's own keybinding config (Sway/Hyprland/niri config, or KDE System Settings -> Custom Shortcuts); see [Hotkey Configuration](#1-hotkey-configuration) | Wayland has no global-hotkey protocol, so Neru cannot register hotkeys itself | All Wayland            | Yes (user config)            |

Notes:

- Item 1 is the only one X11 needs; on X11 global hotkeys work natively via `XGrabKey`.
- Item 2 only changes the effective permission **after a full logout/login or reboot**;
  the running session keeps its old group set. Without it, Neru falls back to the
  less capable overlay-focused keyboard path.
- Item 3 is user configuration by nature and cannot be automated by a package; the
  most an installer can do is ship example snippets.

### Not Fixable At Install Time (Compositor Capability Gaps)

Some requirements are properties of the **compositor**, not the host, so no install
step can add them:

- **`zwlr_virtual_pointer_v1`** — used for pointer move/click on wlroots
  compositors (Sway, Hyprland, niri, River). **KWin 6.6.4 does not advertise it**
  (confirmed below); on KDE, Neru injects input through `libei` via the
  RemoteDesktop portal instead, so a missing virtual pointer is not fatal there.
- **`zwp_virtual_keyboard_manager_v1`** — used for sticky-modifier key injection
  on wlroots compositors; on KDE the same modifiers go through the libei keyboard
  device when the portal grants one.

### Measured Compositor Protocol Support

The clean way to answer "will Neru work on compositor X" is to enumerate the
Wayland globals it advertises, with no build or install required:

```bash
# Run inside the graphical session (needs WAYLAND_DISPLAY).
wayland-info | grep -E 'zwlr_layer_shell|zwlr_virtual_pointer|zwp_virtual_keyboard|fake_input|xdg_output'
```

Neru's wlroots path needs **both** `zwlr_layer_shell_v1` (overlay) and
`zwlr_virtual_pointer_v1` (pointer). If both are present, the shared wlroots path
works as-is; if the pointer protocol is missing, the compositor needs a
desktop-specific input path instead.

| Protocol                            | Purpose                       | wlroots (Sway/Hyprland/niri/River) | KWin 6.6.4 (KDE Plasma) |
| ----------------------------------- | ----------------------------- | ---------------------------------- | ----------------------- |
| `zwlr_layer_shell_v1`               | overlay surfaces              | yes                                | yes (v5)                |
| `zxdg_output_manager_v1`            | screen geometry / xdg-output  | yes                                | yes (v3)                |
| `zwlr_virtual_pointer_v1`           | pointer move / click          | yes                                | **no**                  |
| `zwp_virtual_keyboard_manager_v1`   | sticky-modifier key injection | yes                                | **no**                  |
| `org_kde_kwin_fake_input`           | KWin-native input emulation   | n/a                                | **no** (not advertised) |

**Conclusion for KDE:** the overlay and screen-geometry half uses the shared
wlroots client (both protocols are present); the input half goes through
something KWin actually exposes — `libei` via the
`org.freedesktop.portal.RemoteDesktop` portal. The backend choice is made at the
Wayland input dispatcher (`system_linux_wayland_input.go`): if the compositor
advertises `zwlr_virtual_pointer_v1` it uses the wlroots virtual pointer,
otherwise it uses libei. The two input mechanisms never overlap, and libei never
touches the wlroots input implementation.

> **Reusing this for other desktops (e.g. COSMIC):** run the same `wayland-info`
> check in that session. If it lists both `zwlr_layer_shell_v1` and
> `zwlr_virtual_pointer_v1`, support is the straightforward shared-wlroots case;
> if not, it needs the same desktop-specific input path KDE will need.

### Open Questions for Maintainers (Packaging)

Before this lands, we should ask the repo owner how the build/packaging process is
expected to handle the OS-specific steps above:

- How should the **Homebrew formula** (or distro packages) handle the `input` group
  membership (item 2)? Homebrew on Linux runs unprivileged and should not modify
  system groups or invoke `sudo`, so this likely has to be a documented post-install
  manual step (a `caveats` message), not an automated action.
- Which **runtime shared libraries** must be present on the target system for a bottled
  binary, and how is that expressed in the formula?
- Is manual compositor keybinding setup (item 3) acceptable as the supported path, or
  should we ship per-compositor example configs?

---

## Wayland Keyboard Capture Permissions

On wlroots-based Wayland compositors, Neru uses direct `evdev` keyboard capture
during active modes so modified clicks like `Ctrl+click` and sticky modifiers
work reliably against the app underneath the overlay.

This requires permission to open and grab `/dev/input/event*` keyboard devices.
On many distros those devices are owned by `root:input` with mode `0660`, so
your user must be in the `input` group.

```bash
sudo usermod -aG input "$USER"
```

Then fully log out and back in, or reboot, and confirm:

```bash
id
```

You should see `input` in the printed group list.

> **Security note:** Membership in `input` allows reading system-wide keyboard
> events. If that access is too broad for your environment, use a tighter
> distro-specific `udev`/ACL setup instead of the group-based approach.

When the backend is active, Neru logs:

```text
Using Wayland evdev keyboard capture
```

If Neru cannot access the devices, it falls back to overlay-focused keyboard
capture. Basic mode navigation still works, but modified clicks and sticky
modifier behavior may be degraded under Wayland.

---

## Using nix home manager

Below is a minimal single flake with home manager setup.

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    neru.url = "github:y3owk1n/neru";
  };

  outputs =
    {
      nixpkgs,
      home-manager,
      neru,
      ...
    }:
    {
      homeConfigurations."my-host" = home-manager.lib.homeManagerConfiguration {
        pkgs = nixpkgs.legacyPackages.x86_64-linux; # or aarch64-linux

        modules = [
          (
            { pkgs, ... }:
            {
              nixpkgs.overlays = [ neru.overlays.default ];
              home.username = "youruser";
              home.homeDirectory = "/home/youruser";
              home.stateVersion = "24.05";

              home.packages = [
                pkgs.neru
              ];

              programs.home-manager.enable = true;
            }
          )
        ];
      };
    };
}
```

## Build Dependencies

`libei` and `liboeffis` provide the KDE Plasma input path (input injection
through the `org.freedesktop.portal.RemoteDesktop` portal, since KWin does not
implement `zwlr_virtual_pointer_v1`). The CGO build links them via
`pkg-config libei-1.0 liboeffis-1.0`, so the `-devel`/`-dev` packages are
required at build time and the runtime shared libs at run time.

### Debian / Ubuntu

```bash
sudo apt-get install -y \
  libcairo2-dev \
  libwayland-dev \
  libx11-dev \
  libxtst-dev \
  libxrandr-dev \
  libxinerama-dev \
  libxfixes-dev \
  libxkbcommon-dev \
  libei-dev \
  liboeffis-dev \
  wayland-protocols
```

### Fedora

```bash
sudo dnf install -y \
  cairo-devel \
  wayland-devel \
  libX11-devel \
  libXtst-devel \
  libXrandr-devel \
  libXinerama-devel \
  libXfixes-devel \
  libxkbcommon-devel \
  libei-devel \
  liboeffis-devel \
  wayland-protocols-devel
```

### Arch Linux

```bash
sudo pacman -S \
  cairo \
  wayland \
  libx11 \
  libxtst \
  libxrandr \
  libxinerama \
  libxfixes \
  libxkbcommon \
  libei \
  wayland-protocols
```

---

## Building

```bash
# Build for the current host architecture (recommended for local dev/testing).
# No arch flag needed: Go targets the host arch automatically.
just build

# Build an explicitly-named Linux arch. NOTE: this recipe defaults to amd64.
just build-linux arm64   # Apple Silicon / arm64 hosts (e.g. UTM VMs on a Mac)
just build-linux amd64   # x86_64 hosts

# Cross-compilation from macOS is NOT supported for Linux targets
# because the native backends require CGo and Linux system headers.
```

### Architecture note (Apple Silicon / UTM)

On an Apple Silicon Mac, UTM VMs are **arm64**. Use the native `just build`
there — it produces an arm64 binary with no extra flags. Only the
`just build-linux` recipe needs an explicit arch, and it **defaults to amd64**,
so pass `arm64` on these VMs (`just build-linux arm64`); running it bare
cross-compiles for amd64 with CGO and fails without an amd64 toolchain.

Verify what you built:

```bash
go env GOARCH        # expect: arm64
file bin/neru        # expect: ... ARM aarch64
```

---

## Under The Hood: Wayland (wlroots) Architecture Details for Testers

As a tester on Linux Wayland environments, here are key implementation details you should be aware of to effectively test edge cases.

### Global Cursor Discovery ("Agitation" Trick)

Unlike X11 or macOS, Wayland completely hides the global mouse coordinate from clients for security reasons. Before Neru can successfully navigate matrices or grids, it must learn the current mouse position.
**How we solved it:** Upon startup, Neru spawns invisible full-screen `layer-shell` surfaces across all outputs. It then "wiggles" the virtual pointer natively. This forces the compositor to send a `pointer_enter` event to our transparent surface, allowing us to map local surface coordinates to global screen dimensions and capture exactly where your mouse is.

> **Testing Tip:** Ensure the cursor accurately discovers its initial position across multi-monitor setups (especially asymmetric resolutions).

### Proper Key Modifier Routing

Wayland passes key codes independently of modifiers. Translating `Ctrl+K` relies entirely on your compositor's active XKB map.
**How we solved it:** We leverage `xkb_state_mod_name_is_active` to explicitly inspect `Shift`, `Control`, `Mod1` (Alt), and `Mod4` (Super). Printable UTF-8 characters are correctly resolved utilizing `xkb_state_key_get_utf8` (fixing cases where `,` or `/` evaluate incorrectly to "comma").

> **Testing Tip:** Rapidly tap modifiers alongside character keys (like `Shift`, `Ctrl`, and complex symbols like `+` or `,`) to verify hotkeys trigger successfully.

### Click-Through Layer Shell Overlays

To draw overlay UI (like Grid Mode) without stealing physical mouse clicks, Wayland requires precise protocol negotiations.
**How we solved it:** Neru sets an explicit, empty `wl_region` as the `input_region` for its layer-shell surfaces. This forces the compositor to ignore the overlay for pointer intersection testing, enabling true click-through capability where your synthetic clicks land exactly on the app under the overlay grid.

> **Testing Tip:** While in Recursive Grid mode, executing a synthetic click (e.g., `u` for left click) should seamlessly pass straight through the overlay into your browser or terminal underneath.

### Wayland Smooth Scrolling

Rather than sending redundant, chunky fractional scroll loops, Neru directly pipes raw pixel `deltaY` and `deltaX` into the standard Wayland continuous `axis` event without discretizing them. This affords ultra-smooth precise scrolling behaviors equivalent to macOS behavior.

> **Testing Tip:** Enter Scroll mode and verify scrolling operates smoothly, cleanly maps to your configured scroll increments, and does not overwhelm/lag the compositor event queues.

### Dynamic Rendering Buffer Lifecycles

When UI modes rapidly open, exit, and re-open, the underlying Cairo buffers require clean state management so they don't unexpectedly disappear.
**How we solved it:** Wayland buffers are dynamically and lazily initialized immediately prior to any stroke or draw commands—ensuring they reliably exist, even if a previous mode exit forcefully destroyed the window canvas.

---

## Validation & Setup Guide

### 1. Hotkey Configuration

**X11:**
On X11, Neru registers global hotkeys natively using `XGrabKey`. Hotkeys specified in your `config.toml` "just work".

**Wayland (wlroots):**
Wayland does not have a standard protocol for global hotkey registration. You **must** bind `neru <mode>` via your compositor's own keybinding config!

#### Sway Example

```sway
# ~/.config/sway/config
bindsym $mod+Shift+h exec neru hints
bindsym $mod+Shift+g exec neru grid
bindsym $mod+Shift+s exec neru scroll
```

#### Hyprland Example

```hyprlang
# ~/.config/hypr/hyprland.conf
bind = $mod SHIFT, H, exec, neru hints
bind = $mod SHIFT, G, exec, neru grid
bind = $mod SHIFT, S, exec, neru scroll
```

#### niri Example

```kdl
// ~/.config/niri/config.kdl
binds {
    Mod+Shift+H { spawn-sh "neru hints"; }
    Mod+Shift+G { spawn-sh "neru grid"; }
    Mod+Shift+S { spawn-sh "neru scroll"; }
    Mod+Shift+R { spawn-sh "neru recursive_grid"; }
}
```

#### KDE Plasma Example

KDE Plasma Wayland cannot register global hotkeys from inside Neru, so bind
`neru <mode>` in **System Settings -> Shortcuts -> Custom Shortcuts**. Use the
absolute path to the binary so KWin resolves it reliably:

| Action         | Command                                       |
| -------------- | --------------------------------------------- |
| Hints          | `/home/<you>/.local/bin/neru hints`           |
| Grid           | `/home/<you>/.local/bin/neru grid`            |
| Recursive grid | `/home/<you>/.local/bin/neru recursive_grid`  |
| Scroll         | `/home/<you>/.local/bin/neru scroll`          |

### 2. Application Exclusions

On Linux, applications are identified by their X11 `WM_CLASS` (X11) or process name from `/proc/<pid>/cmdline` (Wayland). Use these exact identifiers in your `excluded_apps` list.

```toml
[general]
excluded_apps = ["firefox", "chromium-browser", "code"]
```

### 3. Service Management

`neru services install/start/stop` uses `launchctl` on macOS. For Linux, use standard `systemd`:

```bash
# Create a systemd user service
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/neru.service << EOF
[Unit]
Description=Neru keyboard navigation daemon

[Service]
ExecStart=%h/.local/bin/neru daemon
Restart=on-failure

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now neru
```

---

## Known Limitations

1. **Wayland global hotkeys**: Must be configured in the compositor, not in Neru's config. See [Hotkey Configuration](#1-hotkey-configuration).
2. **Accessibility (AT-SPI)**: Full AT-SPI integration for clickable element discovery (hints mode) is currently unavailable natively under Wayland without relying on experimental plugins. Grid mode and scroll mode both work perfectly without AT-SPI.
3. **Dark mode detection**: Detected via the `org.freedesktop.appearance` xdg-desktop-portal interface, with a `~/.config/kdeglobals` fallback, so `neru doctor` reports the current color scheme on any desktop that ships a portal. Restyling overlays to match the detected theme is not yet wired up.
4. **Notifications**: Desktop notifications (`org.freedesktop.Notifications`) will log to stdout/file instead of pushing to DBus.
5. **Wayland modified clicks need evdev access**: On wlroots compositors, reliable
   modified pointer actions depend on the `evdev` keyboard-capture path described
   above. Without `/dev/input/event*` access, Neru falls back to a less capable
   overlay-focused path.
6. **KDE input needs RemoteDesktop consent**: On KDE Plasma, input goes through
   `libei` via the RemoteDesktop portal, which shows a one-time "Remote Control"
   consent prompt the first time Neru injects input in a session. Approve it (you
   can tick "remember" where offered) or input will fail with `CodeActionFailed`.
   Modifier keys require the portal to grant a keyboard device; if it grants only
   a pointer, modified clicks degrade.
7. **Screen resolution is read once at startup**: Neru enumerates output geometry
   (`xdg_output` logical size) when the daemon starts and caches it. If the
   resolution changes afterward — common when resizing a VM window, and also on
   monitor hotplug / docking — the overlay and hint coordinates stay at the old
   size. Relaunch Neru after changing resolution to pick up the new geometry.
   Tracking resolution changes live is a planned follow-up.

---

## Troubleshooting

### "WAYLAND_DISPLAY is not set"

You're running under X11 or a TTY. Neru will automatically use the X11 backend when `DISPLAY` is set. If you're in a purely headless TTY wrapper, Neru cannot hook inputs.

### "compositor does not support zwlr_virtual_pointer_v1"

Your Wayland compositor does not currently implement `wlr` unstable protocols. This typically occurs under GNOME Shell, which uses its own private protocols. Check the placeholder docs to learn how libei implementations will govern GNOME support in the future. On KDE Plasma this is expected — KWin does not advertise the protocol and Neru routes input through `libei` instead (see below).

### KDE: "could not establish a libei input session via the RemoteDesktop portal"

On KDE Plasma, Neru injects input through `libei` via the
`org.freedesktop.portal.RemoteDesktop` portal. The first input action in a
session pops a "Remote Control" consent dialog; approve it before the connect
times out. If you denied it, revoke/re-grant the permission in System Settings
(Apps & Window Management / portal permissions) and retry. The session also
needs the portal services running (`xdg-desktop-portal` and
`xdg-desktop-portal-kde`).

### Overlay or hints are the wrong size / offset after resizing

Neru reads screen geometry once when the daemon starts. If you resized a VM
window, changed display scaling, or hotplugged a monitor after launch, the
overlay still uses the old logical size. Relaunch Neru (`neru stop` then
`neru launch`) to pick up the new resolution.

### "failed to connect to Wayland compositor"

Check that `WAYLAND_DISPLAY` is set correctly and the Wayland socket permissions map appropriately (especially useful if testing behind Flatpaks or tight sandbox boundaries).

```bash
echo $WAYLAND_DISPLAY
wl-info  # from wayland-utils package
```

### "Wayland evdev capture unavailable; falling back to overlay keyboard focus"

Neru could not open and grab the keyboard devices needed for reliable Wayland
modifier handling.

Common fix:

```bash
sudo usermod -aG input "$USER"
```

Then log out and back in, and confirm:

```bash
id
```

If the setup is correct, Neru should log:

```text
Using Wayland evdev keyboard capture
```

### Sticky modifier indicator shows `[][][][]`

The sticky modifier overlay uses Unicode modifier symbols on Linux:
`❖⇧⌥⌃`. If you see square boxes instead, the configured font does not include
those glyphs.

Set a font explicitly in your config:

```toml
[sticky_modifiers.ui]
font_family = "Your installed symbol-capable font"
```

An empty `font_family` uses the system default, which may not have the required
symbol coverage. A quick way to verify a candidate font is to paste `❖⇧⌥⌃` into
a text editor and confirm the symbols render there first.
