#!/usr/bin/env bash
#
# neru Windows uninstaller. Invoked by `just uninstall` under a bash such as
# Git Bash. Undoes scripts/install-windows.sh step for step, in reverse.
set -euo pipefail

# Run from the repo root so `just` resolves.
cd "$(dirname "$0")/.."

# -y / --yes auto-accepts every prompt. --purge additionally offers to delete
# your config and logs; without it they are never touched, so -y on its own can
# never destroy a hand-tuned config.toml.
assume_yes=0
purge=0
for arg in "$@"; do
    case "$arg" in
        -y | --yes) assume_yes=1 ;;
        --purge) purge=1 ;;
        *) echo "unknown argument: $arg (use -y to auto-accept prompts, --purge to also remove config and logs)" >&2; exit 2 ;;
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

# win_path prints the Windows form (C:\Users\...) of a POSIX path, which is what
# every Windows-facing consumer needs: PATH entries, shortcuts, the registry.
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

# purge_list accumulates "label|dir" lines for the directories that actually
# exist. Collecting them before deleting anything is what lets the prompt show
# the fully resolved paths: XDG_CONFIG_HOME can point the config directory
# somewhere other than %APPDATA%, and you should see where before saying yes.
purge_list=""
note_purge_target() {
    [ -d "$2" ] && purge_list="$purge_list$1|$2"$'\n'
    return 0
}

dst_dir="$(cygpath -u "${LOCALAPPDATA:-$HOME/AppData/Local}")/Programs/neru"
dst_exe="$dst_dir/neru.exe"
win_dst_dir="$(win_path "$dst_dir")"
run_key="HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run"

echo "This removes Neru from $win_dst_dir and undoes the PATH, Start Menu"
echo "and autostart entries that 'just install' created."
if [ "$purge" -eq 0 ]; then
    echo "Your config and logs are kept; pass --purge to remove those too."
fi

# Step 0: stop a running Neru. Windows locks the image of a running process, so
# the binary in step 4 cannot be deleted while the daemon is up. Matched by full
# path, not image name: a dev build or a second install is also called neru.exe,
# and this must only stop the one it is about to delete.
echo "Step 0/5: Running instance"
if stop_result="$(run_ps "$win_dst_exe" <<'PS'
param([Parameter(Mandatory = $true)][string]$Exe)
$ErrorActionPreference = 'Stop'

$running = @(Get-Process -Name neru -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $Exe })
if ($running.Count -eq 0) {
    Write-Output 'absent'
    exit 0
}

$running | Stop-Process -Force
Write-Output 'stopped'
PS
)"; then
    if [ "${stop_result%$'\r'}" = "stopped" ]; then
        echo "✓ Stopped the Neru running from $win_dst_exe"
    else
        echo "  No Neru running from $win_dst_exe"
    fi
else
    echo "Could not stop a running Neru; close it before retrying." >&2
fi

# Step 1: autostart. Removed first so a half-finished uninstall never leaves a
# login task pointing at a binary that is already gone. The task is Neru's own
# to remove, through the binary about to be deleted; an install made before
# `neru services` reached Windows wrote a Run key instead, so that is checked
# for as well.
echo "Step 1/5: Autostart"
if [ -x "$dst_exe" ] && "$dst_exe" services status 2>/dev/null | grep -q '^Service installed'; then
    if "$dst_exe" services uninstall >/dev/null 2>&1; then
        echo "✓ Removed the Task Scheduler login task"
    else
        # Stopping here keeps the binary the task points at, and with it the
        # command that can still remove the task.
        echo "Could not remove the login task, so nothing else was removed." >&2
        echo "Run '$(win_path "$dst_exe")' services uninstall, or delete the task 'Neru' in Task Scheduler, then run 'just uninstall' again." >&2
        exit 1
    fi
else
    echo "  No login task found"
fi
if MSYS2_ARG_CONV_EXCL='*' reg query "$run_key" /v Neru >/dev/null 2>&1; then
    if MSYS2_ARG_CONV_EXCL='*' reg delete "$run_key" /v Neru /f >/dev/null 2>&1; then
        echo "✓ Removed the Run key entry an older installer wrote"
    else
        echo "Could not remove the Run key; delete 'Neru' from $run_key yourself." >&2
    fi
fi

# Step 2: Start Menu shortcut.
echo "Step 2/5: Start Menu"
if menu_result="$(run_ps <<'PS'
$ErrorActionPreference = 'Stop'

$link = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Neru.lnk'
if (Test-Path -LiteralPath $link) {
    Remove-Item -LiteralPath $link -Force
    Write-Output 'removed'
} else {
    Write-Output 'absent'
}
PS
)"; then
    if [ "${menu_result%$'\r'}" = "removed" ]; then
        echo "✓ Removed the Start Menu shortcut"
    else
        echo "  No Start Menu shortcut found"
    fi
