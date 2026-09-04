#!/usr/bin/env bash
#
# neru Windows installer (minimal). Invoked by `just install` under a
# bash such as Git Bash; build the exe first with `just build-windows`.
set -euo pipefail

# Run from the repo root so build/, bin/, and `just` resolve.
cd "$(dirname "$0")/.."

# Any of -y / --yes on the command line auto-accepts every prompt.
assume_yes=0
for arg in "$@"; do
    case "$arg" in
        -y | --yes) assume_yes=1 ;;
        *) echo "unknown argument: $arg (use -y to auto-accept prompts)" >&2; exit 2 ;;
    esac
done

# ask "prompt" -> prints the reply on stdout. Under -y it echoes the prompt with
# a "y" and answers yes without reading, so the whole run is non-interactive.
ask() {
    if [ "$assume_yes" -eq 1 ]; then
        printf '%sy\n' "$1" >&2
        printf 'y'
        return 0
    fi
    local reply
    read -r -p "$1" reply || reply=""
    printf '%s' "$reply"
}

# win_path prints the Windows form (C:\Users\...) of a POSIX path. Everything
# Windows-facing needs it: PATH entries, shortcut targets, registry values, and
# anything we print for the user to copy. Git Bash's /c/Users/... form is not a
# path Windows itself understands.
win_path() {
    cygpath -w "$1" 2>/dev/null || printf '%s' "$1"
}

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

# run_ps runs a PowerShell script supplied on stdin, passing any extra
# arguments through to it. The script is staged in a temp file and run with
# -File rather than passed inline with -Command so that bash, MSYS path
# mangling and PowerShell quoting never have to agree with each other.
run_ps() {
    local script="$tmp_dir/step.ps1"
    cat >"$script"
    MSYS2_ARG_CONV_EXCL='*' powershell -NoProfile -NonInteractive \
        -ExecutionPolicy Bypass -File "$(win_path "$script")" "$@"
}

# Minimal Windows install: a per-user binary, a Start Menu shortcut so Neru is
# searchable from the taskbar like any other app, and optional autostart via
# `neru services install`, which registers a Task Scheduler logon task the way
# the macOS installer loads a launchd agent. Runs under a bash (e.g. Git Bash);
# `just` needs cygpath on PATH to translate the shebang.
# Windows support is alpha (grid, recursive grid, scroll, hotkeys, mouse
# injection, UIA); see docs/CROSS_PLATFORM.md.
arch="$(uname -m)"
case "$arch" in
    aarch64 | arm64) arch=arm64 ;;
    *) arch=amd64 ;;
esac
exe_src="bin/neru-windows-$arch.exe"
if [ ! -f "$exe_src" ]; then
    echo "Neru has not been built for Windows yet ($exe_src is missing)."
    build_reply="$(ask "Build it now with 'just build-windows $arch'? [y/N] ")"
    case "$build_reply" in
        [Yy] | [Yy][Ee][Ss]) just build-windows "$arch" ;;
        *) echo "Aborted. Run 'just build-windows' first, then 'just install'." >&2; exit 1 ;;
    esac
fi

# Step 1: the binary, to a per-user location. One neru.exe serves as both the
# CLI and the daemon: it is built for the console subsystem so subcommands can
# print to the terminal, and it frees the console when the shell started it.
echo "Step 1/4: Binary"
dst_dir="$(cygpath -u "${LOCALAPPDATA:-$HOME/AppData/Local}")/Programs/neru"
mkdir -p "$dst_dir"
cp "$exe_src" "$dst_dir/neru.exe"
dst_exe="$dst_dir/neru.exe"
win_dst_dir="$(win_path "$dst_dir")"
win_dst_exe="$(win_path "$dst_exe")"
echo "✓ Installed $win_dst_exe"

# Step 2: PATH. Offered rather than applied silently, matching the macOS and
# Linux installers. Windows has no shell rc file to edit by hand, so the offer
# writes the per-user Environment key directly instead of using setx, which
# truncates the value at 1024 characters.
echo "Step 2/4: PATH"
on_path=""
if resolved="$(command -v neru.exe 2>/dev/null)"; then
    on_path="$(cygpath -u "$resolved" 2>/dev/null || printf '%s' "$resolved")"
fi
if [ "$on_path" = "$dst_exe" ]; then
    echo "✓ neru is already on PATH"
else
    path_reply="$(ask "Add $win_dst_dir to your user PATH? [y/N] ")"
    case "$path_reply" in
        [Yy] | [Yy][Ee][Ss])
            if path_result="$(run_ps "$win_dst_dir" <<'PS'
