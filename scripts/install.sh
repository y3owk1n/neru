#!/usr/bin/env bash
#
# Neru installer for macOS and Linux. Downloads a release build, verifies it,
# and installs the binary, man pages, shell completions and login service.
# Running it again updates in place, keeping the channel already installed
# unless told otherwise.
#
#   curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- --channel nightly
#
# Flags (each also readable from the environment for `curl | bash`):
#   --channel stable|nightly   NERU_CHANNEL   release channel; default: the installed one, else stable
#   --version vX.Y.Z           NERU_VERSION   pin a stable release (implies --channel stable)
#   --from DIR                 NERU_FROM      install a local `just dist` tree instead of downloading
#   --bin-dir DIR              NERU_BIN_DIR   where `neru` goes; default /usr/local/bin (macOS), ~/.local/bin (Linux)
#   --app-dir DIR              NERU_APP_DIR   macOS only, where Neru.app goes; default /Applications
#   --no-service                              never register or start the login service
#   --no-completions                          skip shell completions
#   --no-man                                  skip man pages
#   --force                                   reinstall even when the same version is already installed
#   --uninstall                               remove everything a previous run installed (config is kept)
#   --purge                                   with --uninstall, also delete config, data and logs
#   -y, --yes                  NERU_YES=1     accept every prompt (required when no terminal is attached)
#   -h, --help
#
# Output honours NO_COLOR and drops colour when not writing to a terminal.
set -euo pipefail

repo="y3owk1n/neru"
gh_base="https://github.com/$repo"
api_base="https://api.github.com/repos/$repo"

channel="${NERU_CHANNEL:-}"
version="${NERU_VERSION:-}"
from="${NERU_FROM:-}"
bin_dir="${NERU_BIN_DIR:-}"
app_dir="${NERU_APP_DIR:-/Applications}"
assume_yes="${NERU_YES:-0}"
want_service=1
want_completions=1
want_man=1
force=0
uninstall=0
purge=0

# ------------------------------------------------------------ presentation --

# Colour only when stderr is a terminal (all status goes there, so stdout
# stays clean for anyone capturing it) and nobody asked for plain output.
if [ -t 2 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != dumb ]; then
    c_bold=$'\033[1m' c_dim=$'\033[2m' c_red=$'\033[31m' c_green=$'\033[32m'
    c_yellow=$'\033[33m' c_cyan=$'\033[36m' c_reset=$'\033[0m'
else
    c_bold='' c_dim='' c_red='' c_green='' c_yellow='' c_cyan='' c_reset=''
fi

say()   { printf '%s\n' "$*" >&2; }
step()  { printf '\n%s%s%s\n' "$c_bold" "$*" "$c_reset" >&2; }
ok()    { printf '  %s✓%s %s\n' "$c_green" "$c_reset" "$*" >&2; }
info()  { printf '  %s·%s %s\n' "$c_dim" "$c_reset" "$*" >&2; }
note()  { printf '  %s%s%s\n' "$c_dim" "$*" "$c_reset" >&2; }
warn()  { printf '  %s!%s %s\n' "$c_yellow" "$c_reset" "$*" >&2; }
# kv "Label" "value" -> an aligned two-column line for headers and summaries.
kv()    { printf '  %s%-10s%s %s\n' "$c_dim" "$1" "$c_reset" "$2" >&2; }

die() {
    printf '\n  %s✗ %s%s\n' "$c_red" "$*" "$c_reset" >&2
    restore_service
    exit 1
}

usage() {
    cat <<'USAGE'
Neru installer for macOS and Linux. Installs, updates or removes a release build.

  curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash -s -- [flags]

Flags (each also readable from the environment for `curl | bash`):
  --channel stable|nightly   NERU_CHANNEL   release channel; default: the installed one, else stable
  --version vX.Y.Z           NERU_VERSION   pin a stable release (implies --channel stable)
  --from DIR                 NERU_FROM      install a local `just dist` tree instead of downloading
  --bin-dir DIR              NERU_BIN_DIR   where `neru` goes; default /usr/local/bin (macOS), ~/.local/bin (Linux)
  --app-dir DIR              NERU_APP_DIR   macOS only, where Neru.app goes; default /Applications
  --no-service                              never register or start the login service
  --no-completions                          skip shell completions
  --no-man                                  skip man pages
  --force                                   reinstall even when the same version is already installed
  --uninstall                               remove everything a previous run installed (config is kept)
  --purge                                   with --uninstall, also delete config, data and logs
  -y, --yes                  NERU_YES=1     accept every prompt (required when no terminal is attached)
  -h, --help

Output honours NO_COLOR and drops colour when not writing to a terminal.
USAGE
}

