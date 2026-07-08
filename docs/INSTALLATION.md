# Installation

Neru runs on macOS 14.0+ with full support. Linux and Windows have basic support for core modes.

---

## macOS

### Homebrew (Recommended)

```bash
brew tap y3owk1n/tap
brew install --cask y3owk1n/tap/neru
```

> Tap maintained at [y3owk1n/homebrew-tap](https://github.com/y3owk1n/homebrew-tap).

### Homebrew Nightly

```bash
brew install --cask y3owk1n/tap/neru-nightly
brew upgrade --cask --greedy y3owk1n/tap/neru-nightly
```

> You cannot have both `stable` and `nightly` installed simultaneously.

### Nix Flake

```nix
{
  inputs.neru.url = "github:y3owk1n/neru";

  # nix-darwin module
  nixpkgs.overlays = [ neru.overlays.default ];
  services.neru.enable = true;
  services.neru.config = ''
    [hotkeys]
    "Primary+Shift+Space" = "hints"
  '';
}
```

Module options: `package`, `config`, `configFile`, `launchd.enable`, `extraEnvironment`.

**home-manager module:**

```nix
{
  nixpkgs.overlays = [ neru.overlays.default ];
  services.neru.enable = true;
}
```

> **Codesign for source builds:** Add a `postActivation` script to codesign the `.app` bundle. Not needed for the pre-built `pkgs.neru` package.

### Build from Source

Requirements: Go 1.26.4+, Xcode Command Line Tools, `just`.

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru
just release
mv ./bin/neru /usr/local/bin/neru
```

Or build the macOS app bundle: `just bundle && mv ./build/Neru.app /Applications/Neru.app`

---

## Linux

### Supported Backends

| Compositor / Session        | Backend         | Status        |
| :-------------------------- | :-------------- | :------------ |
| Sway, Hyprland, niri, River | wayland-wlroots | Supported     |
| KDE Plasma (Wayland)        | wayland-kde     | Supported     |
| X11 / XOrg, i3              | x11             | Supported     |
| GNOME (Wayland)             | wayland-gnome   | Not supported |

### Nix (Home Manager)

```nix
{
  inputs.neru.url = "github:y3owk1n/neru";
  nixpkgs.overlays = [ neru.overlays.default ];
  home.packages = [ pkgs.neru ];
}
```

### Build from Source

**Install dependencies:**

Debian/Ubuntu:

```bash
sudo apt-get install -y \
  libcairo2-dev libwayland-dev libx11-dev libxtst-dev libxrandr-dev \
  libxinerama-dev libxfixes-dev libxkbcommon-dev libei-dev liboeffis-dev \
  libfontconfig-dev wayland-protocols fonts-dejavu-core
```

Fedora:

```bash
sudo dnf install -y \
  cairo-devel wayland-devel libX11-devel libXtst-devel libXrandr-devel \
  libXinerama-devel libXfixes-devel libxkbcommon-devel libei-devel \
  liboeffis-devel fontconfig-devel wayland-protocols-devel \
  dejavu-sans-fonts dejavu-serif-fonts dejavu-sans-mono-fonts
```

Arch:

```bash
sudo pacman -S cairo wayland libx11 libxtst libxrandr libxinerama \
  libxfixes libxkbcommon libei fontconfig wayland-protocols ttf-dejavu
```

**Add user to `input` group (Wayland):**

```bash
sudo usermod -aG input "$USER"
# Log out and back in
```

**Build:**

```bash
just build            # Native build on the host
just build-linux      # amd64 cross-build
just build-linux arm64 # arm64 cross-build
```

> Cross-compilation from macOS to Linux is NOT supported (CGO + Linux headers).

### Configure Hotkeys

**X11:** Hotkeys in `config.toml` work via `XGrabKey`. No compositor setup needed.

**Wayland:** Bind `neru <mode>` in your compositor config:

Sway (`~/.config/sway/config`):

```sway
bindsym $mod+Shift+h exec neru hints
bindsym $mod+Shift+g exec neru grid
bindsym $mod+Shift+s exec neru scroll
```

Hyprland (`~/.config/hypr/hyprland.conf`):

```hyprlang
bind = $mod SHIFT, H, exec, neru hints
bind = $mod SHIFT, G, exec, neru grid
bind = $mod SHIFT, S, exec, neru scroll
```

niri (`~/.config/niri/config.kdl`):

```kdl
binds { Mod+Shift+H { spawn-sh "neru hints"; } }
```

### systemd Service

```bash
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/neru.service << 'EOF'
[Unit]
Description=Neru keyboard navigation daemon

[Service]
ExecStart=%h/.local/bin/neru launch
Restart=on-failure

[Install]
WantedBy=default.target
EOF
systemctl --user daemon-reload
systemctl --user enable --now neru
```

### Known Linux Limitations

1. **Wayland global hotkeys** — must be configured in the compositor
2. **Hints need AT-SPI** — grid and scroll work without it
3. **Screen geometry cached at startup** — relaunch after resolution changes
4. **Wayland modified clicks** — need `evdev` access (`input` group)
5. **Sticky modifier indicator** — may show `[][][][]` if font lacks modifier glyphs; set `font_family`
6. **Dark mode** — via `org.freedesktop.appearance` portal

For DE-specific details, see [Cross-Platform Guide](CROSS_PLATFORM.md).

---

## Windows

Install via source build:

```bash
just build-windows
```

Supported: grid, recursive grid, scroll modes, mouse injection, global hotkeys, keyboard hooks, UIA accessibility (initial), named-pipe IPC.

---

## Post-Installation

### 1. Grant Permissions (macOS)

Open **System Settings → Privacy & Security → Accessibility** and add Neru.

### 2. Start the Daemon

```bash
neru launch              # Start now
neru services install    # Auto-start on login
```

### 3. Verify

```bash
neru status     # Should show "running"
neru hints      # Should show hint overlays
```

### 4. Configure

```bash
neru config init         # Create starter config
```

Config loads from `~/.config/neru/config.toml`. See [Configuration Guide](CONFIGURATION.md).

---

## Shell Completions

```bash
neru completion bash > /usr/local/etc/bash_completion.d/neru
neru completion zsh > "${fpath[1]}/_neru"
neru completion fish > ~/.config/fish/completions/neru.fish
neru completion powershell > neru.ps1
```

---

## Troubleshooting

| Problem                                                 | Solution                                                |
| :------------------------------------------------------ | :------------------------------------------------------ |
| "Cannot open Neru because developer cannot be verified" | `xattr -cr /Applications/Neru.app`                      |
| "Command not found: neru"                               | Add `/usr/local/bin` to PATH                            |
| Homebrew fails                                          | `brew update && brew reinstall --cask neru`             |
| Nix build fails                                         | Check architecture — use correct URL for arm64 vs amd64 |
| "WAYLAND_DISPLAY is not set"                            | Running under X11; X11 backend used                     |
| "failed to connect to Wayland compositor"               | Check `$WAYLAND_DISPLAY` and `wl-info`                  |

---

## Uninstallation

```bash
# Homebrew
brew uninstall --cask neru

# Manual
neru services uninstall
rm -rf /Applications/Neru.app
rm /usr/local/bin/neru
rm -rf ~/.config/neru ~/Library/Application\ Support/neru ~/Library/Logs/neru

# Nix: remove module from configuration and rebuild
```