param([Parameter(Mandatory = $true)][string]$Dir)
$ErrorActionPreference = 'Stop'

$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
try {
    # Read the raw value: an existing REG_EXPAND_SZ entry such as
    # %USERPROFILE%\bin must be written back unexpanded, and its kind
    # preserved, or every other tool's PATH entry silently freezes.
    $kind = [Microsoft.Win32.RegistryValueKind]::ExpandString
    $current = ''
    if ($key.GetValueNames() -contains 'Path') {
        $current = [string]$key.GetValue('Path', '', 'DoNotExpandEnvironmentNames')
        $kind = $key.GetValueKind('Path')
    }

    $entries = @($current -split ';' | Where-Object { $_ -ne '' })
    if ($entries -contains $Dir) {
        Write-Output 'already'
        exit 0
    }

    $key.SetValue('Path', (($entries + $Dir) -join ';'), $kind)
} finally {
    $key.Close()
}

# Tell already-running processes (Explorer above all) to reread the
# environment, so a shell opened afterwards sees the new entry without a
# sign-out. Without this the change only lands on the next login.
Add-Type -Namespace Neru -Name Env -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg,
    UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout,
    out UIntPtr lpdwResult);
'@
$result = [UIntPtr]::Zero
[void][Neru.Env]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero,
    'Environment', 0x2, 5000, [ref]$result)

Write-Output 'added'
PS
            )"; then
                if [ "${path_result%$'\r'}" = "already" ]; then
                    echo "✓ Already on your user PATH (open a new terminal to pick it up)"
                else
                    echo "✓ Added to your user PATH (open a new terminal to pick it up)"
                fi
            else
                echo "Could not update PATH; add this directory yourself:" >&2
                echo "      $win_dst_dir" >&2
            fi
            ;;
        *)
            echo "Skipped. Add this directory to your PATH to run 'neru' from anywhere:"
            echo "      $win_dst_dir"
            ;;
    esac
fi

# Step 3: Start Menu shortcut. This, not PATH, is what makes an app searchable
# from the taskbar — Windows Search indexes the per-user Start Menu folder.
echo "Step 3/4: Start Menu"
menu_reply="$(ask "Add a Start Menu shortcut (searchable from the taskbar)? [y/N] ")"
case "$menu_reply" in
    [Yy] | [Yy][Ee][Ss])
        if link="$(run_ps "$win_dst_exe" <<'PS'
param([Parameter(Mandatory = $true)][string]$Exe)
$ErrorActionPreference = 'Stop'

$dir = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$link = Join-Path $dir 'Neru.lnk'

$shortcut = (New-Object -ComObject WScript.Shell).CreateShortcut($link)
$shortcut.TargetPath = $Exe
$shortcut.Arguments = 'launch'
$shortcut.WorkingDirectory = Split-Path -Parent $Exe
$shortcut.IconLocation = $Exe
$shortcut.Description = 'Neru - keyboard-driven navigation'
# Minimized: neru.exe is a console-subsystem binary, so Windows allocates a
# console before main runs. Neru frees it immediately, and starting minimized
# keeps even that moment off the screen.
$shortcut.WindowStyle = 7
$shortcut.Save()

Write-Output $link
PS
        )"; then
            echo "✓ Neru is now searchable from the taskbar"
            echo "  Shortcut: ${link%$'\r'}"
        else
            echo "Could not create the Start Menu shortcut." >&2
        fi
        ;;
    *)
        echo "Skipped Start Menu shortcut."
        ;;
esac

# Step 4: autostart via `neru services install`, run from the binary just
# installed so the Task Scheduler task points at it whether or not PATH was
# updated. No admin: the task runs as the current user with a logon trigger.
echo "Step 4/4: Autostart"
run_reply="$(ask "Start Neru at login (a Task Scheduler task via 'neru services install')? [y/N] ")"
case "$run_reply" in
    [Yy] | [Yy][Ee][Ss])
        if "$dst_exe" services install; then
            echo "✓ Neru will start at login"
            echo "  Inspect with: neru services status"
            echo "  Remove later with: neru services uninstall"
        else
            echo "Could not register the login task; run 'neru services install' yourself." >&2
        fi
        ;;
    *)
        echo "Skipped autostart. Start Neru with: neru launch"
        ;;
esac
echo "Windows support is alpha: worth trying, not yet worth switching to."
echo "Hint coverage is incomplete and per-app config does not re-apply."
echo "See docs/CROSS_PLATFORM.md for what works today."
