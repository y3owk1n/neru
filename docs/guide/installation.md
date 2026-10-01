# Installation

Every way to install, update and remove Neru. Once it is installed, continue
with [Getting started](getting-started.md) for permissions and your first
config.

## Requirements

- **macOS** 14 or later.
- **Linux** (beta): X11, or a Wayland compositor from the list in
  [Linux setup](linux.md). The release binary links the X11, Wayland, tesseract
  and pipewire libraries dynamically, so those must be installed.
- **Windows** (beta): Windows 10 or later.

What works on each platform is in
[Platform support](../reference/platform-support.md#capability-matrix).

## Install script

It is the recommended method on every platform. One command installs everything a
release ships: the binary, `Neru.app` on macOS, man pages, shell completions,
and, if you say yes, the login service. Run the same command again to update.

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
version, or follow **nightly**, which is rebuilt from every push to `main`:

```bash
# a specific stable release
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --version v1.52.0

# nightly
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --channel nightly
```

```powershell
# flags need a script block. The NERU_CHANNEL, NERU_VERSION and NERU_YES variables work with plain `irm | iex`
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1))) -Channel nightly
```

Running the script again updates in place on the channel you already have. A
stable install moves to the newest release, or the script tells you it is
current. A nightly install always gets the latest nightly build. Asking for the
other channel is a switch. The script reads the installed version, says what it
found, and asks before it replaces one channel with the other.

### Flags

| bash                        | PowerShell        | Env            | Effect                                                              |
| :-------------------------- | :---------------- | :------------- | :------------------------------------------------------------------ |
| `--channel stable\|nightly` | `-Channel`        | `NERU_CHANNEL` | Release channel. Default: the installed one, else stable            |
| `--version vX.Y.Z`          | `-Version`        | `NERU_VERSION` | Pin a stable release (implies stable)                               |
| `--from DIR`                | `-From`           | `NERU_FROM`    | Install a local `just dist` tree instead of downloading. `just install` uses this |
| `--bin-dir DIR`             |                   | `NERU_BIN_DIR` | Where `neru` goes. Default `/usr/local/bin` (macOS), `~/.local/bin` (Linux) |
| `--app-dir DIR`             |                   | `NERU_APP_DIR` | macOS: where `Neru.app` goes. Default `/Applications`               |
| `--no-service`              | `-NoService`      |                | Never register or start the login service                           |
| `--no-completions`          | `-NoCompletions`  |                | Skip shell completions                                              |
| `--no-man`                  |                   |                | Skip man pages                                                      |
| `--force`                   | `-Force`          |                | Reinstall the same version                                          |
| `--uninstall`               | `-Uninstall`      |                | Remove everything a previous run installed. Config is kept          |
| `--purge`                   | `-Purge`          |                | With uninstall, also delete config, data and logs (each confirmed)  |
| `-y`, `--yes`               | `-Yes`            | `NERU_YES=1`   | Accept every prompt. Required when no terminal is attached          |

### What it does

On macOS it copies `Neru.app` to `/Applications`, symlinks `neru` into
`/usr/local/bin`, puts man pages in `/usr/local/share/man/man1`, and writes
completions for whichever of bash, zsh and fish you have. It asks for sudo only
when one of those directories is not writable, and says so before the password
prompt. Say yes to the last question and it registers the launchd login agent.

On Linux the script runs the downloaded binary once before it installs
anything. When a library is missing it lists the names, points at
[Linux setup](linux.md), and stops without touching your system. Once that
passes it copies `neru` to `~/.local/bin`, man pages to
`~/.local/share/man/man1`, and completions to the usual per-user paths. It
offers the systemd user service and, when you are not already in it, the
`input` group that Wayland keyboard capture needs.

On Windows it puts `neru.exe` under `%LOCALAPPDATA%\Programs\neru`, adds that
directory to your user PATH, creates a Start Menu shortcut, and offers
PowerShell completion in your profile and a Task Scheduler logon task.
PowerShell does not load profiles under the default `Restricted` execution
policy, so the script offers to set `RemoteSigned` for your user account first,
and skips completion if you decline.

For zsh, the script writes `~/.zsh/completions/_neru` and prints the `fpath`
line to add to `~/.zshrc` if that directory is not on your `fpath` yet.

When a login service is already registered, the script unloads it before it
replaces the binary and registers it again afterwards. It refuses to run over a
Homebrew or Nix-managed install and prints the command to use instead.

## Homebrew

macOS only. The tap lives in [y3owk1n/homebrew-tap](https://github.com/y3owk1n/homebrew-tap),
so report tap problems there.

Stable and nightly cannot be installed side by side. Uninstall one before
installing the other.

```bash
brew tap y3owk1n/tap

# stable
brew install --cask y3owk1n/tap/neru
brew upgrade --cask y3owk1n/tap/neru

# nightly (--greedy is required, or brew skips rolling releases)
brew install --cask y3owk1n/tap/neru-nightly
brew upgrade --cask --greedy y3owk1n/tap/neru-nightly
```

## Prebuilt binaries

Download a zip from [GitHub Releases](https://github.com/y3owk1n/neru/releases/latest):

| Platform | Architecture  | File                     |
| :------- | :------------ | :----------------------- |
| macOS    | Apple Silicon | `neru-darwin-arm64.zip`  |
| macOS    | Intel         | `neru-darwin-amd64.zip`  |
| Linux    | x86_64        | `neru-linux-amd64.zip`   |
| Linux    | ARM64         | `neru-linux-arm64.zip`   |
| Windows  | x86_64        | `neru-windows-amd64.zip` |
| Windows  | ARM64         | `neru-windows-arm64.zip` |

Every archive ships a `.sha256` file. Releases from v1.51.0 on also carry a
signed build provenance attestation, which proves the zip was built by this
repo's release workflow:

```bash
gh attestation verify neru-darwin-arm64.zip --repo y3owk1n/neru
```

Unpack it and put `neru`, or `Neru.app` on macOS, wherever you like. A manual
install gets no completions and no login service, so add them yourself with
[Shell completions](#shell-completions) and [Login service](#login-service).
Pointing the install script at the unpacked tree with `--from DIR` does all of
that for you.

## Nix

The flake provides modules for nix-darwin, NixOS and home-manager, and an
overlay with two packages:

- `pkgs.neru`: the published release zip for your system (default).
- `pkgs.neru-source`: built from the flake's source.

Add the input:

```nix
# flake.nix
{
  inputs.neru.url = "github:y3owk1n/neru";
}
```

Each module needs the overlay and its own import:

```nix
modules = [
  { nixpkgs.overlays = [ neru.overlays.default ]; }
  neru.darwinModules.default        # or neru.nixosModules.default, neru.homeManagerModules.default
  {
    services.neru.enable = true;
    services.neru.settings = {
      hotkeys = {
        "Primary+Shift+Space" = "hints --action left_click";
        "Primary+Shift+G" = "grid --action left_click";
      };
    };
  }
];
```

### Module options

All three modules share these options:

| Option                          | Default                     | Description                                              |
| :------------------------------ | :-------------------------- | :------------------------------------------------------- |
| `services.neru.enable`          | `false`                     | Install Neru and its login service                       |
| `services.neru.package`         | `pkgs.neru`                 | Package to use. `pkgs.neru-source` builds from source    |
| `services.neru.settings`        | `{}`                        | Config as a Nix attribute set, written out as TOML       |
| `services.neru.config`          | `configs/default-config.toml` | Config as an inline TOML string                        |
| `services.neru.configFile`      | `null`                      | Path to an existing `config.toml`                        |
| `services.neru.extraEnvironment`| `{}`                        | Extra environment for the service. Merged over a default `PATH` with the Nix binary directories. Setting `PATH` replaces it |

Set only one of `settings`, `config` and `configFile`. The modules fail the
build when more than one is set.

Service options per module:

| Module       | Options                                                                               | Service |
| :----------- | :------------------------------------------------------------------------------------ | :------ |
| nix-darwin   | `launchd.enable` (`true`), `launchd.keepAlive` (`true`)                               | launchd user agent, `RunAtLoad` |
| NixOS        | `systemd.restart` (`"on-failure"`), `systemd.restartSec` (`5`)                        | systemd user service on `graphical-session.target` |
| home-manager | `launchd.enable`, `launchd.keepAlive` on macOS. `systemd.enable` (`true`), `systemd.restart`, `systemd.restartSec` on Linux | Same as above, and writes `~/.config/neru/config.toml` |

> [!WARNING]
> When you set no config, the modules use `configs/default-config.toml`, which
> binds `Primary+Shift+...` hotkeys. On Linux `Primary` is `Ctrl`, so
> `Ctrl+Shift+C` is taken from your terminal's copy. Set your own
> `[hotkeys]` on Linux, or an empty table to leave binding to your compositor.

### Package only

To manage the service yourself, install the package without a module:

```nix
# nix-darwin or NixOS, with the overlay applied
environment.systemPackages = [ pkgs.neru ];

# home-manager, without the overlay
home.packages = [ neru.packages.${system}.default ];   # or .source to build from source
```

### Shell completions under Nix

The packages install bash, zsh and fish completions on macOS only. On Linux,
add them by hand with [Shell completions](#shell-completions).

### Codesigning source builds on macOS

`pkgs.neru` is pre-signed. `pkgs.neru-source` carries only the Go linker's
signature, without the hardened runtime entitlements. Sign it after activation
with the entitlements file bundled at `Contents/Resources/Neru.entitlements`.
For nix-darwin:

```nix
{
  system.activationScripts.postActivation.text = ''
    app="/Applications/Nix Apps/Neru.app"
    if [ -e "$app" ]; then
      /usr/bin/codesign --force --sign - \
        --entitlements "$app/Contents/Resources/Neru.entitlements" \
        --options runtime --timestamp=none "$app"
    fi
  '';
}
```

For home-manager, run the same command from
`home.activation.signNeru = lib.hm.dag.entryAfter [ "copyApps" ] '' ... ''`
with `app="$HOME/Applications/Home Manager Apps/Neru.app"`.

### Older Go in nixpkgs

`pkgs.neru-source` needs the Go version in `go.mod` (1.26.5). If your nixpkgs
ships an older Go, lower the requirement:

```nix
services.neru.package = pkgs.neru-source.overrideAttrs (_: {
  postPatch = ''
    substituteInPlace go.mod --replace-fail "go 1.26.5" "go 1.25.5"
  '';
});
```

### Updating

```bash
nix flake update neru
```

Then rebuild your system or home configuration.

## From source

### Requirements

| Platform | Needs                                                                                               |
| :------- | :-------------------------------------------------------------------------------------------------- |
| All      | Go 1.26+ and [just](https://github.com/casey/just)                                                  |
| macOS    | Xcode Command Line Tools (`xcode-select --install`) for the Objective-C bridge                      |
| Linux    | A C compiler and the `-dev`/`-devel` packages the CGO build links against, listed in [Development setup](../contributing/development.md#development-setup) |
| Windows  | Git for Windows, whose `sh` runs the `just` recipes. The build has CGO off, so no C compiler        |

### Build and install

`just install` builds Neru, assembles the release layout under `build/dist/`,
and hands that tree to the same install script described above. A source
install lands in the same places as a release, so the script can update or
remove it later.

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru

just install      # asks before the login service and other optional steps
just install -y   # accept every prompt
```

On Windows `just install` runs `scripts/install.ps1`, so pass the installer's
flags in their PowerShell spelling (`-Yes`, `-NoService`).

A source build reports a git describe string as its version, so the script
treats it as a `source` install. Running the release installer over it later
asks before replacing it, and `just install` over a release install asks the
same.

Build options beyond this are in the
[Development guide](../contributing/development.md).

## Shell completions

The install script and `just install` write completions for you, and so do the
Nix packages on macOS. For any other install, generate them with
`neru completion`:

```bash
neru completion bash > ~/.local/share/bash-completion/completions/neru
neru completion zsh  > ~/.zsh/completions/_neru     # needs ~/.zsh/completions on your fpath
neru completion fish > ~/.config/fish/completions/neru.fish
```

```powershell
neru completion powershell | Out-String | Invoke-Expression   # add to your $PROFILE to keep it
```

## Login service

The install script offers to register Neru as a login service. For other
installs, or to add it later:

```bash
neru services install
```

That is a launchd agent on macOS, a systemd user unit on Linux and a Task
Scheduler task on Windows. It refuses when Homebrew, Nix or another package
manager already manages the service. The full command is in the
[CLI reference](../reference/cli.md#neru-services).

## Set up with an agent

The repo ships two skills for coding agents such as Claude Code, Codex and
Cursor. `neru-ask` answers what Neru can do and which command or option does
it. `neru-setup-config` writes, validates and applies your config file. Both
read the help, man pages and docs of the installed version, so no checkout is
needed.

```bash
npx skills add y3owk1n/neru --skill neru-ask --skill neru-setup-config
```

Add `-g` to install them for every project instead of the current one.

## Troubleshooting

Quarantine, PATH, Homebrew and missing-library fixes are in
[Troubleshooting](troubleshooting.md#installation--setup).

## Uninstallation

### Install script

The same script removes what it installed: the login service, the app or
binary, the PATH link, man pages and completions, plus the Start Menu shortcut
and PATH entry on Windows. Config, data and logs stay unless you add
`--purge`, and even then it asks before deleting each directory.

```bash
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --uninstall
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --uninstall --purge
```

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1))) -Uninstall
```

From a checkout, `just uninstall` runs the same removal:

```bash
just uninstall            # interactive
just uninstall -y         # accept every prompt, config kept
just uninstall -y --purge # also delete config and logs, each confirmed
```

On macOS the Accessibility and Screen Recording entries stay in System
Settings, under Privacy & Security. Remove them by hand if you are not
reinstalling. On Linux the script leaves your `input` group membership alone,
since other evdev tools may rely on it.

### Homebrew

```bash
brew uninstall --cask y3owk1n/tap/neru    # or neru-nightly
```

### Nix

Remove the module or package from your configuration and rebuild.

### Manual

<details>
<summary>macOS</summary>

```bash
neru services uninstall
rm -rf /Applications/Neru.app
rm /usr/local/bin/neru
rm -f ~/.config/fish/completions/neru.fish ~/.zsh/completions/_neru \
      ~/.local/share/bash-completion/completions/neru
rm -f /usr/local/share/man/man1/neru*.1
rm -rf ~/.config/neru ~/Library/Application\ Support/neru ~/Library/Logs/neru
```

</details>

<details>
<summary>Linux</summary>

```bash
systemctl --user disable --now neru.service
rm -f ~/.config/systemd/user/neru.service
systemctl --user daemon-reload
rm -f ~/.local/bin/neru
rm -f ~/.local/share/bash-completion/completions/neru ~/.zsh/completions/_neru \
      ~/.config/fish/completions/neru.fish
rm -f ~/.local/share/man/man1/neru*.1
rm -rf ~/.config/neru ~/.local/share/neru ~/.local/state/neru

# only if nothing else needs it
sudo gpasswd -d "$USER" input
```

</details>

<details>
<summary>Windows (PowerShell)</summary>

```powershell
neru services uninstall
Stop-Process -Name neru -Force -ErrorAction SilentlyContinue
Remove-Item "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Neru.lnk"
Remove-Item "$env:LOCALAPPDATA\Programs\neru" -Recurse
Remove-Item "$env:APPDATA\neru" -Recurse
Remove-Item "$env:LOCALAPPDATA\neru" -Recurse
```

Removing the PATH entry by hand is fiddly, because `setx` truncates the value
at 1024 characters and flattens other tools' `%VAR%` entries. Use
`just uninstall`, or edit it under *Edit environment variables for your
account* in Settings.

</details>