else
    echo "Could not remove the Start Menu shortcut." >&2
fi

# Step 3: PATH. Drops only our exact entry, reading and writing the value the
# same way the installer does so other tools' %VAR% entries survive intact.
echo "Step 3/5: PATH"
if path_result="$(run_ps "$win_dst_dir" <<'PS'
param([Parameter(Mandatory = $true)][string]$Dir)
$ErrorActionPreference = 'Stop'

$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
try {
    if (-not ($key.GetValueNames() -contains 'Path')) {
        Write-Output 'absent'
        exit 0
    }

    # Read the raw value: an existing REG_EXPAND_SZ entry such as
    # %USERPROFILE%\bin must be written back unexpanded, and its kind
    # preserved, or every other tool's PATH entry silently freezes.
    $current = [string]$key.GetValue('Path', '', 'DoNotExpandEnvironmentNames')
    $kind = $key.GetValueKind('Path')

    # -ne on strings is case-insensitive, which matches how Windows compares
    # paths, so a differently-cased entry is still recognised as ours.
    $entries = @($current -split ';' | Where-Object { $_ -ne '' })
    $kept = @($entries | Where-Object { $_ -ne $Dir })
    if ($kept.Count -eq $entries.Count) {
        Write-Output 'absent'
        exit 0
    }

    $key.SetValue('Path', ($kept -join ';'), $kind)
} finally {
    $key.Close()
}

# Tell already-running processes (Explorer above all) to reread the
# environment, so a shell opened afterwards no longer has the entry.
Add-Type -Namespace Neru -Name Env -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg,
    UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout,
    out UIntPtr lpdwResult);
'@
$result = [UIntPtr]::Zero
[void][Neru.Env]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero,
    'Environment', 0x2, 5000, [ref]$result)

Write-Output 'removed'
PS
)"; then
    if [ "${path_result%$'\r'}" = "removed" ]; then
        echo "✓ Removed $win_dst_dir from your user PATH"
    else
        echo "  $win_dst_dir was not on your user PATH"
    fi
else
    echo "Could not update PATH; remove this directory yourself:" >&2
    echo "      $win_dst_dir" >&2
fi

# Step 4: the binary. Only neru.exe is deleted, and the directory only when
# nothing else is left in it, so anything you put there yourself survives.
echo "Step 4/5: Binary"
if [ -e "$dst_exe" ]; then
    rm -f "$dst_exe"
    echo "✓ Removed $(win_path "$dst_exe")"
    if rmdir "$dst_dir" 2>/dev/null; then
        echo "✓ Removed $win_dst_dir"
    else
        echo "  Kept $win_dst_dir (not empty)"
    fi
else
    echo "  No binary found at $(win_path "$dst_exe")"
fi

# Step 5: config and logs. Opt-in via --purge, and still confirmed, because a
# hand-tuned config.toml is the one thing here that cannot be rebuilt.
echo "Step 5/5: Config and logs"
if [ "$purge" -eq 0 ]; then
    echo "  Kept. Pass --purge to remove your config and logs too."
else
    note_purge_target "config" "$(cygpath -u "${APPDATA:-$HOME/AppData/Roaming}")/neru"
    if [ -n "${XDG_CONFIG_HOME:-}" ]; then
        note_purge_target "XDG config" "$(cygpath -u "$XDG_CONFIG_HOME")/neru"
    fi
    # Neru also reads ~/.config/neru on Windows, for configs carried over from a
    # Unix dotfiles repo.
    note_purge_target "dotfiles config" "$HOME/.config/neru"
    # %LOCALAPPDATA%\neru holds data and logs; the binary lives in the separate
    # %LOCALAPPDATA%\Programs\neru handled above.
    note_purge_target "data and logs" \
        "$(cygpath -u "${LOCALAPPDATA:-$HOME/AppData/Local}")/neru"

    if [ -z "$purge_list" ]; then
        echo "  Nothing to remove."
    else
        echo "  These directories will be permanently deleted:"
        while IFS='|' read -r label dir; do
            [ -n "$dir" ] || continue
            echo "      $(win_path "$dir")  ($label)"
        done <<< "$purge_list"

        purge_reply="$(ask "Delete them? [y/N] ")"
        case "$purge_reply" in
            [Yy] | [Yy][Ee][Ss])
                while IFS='|' read -r label dir; do
                    [ -n "$dir" ] || continue
                    rm -rf "$dir"
                    echo "✓ Removed $label: $(win_path "$dir")"
                done <<< "$purge_list"
                ;;
            *)
                echo "  Kept your config and logs."
                ;;
        esac
    fi
fi

echo "Neru has been uninstalled."