# ask "question" -> 0 when the user says yes. Prompts read from the terminal,
# not stdin, because under `curl | bash` stdin is the script itself. With no
# terminal and no -y there is nobody to answer, so refuse rather than guess.
ask() {
    if [ "$assume_yes" = 1 ]; then
        printf '  %s?%s %s %s[y/N]%s y\n' "$c_cyan" "$c_reset" "$1" "$c_dim" "$c_reset" >&2
        return 0
    fi
    # Open the terminal first, so a missing one fails here rather than
    # swallowing the prompt text along with the error.
    if ! { exec 3</dev/tty; } 2>/dev/null; then
        die "No terminal to answer prompts. Rerun with -y (or NERU_YES=1) to accept them all."
    fi
    local reply
    printf '  %s?%s %s %s[y/N]%s ' "$c_cyan" "$c_reset" "$1" "$c_dim" "$c_reset" >&2
    read -r reply <&3 || reply=""
    exec 3<&-
    case "$reply" in
        [Yy] | [Yy][Ee][Ss]) return 0 ;;
        *) return 1 ;;
    esac
}

need() {
    command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not installed."
}

# neru_version BIN -> the version string, or nothing when BIN cannot run.
neru_version() {
    { "$1" --version 2>/dev/null || true; } | sed -n 's/^Neru version //p' | head -n1
}

# probe_binary BIN -> dies before anything is installed when BIN cannot
# start here. On Linux the release links libtesseract, libpipewire and the
# X11/Wayland libraries dynamically, so a missing one fails every command.
probe_binary() {
    local out missing
    if out="$("$1" --version 2>&1)"; then return 0; fi
    say ""
    warn "The Neru binary cannot start on this system:"
    printf '%s\n' "$out" | sed 's/^/      /' >&2
    if [ "$os" = linux ] && command -v ldd >/dev/null 2>&1; then
        missing="$(ldd "$1" 2>/dev/null | awk '/not found/ { print $1 }')"
        if [ -n "$missing" ]; then
            note "Missing shared libraries:"
            printf '%s\n' "$missing" | sed 's/^/      /' >&2
            # Ask the package manager which package ships each library, so
            # the fix is one command away instead of a docs lookup.
            if command -v dnf >/dev/null 2>&1; then
                note "Find the packages with:"
                printf '%s\n' "$missing" | sed "s|.*|      dnf provides '*/&'|" >&2
            elif command -v apt-get >/dev/null 2>&1; then
                note "Find the packages with (apt-file: sudo apt-get install apt-file && sudo apt-file update):"
                printf '%s\n' "$missing" | sed 's|.*|      apt-file search &|' >&2
            elif command -v pacman >/dev/null 2>&1; then
                note "Find the packages with:"
                printf '%s\n' "$missing" | sed 's|.*|      pacman -F &|' >&2
            fi
        fi
        # Fedora names tesseract's library libtesseract.so.5.5 (CMake soname)
        # where the Ubuntu-built release wants libtesseract.so.5. Installing
        # more packages cannot fix that; a compatibility symlink does.
        local fedora_tess="" f
        for f in /usr/lib64/libtesseract.so.5.[0-9]* /usr/lib/libtesseract.so.5.[0-9]*; do
            [ -e "$f" ] && { fedora_tess="$f"; break; }
        done
        if printf '%s\n' "$missing" | grep -qx 'libtesseract.so.5' && [ -n "$fedora_tess" ]; then
            note "Your distribution ships the library as $(basename "$fedora_tess"). Link it under the"
            note "name the release build expects, then rerun this script:"
            note "      sudo ln -s $(basename "$fedora_tess") $(dirname "$fedora_tess")/libtesseract.so.5"
            note "Details: https://github.com/$repo/blob/main/docs/LINUX_SETUP.md#troubleshooting"
        else
            note "The full list per distribution is under"
            note "https://github.com/$repo/blob/main/docs/LINUX_SETUP.md#build-dependencies"
            note "Install them, then rerun this script."
        fi
    fi
    die "Nothing was installed."
}

