# Installation Guide

This guide covers installation methods for Neru, with the most complete support on macOS.

> [!NOTE]
> macOS is the primary supported platform. Linux source builds are available through the Nix flake and direct builds, but there are no official Linux release artifacts yet. See the [Platform Support section in README.md](../README.md#💻-platform-support) for details.

## Requirements

- macOS 11.0 or later
- Accessibility permissions (granted during setup)

---

## Method 1: Homebrew (Recommended)

> [!NOTE]
> The homebrew tap is maintained in another repo: [y3owk1n/homebrew-tap](https://github.com/y3owk1n/homebrew-tap)
> If there's a problem with the tap, please open an issue in that repo or even better, a PR.

```bash
brew tap y3owk1n/tap
brew install --cask y3owk1n/tap/neru
```

**Update:** `brew upgrade --cask neru`
**Uninstall:** `brew uninstall --cask neru`

---

## Method 2: Nix Flake

Neru is available as a Nix flake with built-in support for nix-darwin (macOS), NixOS (Linux), and home-manager (both platforms).

On macOS, `pkgs.neru` uses the published release zip and `pkgs.neru-source` builds from source.
On Linux, `pkgs.neru` and the flake `default` package both build from source because there is no official Linux release artifact yet.

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
             "Cmd+Shift+Space" = "hints left_click"
             "Cmd+Shift+G" = "grid left_click"

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

The module automatically:

- Installs Neru system-wide
- Creates a launchd user agent
- Configures the agent to run at login with `KeepAlive = true` and `RunAtLoad = true`
- Installs shell completions for bash, fish, and zsh

> [!NOTE]
> **Codesign for source builds (`neru-source`):** Add this to your nix-darwin configuration:
>
> ```nix
> # In your nix-darwin module
> system.activationScripts.postActivation.text = ''
>   # codesign Neru.app
>   if [ -e "/Users/${username}/Applications/Home Manager Apps/Neru.app" ]; then
>      /usr/bin/codesign --force --deep --sign - --timestamp=none "/Users/${username}/Applications/Home Manager Apps/Neru.app"
>      echo "Codesign Neru.app..."
>   fi
> '';
> ```
>
> This is not needed for the default `pkgs.neru` (zip) package, which is pre-signed.

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
            # services.neru.package = pkgs.neru; # This will build from source on Linux

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
- `services.neru.package` - Package to use (default: `pkgs.neru`, always builds from source on Linux)
- `services.neru.config` - Inline TOML configuration (default: uses `configs/default-config.toml`)
- `services.neru.configFile` - Path to existing config file (default: `null`, takes precedence over `config`)

The module automatically:

- Installs Neru system-wide
- Creates a systemd user service tied to `graphical-session.target`
- Configures automatic restart on failure

> [!IMPORTANT]
> **Linux always builds from source.** There are no official pre-built Linux release artifacts yet. On Linux, `pkgs.neru` is equivalent to `pkgs.neru-source` — both build from source. If your nixpkgs doesn't ship a recent enough Go version, see [Patch Go Version](#patch-go-version) below.

> [!WARNING]
> **Default config uses macOS hotkeys.** The built-in default configuration ships with `Cmd+Shift+…` hotkeys, which map to the Super/Meta key on Linux. Linux users should override the `[hotkeys]` section with `Ctrl+…` shortcuts (as shown in the example above) or use the cross-platform `Primary` modifier, which maps to Cmd on macOS and Ctrl on Linux.

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
             "Cmd+Shift+Space" = "hints left_click"
             "Cmd+Shift+G" = "grid left_click"

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
           # Enable Neru (always builds from source on Linux)
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
- `services.neru.package` - Package to use (default: `pkgs.neru`; on macOS uses the release zip, on Linux always builds from source)
- `services.neru.config` - Inline TOML configuration (default: uses `configs/default-config.toml`)
- `services.neru.configFile` - Path to existing config file (default: `null`, takes precedence over `config`)
- `services.neru.launchd.enable` - Enable the launchd agent on macOS (default: `true`)
- `services.neru.launchd.keepAlive` - Keep the launchd service alive on macOS (default: `true`)
- `services.neru.systemd.enable` - Enable the systemd user service on Linux (default: `true`)
- `services.neru.systemd.restart` - Systemd restart policy (default: `"on-failure"`)
- `services.neru.systemd.restartSec` - Seconds to wait before restarting (default: `5`)

The module automatically:

- Installs Neru in user environment
- Creates `~/.config/neru/config.toml` (or uses your `configFile`)
- **macOS:** Creates a launchd user agent (if `launchd.enable` is `true`) with `KeepAlive` and `RunAtLoad = true`
- **Linux:** Creates a systemd user service tied to `graphical-session.target` (if `systemd.enable` is `true`)
- Installs shell completions for bash, fish, and zsh

> [!NOTE]
> **macOS codesign:** You will need to codesign the Neru.app bundle in the nix store.
> Refer to the nix-darwin module above for an example.
> This is not needed for the default `pkgs.neru` (zip) package, which is pre-signed.

> [!IMPORTANT]
> **Linux always builds from source.** On Linux, `pkgs.neru` is equivalent to `pkgs.neru-source` — there are no official pre-built Linux release artifacts yet. If your nixpkgs doesn't ship a recent enough Go version, see [Patch Go Version](#patch-go-version) below.

> [!WARNING]
> **Default config uses macOS hotkeys.** If you don't provide an inline `config` or `configFile`, the module uses the built-in default which has `Cmd+Shift+…` hotkeys (Super/Meta on Linux). Linux users should override the `[hotkeys]` section with `Ctrl+…` or `Primary+…` shortcuts. The `Primary` modifier maps to Cmd on macOS and Ctrl on Linux.

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
     "Cmd+;" = "hints left_click"
     "Cmd+'" = "grid left_click"
     "Cmd+Shift+S" = "scroll"
  '';
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
> This is only required if you're using `nix`, you're using the `neru-source` package and nixpkgs is not on golang `1.26.1` yet.

> This is required if you're using `nix` and nixpkgs is not on golang `1.26.1` yet. It applies to `neru-source` on macOS and **all** Linux builds (since `pkgs.neru` on Linux always builds from source).

```nix
package = pkgs.neru-source.overrideAttrs (_: {
  postPatch = ''
     substituteInPlace go.mod \
       --replace-fail "go 1.26.1" "go 1.25.5"

     # Verify it worked
     echo "=== go.mod after patch ==="
     grep "^go " go.mod || true
  '';
});
```

---

## Method 3: From Source

### Requirements

- Go 1.26+
- Xcode Command Line Tools
- Just command runner

### Build

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru

# Build CLI
just release
mv ./bin/neru /usr/local/bin/neru

# Or build app bundle
just bundle
mv ./build/Neru.app /Applications/Neru.app
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

# Or install as launchd service for auto-startup
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

### "Neru wants to control this computer using accessibility features"

This is normal. Click **OK** and grant permissions in System Settings.

### Command not found: neru

If using the CLI build, ensure the binary is in your PATH:

```bash
# Add to ~/.zshrc or ~/.bashrc
export PATH="/usr/local/bin:$PATH"
```

### Permission denied

Make the binary executable:

```bash
chmod +x /usr/local/bin/neru
```

### App won't open (macOS quarantine)

macOS may quarantine apps from unidentified developers:

```bash
xattr -cr /Applications/Neru.app
```

Then try opening again.

### Nix build fails

Ensure you're on an Apple Silicon Mac (arm64). For Intel Macs, change the URL to:

```nix
url = "https://github.com/y3owk1n/neru/releases/download/v${version}/neru-darwin-amd64.zip";
```

---

## Uninstallation

### Homebrew

```bash
brew uninstall --cask neru
```

### Manual

```bash
# Stop and remove launchd service (if installed)
neru services uninstall

# Remove app bundle
rm -rf /Applications/Neru.app

# Remove CLI
rm /usr/local/bin/neru

# Remove configuration
rm -rf ~/.config/neru
rm -rf ~/Library/Application\ Support/neru

# Remove logs
rm -rf ~/Library/Logs/neru
```

### Nix

Remove the module from your configuration and rebuild.
