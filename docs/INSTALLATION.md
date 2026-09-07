# Installation Guide

This guide covers installation methods for Neru, with the most complete support on macOS.

**Related:** [CLI Reference](CLI.md) · [Configuration Reference](CONFIGURATION.md) ·
[Linux setup](LINUX_SETUP.md) · [Troubleshooting](TROUBLESHOOTING.md)

> [!NOTE]
> macOS is the primary supported platform. Linux builds are available through the Nix flake (uses release artifacts when available, falls back to source build), and direct source builds. See the [Platform Support section in README.md](../README.md#platform-support) for details.

---

## Table of Contents

- [Requirements](#requirements)
- [Method 1: Install Script](#method-1-install-script)
- [Method 2: Homebrew](#method-2-homebrew)
- [Method 3: Nix Flake](#method-3-nix-flake)
- [Method 4: From Source](#method-4-from-source)
- [Post-Installation](#post-installation)
- [Shell Completions](#shell-completions)
- [Troubleshooting](#troubleshooting)
- [Uninstallation](#uninstallation)

---

## Requirements

- **macOS**: 14.0 or later, plus Accessibility permission (granted during setup)
- **Linux** (beta): X11 or a supported Wayland compositor — see
  [LINUX_SETUP.md](LINUX_SETUP.md) for host requirements per backend
- **Windows** (beta): Windows 10 or later — see the
  [capability matrix](CROSS_PLATFORM.md#capability-matrix)

---

## Method 1: Install Script

One command installs everything a release ships. That is the binary, `Neru.app`
on macOS, man pages, shell completions, and the login service if you want it.
Run the same command again to update.

```bash
# macOS and Linux
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash
```

```powershell
# Windows
irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1 | iex
```

The script checks every download against the `.sha256` file published beside it
before it unpacks anything.

### Channels and versions

The script installs the latest **stable** release by default. Pin a stable
version, or follow **nightly** (rebuilt from every push to `main`):

```bash
# latest stable (default)
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash

# a specific stable release
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --version v1.52.0

# nightly
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --channel nightly
```

```powershell
# flags need a script block; the NERU_CHANNEL / NERU_VERSION / NERU_YES variables work with plain `irm | iex`
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1))) -Channel nightly
```

Running the script again updates in place on whichever channel you already have.
A stable install moves to the newest release, or the script tells you it is
current. A nightly install always gets the latest nightly build. Asking for the
other channel is a switch, and the script treats it as one. It reads the version
of the installed build, says what it found, and asks before it replaces stable
with nightly or nightly with stable.

### Flags

| bash                        | PowerShell        | Env            | Effect                                                              |
| :-------------------------- | :---------------- | :------------- | :------------------------------------------------------------------ |
| `--channel stable\|nightly` | `-Channel`        | `NERU_CHANNEL` | Release channel. Default: the installed one, else stable            |
| `--version vX.Y.Z`          | `-Version`        | `NERU_VERSION` | Pin a stable release (implies stable)                               |
| `--from DIR`                | `-From`           | `NERU_FROM`    | Install a local `just dist` tree instead of downloading; what `just install` uses |
| `--bin-dir DIR`             |                   | `NERU_BIN_DIR` | Where `neru` goes. Default `/usr/local/bin` (macOS), `~/.local/bin` (Linux) |
| `--app-dir DIR`             |                   | `NERU_APP_DIR` | macOS: where `Neru.app` goes. Default `/Applications`               |
| `--no-service`              | `-NoService`      |                | Never register or start the login service                           |
| `--no-completions`          | `-NoCompletions`  |                | Skip shell completions                                              |
| `--no-man`                  |                   |                | Skip man pages                                                      |
| `--force`                   | `-Force`          |                | Reinstall the same version                                          |
| `--uninstall`               | `-Uninstall`      |                | Remove everything a previous run installed; config is kept          |
| `--purge`                   | `-Purge`          |                | With uninstall, also delete config, data and logs (each confirmed)  |
| `-y`, `--yes`               | `-Yes`            | `NERU_YES=1`   | Accept every prompt; required when no terminal is attached          |

### What it does

On macOS it copies `Neru.app` to `/Applications`, symlinks `neru` into
`/usr/local/bin`, puts man pages in `/usr/local/share/man/man1`, and writes
completions for whichever of bash, zsh and fish you have. It asks for sudo only
when one of those directories is not writable, and says so before the password
prompt. Say yes to the last question and it registers the launchd login agent.

On Linux the release binary links the X11, Wayland, tesseract and pipewire
libraries dynamically, so the script runs the downloaded binary once before it
installs anything. When a library is missing it lists the names, points at the
package lists in [LINUX_SETUP.md](LINUX_SETUP.md#build-dependencies), and stops
without touching your system. Once that passes it copies `neru` to
`~/.local/bin`, man pages to `~/.local/share/man/man1`, and completions to the
usual per-user paths. It offers
the systemd user service and, when you are not already in it, the `input` group
that Wayland keyboard capture needs.

On Windows it puts `neru.exe` under `%LOCALAPPDATA%\Programs\neru`, adds that
directory to your user PATH, creates a Start Menu shortcut, and offers PowerShell
completion in your profile and a Task Scheduler logon task.

When a login service is already registered, the script unloads it before it
replaces the binary and registers it again afterwards. It refuses to run over a
Homebrew or Nix-managed install and prints the command to use instead.

---

## Method 2: Homebrew

> [!NOTE]
> The homebrew tap is maintained in another repo: [y3owk1n/homebrew-tap](https://github.com/y3owk1n/homebrew-tap)
> If there's a problem with the tap, please open an issue in that repo or even better, a PR.

Note that you cannot have both `stable` and `nightly` installed at the same time. Uninstall the other one first or it will error out.

```bash
brew tap y3owk1n/tap

# Install latest stable release
brew install --cask y3owk1n/tap/neru

# Install latest nightly release
brew install --cask y3owk1n/tap/neru-nightly

# Upgrade to latest stable release
brew upgrade --cask y3owk1n/tap/neru

# Upgrade to latest nightly release
# Note that you will need to do `--greedy` due to the nature of nightly releases
# without `--greedy`, it won't upgrade the rolling releases
brew upgrade --cask --greedy y3owk1n/tap/neru-nightly

# Uninstall stable
brew uninstall --cask y3owk1n/tap/neru

# Uninstall nightly
brew uninstall --cask y3owk1n/tap/neru-nightly
```

---

## Method 3: Nix Flake

Neru is available as a Nix flake with built-in support for nix-darwin (macOS), NixOS (Linux), and home-manager (both platforms).

On macOS, `pkgs.neru` uses the published release zip and `pkgs.neru-source` builds from source.
On Linux, `pkgs.neru` uses the published release artifact and `pkgs.neru-source` builds from source.

### Add Flake Input

Add Neru to your flake inputs:

```nix
# flake.nix
{
  inputs = {
     # ... other inputs
     neru.url = "github:y3owk1n/neru"; # or "https://flakehub.com/f/y3owk1n/neru/0.1"
     # ... other inputs
  };
}
```

### Option 1: nix-darwin Module (System-Level)

Use the nix-darwin module for system-wide installation:

```nix
# flake.nix
{
  outputs = { self, nixpkgs, nix-darwin, neru, ... }: {
     darwinConfigurations.your-hostname = nix-darwin.lib.darwinSystem {
       modules = [
         # Apply the Neru overlay
         {
           nixpkgs.overlays = [ neru.overlays.default ];
         }

         # Import the Neru module
         neru.darwinModules.default

         # Configure Neru
         {
            # Enable Neru
            services.neru.enable = true;

            # Optional: Use specific package version
            # services.neru.package = pkgs.neru; # This will use the latest version
            # services.neru.package = pkgs.neru-source; # This will build from source

            # Optional: Inline configuration
            services.neru.config = ''
              [hotkeys]
              "Primary+Shift+Space" = "hints left_click"
              "Primary+Shift+G" = "grid left_click"

              [general]
              excluded_apps = ["com.apple.Terminal"]
            '';
         }
       ];
     };
  };
}
```

**Module Options:**

- `services.neru.enable` - Enable Neru (default: `false`)
- `services.neru.package` - Package to use (default: `pkgs.neru` for latest version) or `pkgs.neru-source` for building from source
- `services.neru.config` - Inline TOML configuration (default: uses `configs/default-config.toml`)
- `services.neru.configFile` - Path to existing config file (default: `null`, takes precedence over `config`)
- `services.neru.settings` - TOML configuration expressed as a Nix attribute set (default: `{}`, takes precedence over `config`)
- `services.neru.launchd.enable` - Enable the launchd agent (default: `true`)
- `services.neru.launchd.keepAlive` - Keep the launchd service alive (default: `true`)
- `services.neru.extraEnvironment` - Additional environment variables for the launchd service (default: `{}`; includes a sensible `PATH` with Nix binary directories)

The module automatically:

- Installs Neru system-wide
- Creates a launchd user agent with the configured environment
- Configures the agent to run at login with `KeepAlive` and `RunAtLoad = true`
- Installs shell completions for bash, fish, and zsh

> [!NOTE]
> **Codesign for source builds (`neru-source`):** The Go linker signs the binary
> automatically, but this linker signature lacks hardened runtime entitlements.
> To embed our `Neru.entitlements` with `--options runtime`, use Apple's `codesign`
> (available outside the build sandbox). The entitlements file is bundled at
> `Contents/Resources/Neru.entitlements`.
>
> This is not needed for the default `pkgs.neru` (zip) package, which is pre-signed.
>
> #### Home Manager
>
> ```nix
> { config, lib, ... }:
>
> let
>   username = config.home.username or "changeme";
>   appPath = "/Users/${username}/Applications/Home Manager Apps/Neru.app";
>   entitlements = "${appPath}/Contents/Resources/Neru.entitlements";
> in {
>   home.activation.signNeru = lib.hm.dag.entryAfter [ "copyApps" ] ''
>     if [ -e "${appPath}" ]; then
>       echo "Codesigning Neru.app..."
>       /usr/bin/codesign --force --sign - \
>         --entitlements "${entitlements}" \
>         --options runtime \
>         --timestamp=none \
>         "${appPath}"
>     fi
>   '';
> }
> ```
>
> #### nix-darwin
>
> ```nix
> { config, lib, ... }:
>
> let
>   appPath = "/Applications/Nix Apps/Neru.app";
>   entitlements = "${appPath}/Contents/Resources/Neru.entitlements";
> in {
>   system.activationScripts.postActivation.text = ''
>     if [ -e "${appPath}" ]; then
>       echo "Codesigning Neru.app..."
>       /usr/bin/codesign --force --sign - \
>         --entitlements "${entitlements}" \
>         --options runtime \
>         --timestamp=none \
>         "${appPath}"
>     fi
>   '';
> }
> ```

### Option 2: NixOS Module (System-Level, Linux)

Use the NixOS module for system-wide installation on Linux:

```nix
# flake.nix
{
  outputs = { self, nixpkgs, neru, ... }: {
     nixosConfigurations.your-hostname = nixpkgs.lib.nixosSystem {
       system = "x86_64-linux";
       modules = [
         # Apply the Neru overlay
         {
           nixpkgs.overlays = [ neru.overlays.default ];
         }

         # Import the Neru module
         neru.nixosModules.default

         # Configure Neru
         {
            # Enable Neru
            services.neru.enable = true;

            # Optional: Use specific package version
            # services.neru.package = pkgs.neru; # This will use the pre-built artifact on Linux

            # Optional: Inline configuration
            services.neru.config = ''
             [hotkeys]
             "Ctrl+Shift+Space" = "hints left_click"
             "Ctrl+Shift+G" = "grid left_click"
           '';

            # Optional: Use existing config file (takes precedence)
            # services.neru.configFile = ./path/to/config.toml;
         }
       ];
     };
  };
}
```

**Module Options:**

- `services.neru.enable` - Enable Neru (default: `false`)
- `services.neru.package` - Package to use (default: `pkgs.neru`; uses release artifact on Linux, builds from source if unavailable)
- `services.neru.config` - Inline TOML configuration (default: uses `configs/default-config.toml`)
- `services.neru.configFile` - Path to existing config file (default: `null`, takes precedence over `config`)
- `services.neru.settings` - TOML configuration expressed as a Nix attribute set (default: `{}`, takes precedence over `config`)
- `services.neru.systemd.restart` - Systemd restart policy (default: `"on-failure"`)
- `services.neru.systemd.restartSec` - Seconds to wait before restarting (default: `5`)
- `services.neru.extraEnvironment` - Additional environment variables for the systemd service (default: `{}`; includes a sensible `PATH` with Nix binary directories)

The module automatically:

- Installs Neru system-wide
- Creates a systemd user service tied to `graphical-session.target`
- Configures automatic restart on failure
- Sets the configured environment variables in the service

> [!IMPORTANT]
> On Linux, `pkgs.neru` uses the release artifact when available. Use `pkgs.neru-source` to build from source. If your nixpkgs doesn't ship a recent enough Go version, see [Patch Go Version](#patch-go-version) below.

> [!WARNING]
> **Default config uses cross-platform hotkeys.** The built-in default configuration uses the `Primary+…` modifier, which maps to Cmd on macOS and Ctrl on Linux.

### Option 3: home-manager Module (User-Level)

Use the home-manager module for user-specific installation on macOS or Linux:

**macOS example:**

```nix
# flake.nix
{
  outputs = { self, nixpkgs, home-manager, neru, ... }: {
     homeConfigurations.your-username = home-manager.lib.homeManagerConfiguration {
       pkgs = nixpkgs.legacyPackages.aarch64-darwin;

       modules = [
         # Apply the Neru overlay
         {
           nixpkgs.overlays = [ neru.overlays.default ];
         }

         # Import the Neru module
         neru.homeManagerModules.default

         # Configure Neru
         {
           # Enable Neru
           services.neru.enable = true;

           # Optional: Use specific package version
           # services.neru.package = pkgs.neru; # This will use the latest version
           # services.neru.package = pkgs.neru-source; # This will build from source

           # Option A: Inline configuration
           services.neru.config = ''
              [hotkeys]
              "Primary+Shift+Space" = "hints left_click"
              "Primary+Shift+G" = "grid left_click"

              [general]
              excluded_apps = ["com.apple.Terminal"]
           '';

           # Option B: Use existing config file (takes precedence)
           # services.neru.configFile = ./path/to/config.toml;
         }
       ];
     };
  };
}
```

**Linux example:**

```nix
# flake.nix
{
  outputs = { self, nixpkgs, home-manager, neru, ... }: {
     homeConfigurations.your-username = home-manager.lib.homeManagerConfiguration {
       pkgs = nixpkgs.legacyPackages.x86_64-linux;

       modules = [
         # Apply the Neru overlay
         {
           nixpkgs.overlays = [ neru.overlays.default ];
         }

         # Import the Neru module
         neru.homeManagerModules.default

         # Configure Neru
         {
           # Enable Neru (uses pre-built artifact on Linux if available)
           services.neru.enable = true;

           # Optional: Inline configuration
           services.neru.config = ''
             [hotkeys]
             "Ctrl+Shift+Space" = "hints left_click"
             "Ctrl+Shift+G" = "grid left_click"
           '';

           # Optional: Use existing config file (takes precedence)
         }
       ];
     };
  };
}
```

**Module Options:**

- `services.neru.enable` - Enable Neru (default: `false`)
- `services.neru.package` - Package to use (default: `pkgs.neru`; uses release artifact on Linux, builds from source if unavailable)
- `services.neru.config` - Inline TOML configuration (default: uses `configs/default-config.toml`)
- `services.neru.configFile` - Path to existing config file (default: `null`, takes precedence over `config`)
- `services.neru.settings` - TOML configuration expressed as a Nix attribute set (default: `{}`, takes precedence over `config`)
- `services.neru.launchd.enable` - Enable the launchd agent on macOS (default: `true`)
- `services.neru.launchd.keepAlive` - Keep the launchd service alive on macOS (default: `true`)
- `services.neru.systemd.enable` - Enable the systemd user service on Linux (default: `true`)
- `services.neru.systemd.restart` - Systemd restart policy (default: `"on-failure"`)
- `services.neru.systemd.restartSec` - Seconds to wait before restarting (default: `5`)
- `services.neru.extraEnvironment` - Additional environment variables for the launchd or systemd service (default: `{}`; includes a sensible `PATH` with Nix binary directories and the user's Nix profile)

The module automatically:

- Installs Neru in user environment
- Creates `~/.config/neru/config.toml` (or uses your `configFile`)
- **macOS:** Creates a launchd user agent (if `launchd.enable` is `true`) with `KeepAlive`, `RunAtLoad = true`, and the configured environment
- **Linux:** Creates a systemd user service tied to `graphical-session.target` (if `systemd.enable` is `true`) with the configured environment
- Installs shell completions for bash, fish, and zsh

> [!NOTE]
> **macOS codesign:** You will need to codesign the Neru.app bundle in the nix store.
> Refer to the nix-darwin module above for an example.
> This is not needed for the default `pkgs.neru` (zip) package, which is pre-signed.

> [!IMPORTANT]
> On Linux, `pkgs.neru` uses the release artifact when available. Use `pkgs.neru-source` to build from source. If your nixpkgs doesn't ship a recent enough Go version, see [Patch Go Version](#patch-go-version) below.

> [!WARNING]
> **Default config uses cross-platform hotkeys.** The built-in default uses the `Primary+…` modifier, which maps to Cmd on macOS and Ctrl on Linux.

### Option 4: Using as an Overlay Only

If you prefer to manage the service yourself, you can just use the overlay:

> [!NOTE]
> Direct installation requires manual configuration and launch agent setup.

```nix
{
  outputs = { self, nixpkgs, neru, ... }: {
     darwinConfigurations.your-hostname = nix-darwin.lib.darwinSystem {
       modules = [
         {
           nixpkgs.overlays = [ neru.overlays.default ];
           environment.systemPackages = [ pkgs.neru ];
         }
       ];
     };
  };
}
```

Or install directly as a package:

```nix
{
  outputs = { self, nixpkgs, neru, ... }: {
     darwinConfigurations.your-hostname = nix-darwin.lib.darwinSystem {
       modules = [
         {
           environment.systemPackages = [
             neru.packages.aarch64-darwin.default
           ];
         }
       ];
     };
  };
}
```

Or with home-manager:

```nix
{
  home.packages = [ neru.packages.${system}.neru ];
}
```

### Configuration Examples

**Minimal setup (nix-darwin):**

```nix
{
  services.neru.enable = true;
}
```

**Custom hotkeys (home-manager):**

```nix
{
  services.neru.enable = true;
  services.neru.config = ''
     [hotkeys]
     "Primary+;" = "hints left_click"
     "Primary+'" = "grid left_click"
     "Primary+Shift+S" = "scroll"
  '';
}
```

**Custom hotkeys using `services.neru.settings`(home-manager):**

```nix
{
  services.neru.enable = true;
  services.neru.settings = {
	hotkeys = {
	  "Primary+;" = "hints left_click";
	  "Primary+'" = "grid left_click";
	  "Primary+Shift+S" = "scroll";
	};
  };
}
```

**Using external config file (home-manager):**

```nix
{
  services.neru.enable = true;
  services.neru.configFile = ./dotfiles/neru/config.toml;
}
```

### Updating

To update Neru, update your flake lock:

```bash
nix flake update neru
# Then rebuild your system/home configuration
```

### Patch Go Version

> [!NOTE]
> This is only required if you're using `nix`, you're using the `neru-source` package and nixpkgs is not on golang `1.26.4` yet.

```nix
package = pkgs.neru-source.overrideAttrs (_: {
  postPatch = ''
     substituteInPlace go.mod \
       --replace-fail "go 1.26.4" "go 1.25.5"

     # Verify it worked
     echo "=== go.mod after patch ==="
     grep "^go " go.mod || true
  '';
});
```

---

## Method 4: From Source

### Requirements

- Go 1.26+
- Xcode Command Line Tools
- Just command runner

### Build and install

`just install` builds Neru, runs `just dist` to assemble the layout a release zip
unpacks to under `build/dist/`, and hands that tree to the same installer a
`curl | bash` user runs. A source install lands in the same places as a release,
so the curl installer can update or remove it later.

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru

just install      # interactive: asks before the login service and other optional steps
just install -y   # accept every prompt
```

On Windows `just install` runs `scripts/install.ps1` through PowerShell, so the
installer's flags use their PowerShell spelling there (`-Yes`, `-NoService`).
Nothing in that path needs Bash: the release layout step runs as PowerShell
there too. `just` itself runs recipe lines through `sh` on Windows, which Git
for Windows provides.

A source build reports its version as a git describe string, so the installer
classifies it as a `source` install. Running the curl installer on top of it
later says so and asks before replacing it with a release, and `just install`
over a release install asks the same in reverse.

To undo it later, see [`just uninstall`](#just-uninstall).

### Build and install manually

If you would rather place the files yourself:

```bash
# macOS CLI only
just release
mv ./bin/neru /usr/local/bin/neru

# macOS app bundle
just build && just dist
mv ./build/dist/Neru.app /Applications/Neru.app
```

See [DEVELOPMENT.md](DEVELOPMENT.md) for detailed build options.

---

## Post-Installation

### 1. Grant Permissions

**Required:** Open System Settings → Privacy & Security → Accessibility → Add Neru

### 2. Start Neru

```bash
# App bundle
open -a Neru

# Or CLI
neru launch

# Or install as a login service for auto-startup: a launchd agent on macOS,
# a systemd user unit on Linux, a Task Scheduler task on Windows
neru services install
```

> [!NOTE]
> If Neru is already installed via nix-darwin, home-manager, or other methods, `services install` will detect the conflict and refuse to install. Check your existing configurations first.

### 3. Verify

```bash
neru --version
neru status  # Should show "running"
```

### 4. Configure

Neru loads config from `~/.config/neru/config.toml` (recommended). See [CONFIGURATION.md](CONFIGURATION.md) for the full search order.

**Get started:** Copy `configs/default-config.toml` to `~/.config/neru/config.toml`

See [CONFIGURATION.md](CONFIGURATION.md) for all options. Having issues? Check [TROUBLESHOOTING.md](TROUBLESHOOTING.md).

---

## Shell Completions

Neru provides shell completions for bash, zsh, and fish.

### Bash

```bash
neru completion bash > /usr/local/etc/bash_completion.d/neru
```

### Zsh

```bash
neru completion zsh > "${fpath[1]}/_neru"
```

### Fish

```bash
neru completion fish > ~/.config/fish/completions/neru.fish
```

---

## Troubleshooting

Install-time fixes (quarantine, PATH, permissions, Homebrew, Nix) live with all
the other fixes in [TROUBLESHOOTING.md](TROUBLESHOOTING.md) — start at
[Installation & Setup](TROUBLESHOOTING.md#installation--setup). One
Nix-specific note: the flake's release-artifact URL is arch-specific, so on
Intel Macs use `neru-darwin-amd64.zip` in place of the arm64 artifact.

---

## Uninstallation

### Install script

The same script removes what it installed. That covers the login service, the
app or binary, the PATH link, man pages and completions, plus the Start Menu
shortcut and PATH entry on Windows. Config, data and logs stay unless you add
`--purge`, and even then it asks before deleting each directory.

```bash
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --uninstall
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --uninstall --purge
```

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1))) -Uninstall
```

### Homebrew

```bash
brew uninstall --cask neru
```

### Nix

Remove the module from your configuration and rebuild.

### `just uninstall`

This runs the installer's `--uninstall` from the checkout you are in. It is the
same removal as above.

```bash
just uninstall            # interactive
just uninstall -y         # accept every prompt
just uninstall -y --purge # ...and delete your config and logs too
```

**Your config and logs are kept unless you pass `--purge`.** `-y` on its own can
never delete a hand-tuned `config.toml`. With `--purge` it asks before deleting
each directory.

On macOS, the Accessibility and Input Monitoring entries stay in System Settings →
Privacy & Security. Remove them by hand if you are not reinstalling. On Linux the
script leaves your `input` group membership alone, since other evdev tools may rely on it.

### Manual

<details>
<summary>macOS</summary>

```bash
# Stop and remove launchd service (if installed)
neru services uninstall

# Remove app bundle and CLI
rm -rf /Applications/Neru.app
rm /usr/local/bin/neru

# Remove completions and man pages
rm -f ~/.config/fish/completions/neru.fish ~/.zsh/completions/_neru \
      ~/.local/share/bash-completion/completions/neru
rm -f /usr/local/share/man/man1/neru*.1

# Remove configuration, data and logs
rm -rf ~/.config/neru ~/Library/Application\ Support/neru ~/Library/Logs/neru
```

</details>

<details>
<summary>Linux</summary>

```bash
# Stop and remove the systemd user service
systemctl --user disable --now neru.service
rm -f ~/.config/systemd/user/neru.service
systemctl --user daemon-reload

# Remove the binary
rm -f ~/.local/bin/neru

# Remove completions and man pages
rm -f ~/.local/share/bash-completion/completions/neru ~/.zsh/completions/_neru \
      ~/.config/fish/completions/neru.fish
rm -f ~/.local/share/man/man1/neru*.1

# Remove configuration, data and logs
rm -rf ~/.config/neru ~/.local/share/neru ~/.local/state/neru

# Only if nothing else needs it
sudo gpasswd -d "$USER" input
```

</details>

<details>
<summary>Windows (PowerShell)</summary>

```powershell
# Stop and remove the Task Scheduler task (if installed); an install made
# before `neru services` reached Windows used a Run key instead
neru services uninstall
Remove-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name Neru -ErrorAction SilentlyContinue
Stop-Process -Name neru -Force -ErrorAction SilentlyContinue

# Start Menu shortcut
Remove-Item "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Neru.lnk"

# Binary
Remove-Item "$env:LOCALAPPDATA\Programs\neru" -Recurse

# Configuration, data and logs
Remove-Item "$env:APPDATA\neru" -Recurse
Remove-Item "$env:LOCALAPPDATA\neru" -Recurse
```

Removing the PATH entry by hand is fiddly — `setx` truncates the value at 1024
characters and flattens other tools' `%VAR%` entries. Either use
`just uninstall`, or edit it through System Settings → *Edit environment
variables for your account*.

</details>