# sudo_for DIR -> prints "sudo" when DIR (or its nearest existing parent) is
# not writable by this user, so callers prefix commands with it.
sudo_for() {
    local d="$1"
    while [ ! -e "$d" ]; do d="$(dirname "$d")"; done
    if [ -w "$d" ]; then printf ''; else printf 'sudo'; fi
}

# Runs curl with a progress bar on a terminal and quietly otherwise.
fetch() {
    if [ -t 2 ]; then
        curl -fL --retry 3 --retry-delay 1 --progress-bar "$1" -o "$2"
    else
        curl -fsSL --retry 3 --retry-delay 1 "$1" -o "$2"
    fi
}

tmp=""
cleanup() { [ -z "$tmp" ] || rm -rf "$tmp"; }
# An update unloads a registered login service before swapping the binary.
# If the run dies in between, put the service back on whichever binary is
# at the install path now, so a failed update never costs the user autostart.
service_unloaded=0
restore_service() {
    [ "$service_unloaded" = 1 ] || return 0
    service_unloaded=0
    if [ -x "${neru_bin:-}" ] && "$neru_bin" services install >/dev/null 2>&1; then
        warn "Re-registered the login service that was unloaded for the update."
    else
        warn "The login service was unloaded for the update and could not be restored. Run: neru services install"
    fi
}
on_interrupt() {
    say ""
    warn "Interrupted. Files are only ever swapped into place whole, so nothing is left half-installed."
    restore_service
    exit 130
}
on_error() {
    # A command substitution inherits this trap; let the parent shell report.
    [ "${BASH_SUBSHELL:-0}" -eq 0 ] || exit 1
    say ""
    printf '  %s✗ Unexpected failure at line %s: %s%s\n' "$c_red" "$1" "$BASH_COMMAND" "$c_reset" >&2
    note "Please report this with the lines above at $gh_base/issues."
    restore_service
    exit 1
}
trap cleanup EXIT
trap on_interrupt INT
trap 'on_error $LINENO' ERR
set -E

# ------------------------------------------------------------------- flags --

while [ $# -gt 0 ]; do
    case "$1" in
        --channel) [ $# -ge 2 ] || die "--channel needs a value."; channel="$2"; shift 2 ;;
        --channel=*) channel="${1#*=}"; shift ;;
        --version) [ $# -ge 2 ] || die "--version needs a value."; version="$2"; shift 2 ;;
        --version=*) version="${1#*=}"; shift ;;
        --from) [ $# -ge 2 ] || die "--from needs a value."; from="$2"; shift 2 ;;
        --from=*) from="${1#*=}"; shift ;;
        --bin-dir) [ $# -ge 2 ] || die "--bin-dir needs a value."; bin_dir="$2"; shift 2 ;;
        --bin-dir=*) bin_dir="${1#*=}"; shift ;;
        --app-dir) [ $# -ge 2 ] || die "--app-dir needs a value."; app_dir="$2"; shift 2 ;;
        --app-dir=*) app_dir="${1#*=}"; shift ;;
        --no-service) want_service=0; shift ;;
        --no-completions) want_completions=0; shift ;;
        --no-man) want_man=0; shift ;;
        --force) force=1; shift ;;
        --uninstall) uninstall=1; shift ;;
        --purge) purge=1; shift ;;
        -y | --yes) assume_yes=1; shift ;;
        -h | --help) usage; exit 0 ;;
        *) die "Unknown argument: $1 (see --help)." ;;
    esac
done

[ "$purge" = 1 ] && [ "$uninstall" = 0 ] && die "--purge only makes sense with --uninstall."
case "$channel" in
    "" | stable | nightly) ;;
    *) die "--channel must be stable or nightly, got '$channel'." ;;
esac
if [ -n "$version" ]; then
    case "$version" in
        v[0-9]*) ;;
        [0-9]*) version="v$version" ;;
        *) die "--version must look like v1.52.0, got '$version'." ;;
    esac
    [ "$channel" = nightly ] && die "--version pins a stable release; it cannot be combined with --channel nightly."
    channel=stable
fi
if [ -n "$from" ]; then
    [ -z "$channel" ] && [ -z "$version" ] || die "--from installs a local build; it cannot be combined with --channel or --version."
    [ -x "$from/bin/neru" ] || die "$from/bin/neru not found. Build the tree with 'just dist' first."
    channel=source
