# Installation Guide

This guide covers all installation methods for Neru on macOS.

## Prerequisites

- macOS 11.0 or later
- Accessibility permissions (granted after installation)

---

## Method 1: Homebrew (Recommended)

The easiest way to install Neru:

```bash
brew tap y3owk1n/tap
brew install --cask y3owk1n/tap/neru
```

**To update:**

```bash
brew upgrade --cask neru
```

**To uninstall:**

```bash
brew uninstall --cask neru
```

---

## Method 2: Nix Flake

Neru is available as a Nix flake with built-in support for nix-darwin and home-manager.

### Add Flake Input

Add Neru to your flake inputs:

```nix
# flake.nix
{
  inputs = {
    # ... other inputs
    neru.url = "github:y3owk1n/neru";
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
          neru.enable = true;

          # Optional: Use specific package version
          # neru.package = pkgs.neru;

          # Optional: Inline configuration
          neru.config = ''
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

- `neru.enable` - Enable Neru (default: `false`)
- `neru.package` - Package to use (default: `pkgs.neru`)
- `neru.config` - Inline TOML configuration (default: uses `configs/default-config.toml`)

The module automatically:

- Installs Neru system-wide
- Creates a launchd user agent
- Configures the agent to run at login with `KeepAlive = true` and `RunAtLoad = true`
- Installs shell completions for bash, fish, and zsh

### Option 2: home-manager Module (User-Level)

Use the home-manager module for user-specific installation:

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
          programs.neru.enable = true;

          # Optional: Use specific package version
          # programs.neru.package = pkgs.neru;

          # Option A: Inline configuration
          programs.neru.config = ''
            [hotkeys]
            "Cmd+Shift+Space" = "hints left_click"
            "Cmd+Shift+G" = "grid left_click"

            [general]
            excluded_apps = ["com.apple.Terminal"]
          '';

          # Option B: Use existing config file (takes precedence)
          # programs.neru.configFile = ./path/to/config.toml;
        }
      ];
    };
  };
}
```

**Module Options:**

- `programs.neru.enable` - Enable Neru (default: `false`)
- `programs.neru.package` - Package to use (default: `pkgs.neru`)
- `programs.neru.config` - Inline TOML configuration (default: uses `configs/default-config.toml`)
- `programs.neru.configFile` - Path to existing config file (default: `null`, takes precedence over `config`)

The module automatically:

- Installs Neru in user environment
- Creates `~/.config/neru/config.toml` (or uses your `configFile`)
- Creates a launchd user agent
- Configures the agent to run at login with `KeepAlive = true` and `RunAtLoad = true`
- Installs shell completions for bash, fish, and zsh

### Option 3: Using as an Overlay Only

If you prefer to manage the service yourself, you can just use the overlay:

> [!NOTE] Direct installation requires manual configuration and launch agent setup.

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
  neru.enable = true;
}
```

**Custom hotkeys (home-manager):**

```nix
{
  programs.neru.enable = true;
  programs.neru.config = ''
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
  programs.neru.enable = true;
  programs.neru.configFile = ./dotfiles/neru/config.toml;
}
```

### Updating

To update Neru, update your flake lock:

```bash
nix flake update neru
# Then rebuild your system/home configuration
```

---

## Method 3: From Source

### Requirements

- Go 1.25 or later
- Xcode Command Line Tools
- [Just](https://github.com/casey/just) (command runner)

### Build Steps

```bash
# Clone repository
git clone https://github.com/y3owk1n/neru.git
cd neru

# Build CLI only
just release

# Or build app bundle
just bundle

# Move to installation location
# For CLI:
mv ./bin/neru /usr/local/bin/neru

# For app bundle:
mv ./build/Neru.app /Applications/Neru.app
```

### Manual Build (without Just)

```bash
# Build with version info
go build \
  -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version=$(git describe --tags --always)" \
  -o bin/neru \
  ./cmd/neru
```

See [DEVELOPMENT.md](DEVELOPMENT.md) for more build options.

---

## Post-Installation

### 1. Grant Accessibility Permissions

**Required for Neru to function:**

1. Open **System Settings**
2. Navigate to **Privacy & Security → Accessibility**
3. Click the lock icon to make changes
4. Click **+** and add Neru
5. Ensure the checkbox is enabled

### 2. Start Neru

**For app bundle:**

```bash
open -a Neru
```

**For CLI:**

```bash
neru launch
```

**With custom config:**

```bash
neru launch --config /path/to/config.toml
```

### 3. Verify Installation

```bash
# Check version
neru --version

# Check status
neru status
```

Expected output:

```
Neru Status:
  Status: running
  Mode: idle
  Config: /Users/you/.config/neru/config.toml
```

---

## Configuration Setup

After installation, Neru looks for configuration in:

1. **~/.config/neru/config.toml** (XDG standard - recommended)
2. **~/Library/Application Support/neru/config.toml** (macOS convention)
3. Custom path via `--config` flag

**Start with default config:**

```bash
# Create config directory
mkdir -p ~/.config/neru

# Copy default config
curl -o ~/.config/neru/config.toml \
  https://raw.githubusercontent.com/y3owk1n/neru/main/configs/default-config.toml
```

See [CONFIGURATION.md](CONFIGURATION.md) for detailed configuration options.

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

---

## Next Steps

- Read [CONFIGURATION.md](CONFIGURATION.md) to customize Neru
- Check [CLI.md](CLI.md) for command-line usage
- Review [TROUBLESHOOTING.md](TROUBLESHOOTING.md) if you encounter issues
