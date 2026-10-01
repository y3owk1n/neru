# Installation

Every way to install, update and remove Neru. Supported systems are in
[Platform support](../reference/platform-support.md#platform-status). After
installing, continue with [Getting started](getting-started.md).

## Install script

The recommended method on every platform. Run it again to update.

```bash
# macOS and Linux
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash
```

```powershell
# Windows
irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1 | iex
```

It installs the binary, `Neru.app` on macOS, man pages, shell completions and,
if you accept, the login service. It checks every download against its
published `.sha256` file.

> [!NOTE]
> On Linux, install the [runtime libraries](linux.md#runtime-libraries)
> first. The script checks for them and stops with a list if any are missing.

Every flag, and where each file goes, is in
[Install script reference](#install-script-reference).

### Channels and versions

The default is the latest **stable** release. **Nightly** is rebuilt from
every push to `main`. A rerun stays on the installed channel, and the script
asks before it switches channels.

```bash
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --version v1.52.0
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --channel nightly
```

```powershell
# flags need a script block. NERU_CHANNEL, NERU_VERSION and NERU_YES work with plain `irm | iex`
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1))) -Channel nightly
```

## Homebrew

macOS only. Report tap problems at
[y3owk1n/homebrew-tap](https://github.com/y3owk1n/homebrew-tap). Stable and
nightly cannot be installed side by side, so uninstall one first.

```bash
brew install --cask y3owk1n/tap/neru            # stable, upgrade with brew upgrade --cask
brew install --cask y3owk1n/tap/neru-nightly    # nightly, upgrade with brew upgrade --cask --greedy
```

Nightly upgrades need `--greedy`, or brew skips rolling releases.

## Prebuilt binaries

Download `neru-<os>-<arch>.zip` from
[GitHub Releases](https://github.com/y3owk1n/neru/releases/latest). `<os>` is
`darwin`, `linux` or `windows`, and `<arch>` is `arm64` (Apple Silicon, ARM64)
or `amd64` (Intel, x86_64). Every archive has a `.sha256` file, and releases
from v1.51.0 on carry a build provenance attestation:

```bash
gh attestation verify neru-darwin-arm64.zip --repo y3owk1n/neru
```

Unpack `neru`, or `Neru.app` on macOS, anywhere you like. A manual install has
no completions or login service. Add them with
[Shell completions](#shell-completions) and [Login service](#login-service),
or run the install script with `--from DIR` on the unpacked tree.

## Nix

The flake provides modules for nix-darwin, NixOS and home-manager, and an
overlay with two packages:

- `pkgs.neru`: the release zip for your system (default).
- `pkgs.neru-source`: built from the flake's source.

Add the input `inputs.neru.url = "github:y3owk1n/neru";` to `flake.nix`,
then the overlay and one module:

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

Shared by all three modules. Set only one of `settings`, `config` and
`configFile`, or the build fails.

| Option                          | Default                     | Description                                              |
| :------------------------------ | :-------------------------- | :------------------------------------------------------- |
| `services.neru.enable`          | `false`                     | Install Neru and its login service                       |
| `services.neru.package`         | `pkgs.neru`                 | Package to use. `pkgs.neru-source` builds from source    |
| `services.neru.settings`        | `{}`                        | Config as a Nix attribute set, written out as TOML       |
| `services.neru.config`          | `configs/default-config.toml` | Config as an inline TOML string                        |
| `services.neru.configFile`      | `null`                      | Path to an existing `config.toml`                        |
| `services.neru.extraEnvironment`| `{}`                        | Extra environment for the service. Merged over a default `PATH` with the Nix binary directories. Setting `PATH` replaces it |

Service options per module:

| Module       | Options                                                                               | Service |
| :----------- | :------------------------------------------------------------------------------------ | :------ |
| nix-darwin   | `launchd.enable` (`true`), `launchd.keepAlive` (`true`)                               | launchd user agent, `RunAtLoad` |
| NixOS        | `systemd.restart` (`"on-failure"`), `systemd.restartSec` (`5`)                        | systemd user service on `graphical-session.target` |
| home-manager | `launchd.enable`, `launchd.keepAlive` on macOS. `systemd.enable` (`true`), `systemd.restart`, `systemd.restartSec` on Linux | Same as above, and writes `~/.config/neru/config.toml` |

> [!WARNING]
> With no config set, the modules use `configs/default-config.toml`, which
> binds `Primary+Shift+...` hotkeys. On Linux `Primary` is `Ctrl`, so
> `Ctrl+Shift+C` shadows your terminal's copy. On Linux, set your own
> `[hotkeys]`, or an empty table to leave binding to your compositor.

### Nix notes

- **Package only:** to manage the service yourself, skip the module and add
  `pkgs.neru` to `environment.systemPackages` (nix-darwin or NixOS, with the
  overlay), or `neru.packages.${system}.default` to `home.packages`
  (home-manager, no overlay). Use `.source` to build from source.
- **Completions:** the packages install bash, zsh and fish completions on
  macOS only. On Linux, add them with [Shell completions](#shell-completions).
- **Updating:** run `nix flake update neru`, then rebuild.
- **Older Go in nixpkgs:** `pkgs.neru-source` needs the Go version in `go.mod`
  (1.26.5). Lower it if your nixpkgs is older:

  ```nix
  services.neru.package = pkgs.neru-source.overrideAttrs (_: {
    postPatch = ''
      substituteInPlace go.mod --replace-fail "go 1.26.5" "go 1.25.5"
    '';
  });
  ```

- **Codesigning `pkgs.neru-source` on macOS:** `pkgs.neru` is pre-signed.
  `pkgs.neru-source` has only the Go linker's signature, without the hardened
  runtime entitlements. Sign it after activation with the bundled
  `Contents/Resources/Neru.entitlements`. For nix-darwin:

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

## From source

| Platform | Needs                                                                                               |
| :------- | :-------------------------------------------------------------------------------------------------- |
| All      | Go 1.26+ and [just](https://github.com/casey/just)                                                  |
| macOS    | Xcode Command Line Tools (`xcode-select --install`) for the Objective-C bridge                      |
| Linux    | A C compiler and the `-dev`/`-devel` packages the CGO build links against, listed in [Development setup](../contributing/development.md#development-setup) |
| Windows  | Git for Windows, whose `sh` runs the `just` recipes. The build has CGO off, so no C compiler        |

`just install` builds Neru into `build/dist/` and runs the install script on
that tree, so a source install lands in the same places as a release.

```bash
git clone https://github.com/y3owk1n/neru.git && cd neru
just install      # asks before the login service and other optional steps
just install -y   # accept every prompt
```

On Windows `just install` runs `scripts/install.ps1`, so use the PowerShell
flag spelling (`-Yes`, `-NoService`). A source build reports a git describe
version, so the script treats it as a `source` install and asks before a
release replaces it, or it replaces a release. Other build options are in the
[Development guide](../contributing/development.md).

## Shell completions

The install script, `just install` and the Nix packages on macOS write
completions for you. Otherwise run `neru completion`:

```bash
neru completion bash > ~/.local/share/bash-completion/completions/neru
neru completion zsh  > ~/.zsh/completions/_neru     # needs ~/.zsh/completions on your fpath
neru completion fish > ~/.config/fish/completions/neru.fish
```

```powershell
neru completion powershell | Out-String | Invoke-Expression   # add to your $PROFILE to keep it
```

## Login service

The install script offers this. To add it later:

```bash
neru services install
```

This registers a launchd agent on macOS, a systemd user unit on Linux, or a
Task Scheduler task on Windows. It refuses when Homebrew, Nix or another
package manager manages the service. See
[`neru services`](../reference/cli.md#neru-services).

## Set up with an agent

Two skills for coding agents such as Claude Code, Codex and Cursor.
`neru-ask` answers what Neru can do and which command or option does it.
`neru-setup-config` writes, validates and applies your config. Both read the
installed version's help, man pages and docs, so no checkout is needed.

```bash
npx skills add y3owk1n/neru --skill neru-ask --skill neru-setup-config
```

Add `-g` to install them for every project.

## Install script reference

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

### What it installs

| Platform | Binary and app | Man pages | Completions | Login service |
| :------- | :------------- | :-------- | :---------- | :------------ |
| macOS    | `Neru.app` in `/Applications`, `neru` symlinked into `/usr/local/bin` | `/usr/local/share/man/man1` | bash, zsh and fish, whichever you have | launchd agent |
| Linux    | `~/.local/bin/neru` | `~/.local/share/man/man1` | per-user bash, zsh and fish paths | systemd user service |
| Windows  | `%LOCALAPPDATA%\Programs\neru\neru.exe`, added to your user PATH, plus a Start Menu shortcut | none | PowerShell profile | Task Scheduler logon task |

- **macOS:** it asks for sudo only when a target directory is not writable.
- **Linux:** it runs the downloaded binary once first. If a library is
  missing, it lists the names, points at [Linux setup](linux.md), and stops
  without changing anything. It offers the `input` group that Wayland
  keyboard capture needs.
- **Windows:** it offers to set the `RemoteSigned` execution policy for your
  user, since PowerShell loads no profile under `Restricted`. Decline and it
  skips completion.
- **zsh:** it writes `~/.zsh/completions/_neru` and prints the `fpath` line
  for `~/.zshrc` if that directory is not on your `fpath`.
- **Any:** it stops a registered login service while it replaces the binary.
  It refuses to run over a Homebrew or Nix install and prints the command to
  use instead.

## Uninstallation

### Install script

The script removes what it installed: the login service, the app or binary,
the PATH link, man pages, completions, and on Windows the Start Menu shortcut
and PATH entry. Config, data and logs stay unless you add `--purge`, which
asks before deleting each directory.

```bash
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --uninstall   # add --purge to delete config, data and logs
just uninstall    # from a checkout, takes the same -y and --purge
```

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1))) -Uninstall
```

On macOS, the Accessibility and Screen Recording entries stay in System
Settings, under Privacy & Security. Remove them by hand if you are not
reinstalling. On Linux, the script leaves your `input` group membership,
since other evdev tools may need it.

### Homebrew and Nix

Homebrew: `brew uninstall --cask y3owk1n/tap/neru`, or `neru-nightly`. Nix:
remove the module or package from your configuration and rebuild.

### Manual

macOS:

```bash
neru services uninstall
rm -rf /Applications/Neru.app
rm /usr/local/bin/neru
rm -f ~/.config/fish/completions/neru.fish ~/.zsh/completions/_neru \
      ~/.local/share/bash-completion/completions/neru
rm -f /usr/local/share/man/man1/neru*.1
rm -rf ~/.config/neru ~/Library/Application\ Support/neru ~/Library/Logs/neru
```

Linux:

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

Windows:

```powershell
neru services uninstall
Stop-Process -Name neru -Force -ErrorAction SilentlyContinue
Remove-Item "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Neru.lnk"
Remove-Item "$env:LOCALAPPDATA\Programs\neru" -Recurse
Remove-Item "$env:APPDATA\neru" -Recurse
Remove-Item "$env:LOCALAPPDATA\neru" -Recurse
```

Remove the PATH entry under *Edit environment variables for your account* in
Settings, or use `just uninstall`. Avoid `setx`, which truncates the value at
1024 characters and flattens other tools' `%VAR%` entries.