fi

# ---------------------------------------------------------------- platform --

os="$(uname -s)"
case "$os" in
    Darwin) os=darwin; os_label=macOS ;;
    Linux) os=linux; os_label=Linux ;;
    *) die "Unsupported OS '$os'. On Windows run: irm https://raw.githubusercontent.com/$repo/main/scripts/install.ps1 | iex" ;;
esac
arch="$(uname -m)"
case "$arch" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) die "Unsupported architecture '$arch' (releases ship amd64 and arm64)." ;;
esac
asset="neru-$os-$arch.zip"

if [ -z "$bin_dir" ]; then
    if [ "$os" = darwin ]; then bin_dir=/usr/local/bin; else bin_dir="$HOME/.local/bin"; fi
fi
bin_dir="${bin_dir%/}"
app_dir="${app_dir%/}"
man_dir="$(dirname "$bin_dir")/share/man/man1"
state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/neru"
manifest="$state_dir/install-manifest"

if [ "$os" = darwin ]; then
    neru_bin="$app_dir/Neru.app/Contents/MacOS/neru"
else
    neru_bin="$bin_dir/neru"
fi

# ------------------------------------------------------------------ header --

say ""
if [ "$uninstall" = 1 ]; then
    say "  ${c_bold}Neru uninstaller${c_reset}"
else
    say "  ${c_bold}Neru installer${c_reset}"
fi
kv "System" "$os_label $arch"
if [ -n "$from" ]; then
    kv "Source" "$from (local build)"
elif [ "$uninstall" = 0 ]; then
    kv "Channel" "${channel:-auto (keeps the installed channel, else stable)}${version:+ ($version)}"
fi

# ---------------------------------------------------- what is installed? --

step "Checking your system"

if [ "$os" = darwin ] && command -v brew >/dev/null 2>&1 &&
    brew list --cask 2>/dev/null | grep -qxE 'neru|neru-nightly'; then
    warn "Neru is already installed with Homebrew, so this script will not touch it."
    note "Update:  brew upgrade --cask y3owk1n/tap/neru"
    note "Remove:  brew uninstall --cask y3owk1n/tap/neru   (then rerun this script)"
    exit 1
fi
if existing_on_path="$(command -v neru 2>/dev/null)"; then
    case "$(readlink -f "$existing_on_path" 2>/dev/null || printf '%s' "$existing_on_path")" in
        /nix/store/*)
            warn "The neru on your PATH comes from Nix ($existing_on_path)."
            note "Update it through your nix-darwin, NixOS or home-manager config instead."
            exit 1
            ;;
    esac
fi

installed_version=""
installed_channel=""
if [ -x "$neru_bin" ]; then
    installed_version="$(neru_version "$neru_bin")"
    # A release tag is exactly vX.Y.Z; a source build carries git describe's
    # -N-gSHA or -dirty suffix behind it.
    if [ -z "$installed_version" ]; then installed_channel=unknown
    elif [[ "$installed_version" == nightly* ]]; then installed_channel=nightly
    elif [[ "$installed_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then installed_channel=stable
    else installed_channel=source
    fi
    ok "Found Neru ${installed_version:-?} ${c_dim}($installed_channel, $neru_bin)${c_reset}"
else
    info "No existing Neru at $neru_bin"
fi

if [ "$uninstall" = 0 ] && [ -z "$from" ]; then
    need curl
    if [ "$os" = darwin ]; then need shasum; else need sha256sum; fi
    if command -v unzip >/dev/null 2>&1; then
        extract() { unzip -q "$1" -d "$2"; }
    elif command -v bsdtar >/dev/null 2>&1; then
        extract() { bsdtar -xf "$1" -C "$2"; }
    elif tar --version 2>/dev/null | grep -q bsdtar; then
        extract() { tar -xf "$1" -C "$2"; }
    else
        die "'unzip' (or bsdtar) is required to unpack the release."
    fi
fi

# Say up front which locations need administrator rights, so the password
# prompt that follows is not a surprise, and ask for it once.
sudo_targets=()
if [ "$uninstall" = 0 ]; then
    [ "$os" = darwin ] && [ -n "$(sudo_for "$app_dir")" ] && sudo_targets+=("$app_dir")
    [ -n "$(sudo_for "$bin_dir")" ] && sudo_targets+=("$bin_dir")
    [ "$want_man" = 1 ] && [ -n "$(sudo_for "$man_dir")" ] && sudo_targets+=("$man_dir")
fi
if [ "${#sudo_targets[@]}" -gt 0 ]; then
    warn "Administrator rights are needed to write to: ${sudo_targets[*]}"
    sudo -v || die "Could not obtain administrator rights. Pass --bin-dir (and --app-dir) to install somewhere you can write."
fi

# --------------------------------------------------------------- uninstall --

# Undoes a previous run: what the manifest lists, else the default locations.
# Config, data and logs stay unless --purge, and each of those is confirmed.
if [ "$uninstall" = 1 ]; then
    man_dirs=("$man_dir")
    comps=(
        "${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion/completions/neru"
        "$HOME/.zsh/completions/_neru"
        "${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions/neru.fish"
    )
    # The manifest is user-writable and some removals run under sudo, so its
    # paths are hints, not authority: each must still look like the thing
    # this script installs (its basename, and for the app the binary inside)
    # before it is removed.
    app_dst=""
    link=""
    if [ -f "$manifest" ]; then
        while IFS='=' read -r k v; do
            case "$k" in
                binary) [ "$(basename "$v")" = neru ] && [ -f "$v" ] && [ ! -L "$v" ] && neru_bin="$v" ;;
                app) [ "$(basename "$v")" = Neru.app ] && [ -f "$v/Contents/MacOS/neru" ] && app_dst="$v" ;;
                link) [ "$(basename "$v")" = neru ] && [ -L "$v" ] && link="$v" ;;
                man_dir) [ "$(basename "$v")" = man1 ] && man_dirs+=("$v") ;;
                completion) case "$(basename "$v")" in neru | _neru | neru.fish) comps+=("$v") ;; esac ;;
            esac
        done <"$manifest"
    fi
    if [ "$os" = darwin ]; then
        link="${link:-$bin_dir/neru}"
        # The PATH symlink points into the app, so it locates a bundle
        # installed with --app-dir even when the manifest is unusable.
        if [ -z "$app_dst" ] && [ -L "$link" ]; then
            case "$(readlink "$link")" in
                */Neru.app/Contents/MacOS/neru) target="$(readlink "$link")"; app_dst="${target%/Contents/MacOS/neru}" ;;
            esac
        fi
        app_dst="${app_dst:-$app_dir/Neru.app}"
        neru_bin="$app_dst/Contents/MacOS/neru"
    fi

    if [ ! -e "$neru_bin" ] && [ ! -e "$app_dst" ] && [ ! -e "$link" ]; then
        say ""
        say "  Nothing to uninstall."
        exit 0
    fi

    step "Removing Neru"
    note "The login service, app or binary, PATH link, man pages and completions go."
    if [ "$purge" = 1 ]; then
        note "Config, data and logs are removed too, each after its own confirmation."
    else
        note "Config, data and logs are kept (add --purge to remove them)."
    fi
    ask "Uninstall Neru?" || { say ""; say "  Nothing changed."; exit 0; }

    if [ -x "$neru_bin" ]; then
        case "$("$neru_bin" services status 2>/dev/null)" in
            "Service loaded"* | "Service installed"*)
                if "$neru_bin" services uninstall >/dev/null 2>&1; then ok "Login service removed"; fi
                ;;
        esac
        "$neru_bin" stop >/dev/null 2>&1 || true
    fi
    if [ "$os" = darwin ]; then
        if [ -L "$link" ]; then
            $(sudo_for "$(dirname "$link")") rm -f "$link" && ok "Removed $link"
        elif [ -e "$link" ]; then
            warn "Left $link alone: it is not a symlink, so this script did not create it."
        fi
        if [ -e "$app_dst" ]; then
            $(sudo_for "$(dirname "$app_dst")") rm -rf "$app_dst" && ok "Removed $app_dst"
        fi
    elif [ -e "$neru_bin" ]; then
        $(sudo_for "$(dirname "$neru_bin")") rm -f "$neru_bin" && ok "Removed $neru_bin"
    fi
    for d in "${man_dirs[@]}"; do
        if ls "$d"/neru*.1 >/dev/null 2>&1; then
            $(sudo_for "$d") rm -f "$d"/neru*.1 && ok "Removed man pages from $d"
        fi
    done
    for f in "${comps[@]}"; do
        if [ -e "$f" ]; then rm -f "$f" && ok "Removed $f"; fi
    done
    rm -f "$manifest"
    rmdir "$state_dir" 2>/dev/null || true

    if [ "$purge" = 1 ]; then
        step "Removing config, data and logs"
        if [ "$os" = darwin ]; then
            data_dirs=("${XDG_CONFIG_HOME:-$HOME/.config}/neru" "$HOME/Library/Application Support/neru" "$HOME/Library/Logs/neru")
        else
            data_dirs=("${XDG_CONFIG_HOME:-$HOME/.config}/neru" "${XDG_DATA_HOME:-$HOME/.local/share}/neru" "$state_dir")
        fi
        for d in "${data_dirs[@]}"; do
            [ -e "$d" ] || continue
            if ask "Delete $d?"; then rm -rf "$d" && ok "Removed $d"; else info "Kept $d"; fi
        done
    fi

    say ""
    say "  ${c_bold}${c_green}Neru is uninstalled.${c_reset}"
    if [ "$purge" = 0 ]; then
        note "Config, data and logs were kept. Rerun with --uninstall --purge to remove them."
    fi
    if [ "$os" = darwin ]; then
        note "The Accessibility and Input Monitoring entries stay in System Settings; remove them by hand."
    fi
    say ""
    exit 0
fi

# ----------------------------------------------------------------- channel --

if [ -z "$channel" ]; then
    case "$installed_channel" in
        stable | nightly) channel="$installed_channel" ;;
        *) channel=stable ;;
    esac
fi

if [ -n "$installed_channel" ] && [ "$installed_channel" != "$channel" ]; then
    say ""
    case "$installed_channel:$channel" in
        *:source)
            warn "This replaces the installed $installed_channel build with your local build from $from."
            note "A later run without --from goes back to a release and asks first."
            ;;
        stable:nightly)
            warn "You have the ${c_bold}stable${c_reset} release installed and asked for ${c_bold}nightly${c_reset}."
            note "Nightly is rebuilt from every push to main. It may carry regressions, half-finished"
            note "features and config keys that later change. Every future run of this script keeps"
            note "you on nightly unless you pass --channel stable."
            ;;
        nightly:stable)
            warn "You have a ${c_bold}nightly${c_reset} build installed and asked for ${c_bold}stable${c_reset}."
            note "Stable is older than your nightly, so options that only exist on main are rejected"
            note "by config validation until they ship in a release."
            ;;
        *)
            warn "The installed Neru is a $installed_channel build (from 'just install', most likely)."
            note "Continuing replaces it with the $channel release."
            ;;
    esac
    ask "Switch to $channel?" || { say ""; say "  Nothing changed."; exit 0; }
fi

# ------------------------------------------------------------------- fetch --

tmp="$(mktemp -d)"
# src is the release layout to install from: bin/neru, share/man/man1 and,
# on macOS, Neru.app. A downloaded zip and a `just dist` tree share it.
src="$tmp/unpacked"
if [ -n "$from" ]; then
    src="$from"
    target_label="local build"
else
    if [ "$channel" = nightly ]; then
        tag=nightly
    elif [ -n "$version" ]; then
        tag="$version"
    else
        tag="$(curl -fsSL --retry 3 "$api_base/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
        [ -n "$tag" ] || die "Could not determine the latest release from $api_base/releases/latest."
    fi

    if [ "$channel" = stable ] && [ "$installed_version" = "$tag" ] && [ "$force" = 0 ]; then
        say ""
        say "  ${c_green}Neru $tag is already installed and up to date.${c_reset}"
        note "Pass --force to reinstall it anyway."
        say ""
        exit 0
    fi
    if [ "$channel" = nightly ]; then target_label="the latest nightly build"; else target_label="Neru $tag"; fi

    step "Fetching $target_label"
    url="$gh_base/releases/download/$tag/$asset"
    info "$url"
    fetch "$url" "$tmp/$asset" || die "Download failed: $url"
    curl -fsSL --retry 3 "$url.sha256" -o "$tmp/$asset.sha256" || die "Download failed: $url.sha256"
    ok "Downloaded $asset"
    (
        cd "$tmp"
        if [ "$os" = darwin ]; then shasum -a 256 -c "$asset.sha256" >/dev/null; else sha256sum -c "$asset.sha256" >/dev/null; fi
    ) || die "Checksum mismatch for $asset. Refusing to install a corrupted download."
    ok "Checksum verified"
    mkdir -p "$src"
    extract "$tmp/$asset" "$src"
    [ -x "$src/bin/neru" ] || die "The release archive did not contain bin/neru."
fi

probe_binary "$src/bin/neru"

# ----------------------------------------------------------------- install --

if [ -n "$installed_channel" ]; then
    step "Updating ${installed_version:-Neru} to $target_label"
else
    step "Installing $target_label"
fi

# Replacing the binary under a live daemon is asking for trouble. A registered
# login service is unloaded (a plain stop would be undone by KeepAlive on
# macOS) and registered again after the copy; a bare daemon is stopped and
# mentioned at the end. `services status` always exits 0, so read its text.
service_was_installed=0
daemon_was_running=0
if [ -x "$neru_bin" ]; then
    case "$("$neru_bin" services status 2>/dev/null)" in
        "Service loaded"* | "Service installed"*)
            service_was_installed=1
            "$neru_bin" services uninstall >/dev/null 2>&1 || true
            service_unloaded=1
            info "Paused the login service for the update"
            ;;
    esac
    if "$neru_bin" status >/dev/null 2>&1; then
        daemon_was_running=1
        "$neru_bin" stop >/dev/null 2>&1 || true
        info "Stopped the running daemon"
    fi
fi

replaced_app=0
if [ "$os" = darwin ]; then
    [ -d "$src/Neru.app" ] || die "The release archive did not contain Neru.app."
    app_dst="$app_dir/Neru.app"
    s="$(sudo_for "$app_dir")"
    $s mkdir -p "$app_dir"
    staging="$app_dir/.Neru.app.new.$$"
    previous="$app_dir/.Neru.app.old.$$"
    $s rm -rf "$staging" "$previous"
    $s cp -R "$src/Neru.app" "$staging"
    $s xattr -dr com.apple.quarantine "$staging" 2>/dev/null || true
    if [ -e "$app_dst" ]; then
        replaced_app=1
        $s mv "$app_dst" "$previous"
    fi
    $s mv "$staging" "$app_dst"
    $s rm -rf "$previous"
    ok "Neru.app ${c_dim}→${c_reset} $app_dir"

    link="$bin_dir/neru"
    s="$(sudo_for "$bin_dir")"
    if [ -e "$link" ] && [ ! -L "$link" ]; then
        warn "$link exists and is a regular file, not a symlink (a hand-installed neru?)."
        ask "Replace it with a symlink to the app binary?" || die "Left $link alone. Put $neru_bin on your PATH yourself."
    fi
    $s mkdir -p "$bin_dir"
    $s ln -sfn "$neru_bin" "$link"
    ok "neru ${c_dim}→${c_reset} $link"
else
    s="$(sudo_for "$bin_dir")"
    $s mkdir -p "$bin_dir"
    $s install -m 0755 "$src/bin/neru" "$bin_dir/neru.new.$$"
    $s mv -f "$bin_dir/neru.new.$$" "$neru_bin"
    ok "neru ${c_dim}→${c_reset} $neru_bin"
fi

path_missing=0
case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *) path_missing=1 ;;
esac

if [ "$want_man" = 1 ]; then
    if ls "$src"/share/man/man1/*.1 >/dev/null 2>&1; then
        s="$(sudo_for "$man_dir")"
        $s mkdir -p "$man_dir"
        $s cp "$src"/share/man/man1/*.1 "$man_dir/"
        ok "Man pages ${c_dim}→${c_reset} $man_dir"
    else
        info "This build carries no man pages; skipped"
    fi
fi

comp_files=()
comp_shells=""
zsh_hint=0
if [ "$want_completions" = 1 ]; then
    if command -v bash >/dev/null 2>&1; then
        d="${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion/completions"
        mkdir -p "$d" && "$neru_bin" completion bash >"$d/neru" && comp_files+=("$d/neru") && comp_shells="$comp_shells bash"
    fi
    if command -v zsh >/dev/null 2>&1; then
        d="$HOME/.zsh/completions"
        mkdir -p "$d" && "$neru_bin" completion zsh >"$d/_neru" && comp_files+=("$d/_neru") && comp_shells="$comp_shells zsh"
        zsh_hint=1
    fi
    if command -v fish >/dev/null 2>&1; then
        d="${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions"
        mkdir -p "$d" && "$neru_bin" completion fish >"$d/neru.fish" && comp_files+=("$d/neru.fish") && comp_shells="$comp_shells fish"
    fi
    if [ "${#comp_files[@]}" -gt 0 ]; then
        ok "Shell completions ${c_dim}for${c_reset}${comp_shells}"
    fi
fi

# ----------------------------------------------------------------- service --

service_state=none
if [ "$want_service" = 1 ]; then
    if [ "$service_was_installed" = 1 ]; then
        service_unloaded=0
        if "$neru_bin" services install >/dev/null 2>&1; then
            service_state=restarted
            ok "Login service resumed on the new version"
        else
            service_state=failed
            warn "Could not resume the login service. Run: neru services install"
        fi
    elif [ "$daemon_was_running" = 0 ]; then
        step "Login service"
        note "Neru runs as a background daemon. Registering it starts it now and at every login."
        if ask "Start Neru now and at every login?"; then
            if "$neru_bin" services install >/dev/null 2>&1; then
                service_state=installed
                ok "Registered and running"
            else
                service_state=failed
                warn "Registration failed. Retry later with: neru services install"
            fi
        else
            info "Skipped. Start it any time with: neru launch"
        fi
    fi
fi

if [ "$os" = linux ] && ! id -nG 2>/dev/null | tr ' ' '\n' | grep -qx input; then
    step "Wayland keyboard access"
    note "On Wayland, Neru reads the keyboard through evdev, which needs the 'input' group."
    if ask "Add $USER to the 'input' group? (sudo; takes effect after you log in again)"; then
        if sudo usermod -aG input "$USER"; then
            ok "Added to 'input'. Log out and back in for it to take effect."
        else
            warn "Could not modify groups. Later: sudo usermod -aG input \$USER"
        fi
    else
        info "Skipped. Needed on Wayland only: sudo usermod -aG input \$USER"
    fi
fi

# ---------------------------------------------------------------- manifest --

new_version="$(neru_version "$neru_bin")"
mkdir -p "$state_dir"
{
    echo "channel=$channel"
    echo "version=$new_version"
    echo "binary=$neru_bin"
    [ "$os" = darwin ] && echo "app=$app_dir/Neru.app" && echo "link=$bin_dir/neru"
    [ "$want_man" = 1 ] && echo "man_dir=$man_dir"
    for f in ${comp_files[@]+"${comp_files[@]}"}; do echo "completion=$f"; done
    echo "installed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >"$manifest"

# ----------------------------------------------------------------- summary --

say ""
say "  ${c_bold}${c_green}Neru ${new_version:-?} is installed.${c_reset} ${c_dim}($channel)${c_reset}"
say ""
say "  ${c_bold}Next steps${c_reset}"
n=0
next() { n=$((n + 1)); printf '  %s%d.%s %s\n' "$c_dim" "$n" "$c_reset" "$*" >&2; }
if [ "$os" = darwin ]; then
    next "Grant Accessibility and Input Monitoring: System Settings › Privacy & Security."
    if [ "$replaced_app" = 1 ]; then
        note "   Replacing the app changes its ad-hoc signature, so macOS may drop the"
        note "   Accessibility grant. Re-approve it if navigation stops working."
    fi
fi
case "$service_state" in
    installed | restarted) ;;
    *)
        if [ "$daemon_was_running" = 1 ]; then
            next "Start the daemon again: ${c_bold}neru launch${c_reset} (it was stopped for the update)."
        else
            next "Start it: ${c_bold}neru launch${c_reset}, or ${c_bold}neru services install${c_reset} to start at login."
        fi
        ;;
esac
if [ "$path_missing" = 1 ]; then
    next "Add $bin_dir to your PATH so 'neru' resolves by name."
fi
if [ "$zsh_hint" = 1 ]; then
    next "zsh: add ${c_bold}fpath=(~/.zsh/completions \$fpath)${c_reset} to ~/.zshrc before compinit for completions."
fi
if [ "$os" = linux ]; then
    next "On Wayland, bind compositor hotkeys to 'neru hints' and friends. See docs/LINUX_SETUP.md."
fi
next "Configure: ${c_bold}neru config init${c_reset} writes a starter ~/.config/neru/config.toml."
say ""
note "Manage the service with 'neru services status|stop|restart'. Rerun this script to update,"
note "add --channel stable|nightly to switch, or --uninstall to remove."
say ""
