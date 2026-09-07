<#
.SYNOPSIS
Neru installer for Windows. Downloads a release build, verifies it, and
installs the executable, PATH entry, Start Menu shortcut, PowerShell
completion and login task. Running it again updates in place, keeping the
channel already installed unless told otherwise.

.DESCRIPTION
    irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1 | iex

With flags, wrap it in a script block (or set the NERU_* variables first):

    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1))) -Channel nightly -Yes

Output honours NO_COLOR.

.PARAMETER Channel
stable or nightly (NERU_CHANNEL). Default: the installed channel, else stable.
.PARAMETER Version
Pin a stable release such as v1.52.0 (NERU_VERSION). Implies -Channel stable.
.PARAMETER From
Install a local `just dist` tree (bin\neru.exe, share\man) instead of downloading (NERU_FROM).
.PARAMETER NoService
Never register or start the login task.
.PARAMETER NoCompletions
Do not add PowerShell completion to your profile.
.PARAMETER Force
Reinstall even when the same version is already installed.
.PARAMETER Uninstall
Remove everything a previous run installed. Config, data and logs are kept.
.PARAMETER Purge
With -Uninstall, also delete config, data and logs (each confirmed).
.PARAMETER Yes
Accept every prompt (NERU_YES=1).
#>
[CmdletBinding()]
param(
    [string]$Channel = $env:NERU_CHANNEL,
    [string]$Version = $env:NERU_VERSION,
    [string]$From = $env:NERU_FROM,
    [switch]$NoService,
    [switch]$NoCompletions,
    [switch]$Force,
    [switch]$Uninstall,
    [switch]$Purge,
    [Alias('y')]
    [switch]$Yes
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$repo = 'y3owk1n/neru'
$ghBase = "https://github.com/$repo"
$apiBase = "https://api.github.com/repos/$repo"
if ($env:NERU_YES -eq '1') { $Yes = $true }

# Under `irm ... | iex` this script runs inside the user's session, where
# `exit` would close their window. Early exits therefore throw a sentinel
# that the driver at the bottom turns into a return, or into a real exit
# code when the script was run as a file.
function Stop-Installer([int]$Code) { throw "NERU_EXIT:$Code" }

# ------------------------------------------------------------ presentation --

$script:useColor = -not $env:NO_COLOR
function Write-Piece([string]$Text, [string]$Color = '', [switch]$NoNewline) {
    if ($Color -and $script:useColor) {
        Write-Host $Text -ForegroundColor $Color -NoNewline:$NoNewline
    } else {
        Write-Host $Text -NoNewline:$NoNewline
    }
}
function Write-Step([string]$Text) { Write-Host ''; Write-Piece $Text 'White' }
function Write-Ok([string]$Text) { Write-Piece '  ' -NoNewline; Write-Piece ([char]0x2713) 'Green' -NoNewline; Write-Piece " $Text" }
function Write-Info([string]$Text) { Write-Piece "  $([char]0x00B7) $Text" 'DarkGray' }
function Write-Note([string]$Text) { Write-Piece "  $Text" 'DarkGray' }
function Write-Warn([string]$Text) { Write-Piece '  ' -NoNewline; Write-Piece '!' 'Yellow' -NoNewline; Write-Piece " $Text" }
function Write-KV([string]$Key, [string]$Value) { Write-Piece ('  {0,-10} ' -f $Key) 'DarkGray' -NoNewline; Write-Piece $Value }

# An update removes a registered login task before swapping the binary. If
# the run dies anywhere after that, put the task back on whichever neru.exe
# is at the install path now, so a failed update never costs autostart.
$script:serviceUnloaded = $false
function Restore-LoginTask {
    if (-not $script:serviceUnloaded) { return }
    $script:serviceUnloaded = $false
    $restored = $false
    if (Test-Path $exe) {
        Invoke-Neru services install | Out-Null
        $restored = ($script:neruExit -eq 0)
    }
    if ($restored) {
        Write-Warn 'Re-registered the login task that was unloaded for the update.'
    } else {
        Write-Warn 'The login task was unloaded for the update and could not be restored. Run: neru services install'
    }
}
function Fail([string]$Message) {
    Write-Host ''
    Write-Piece "  $([char]0x2717) $Message" 'Red'
    Restore-LoginTask
    Stop-Installer 1
}

function Invoke-NeruInstaller {

# Ask returns $true when the user says yes. -Yes answers for them.
function Ask([string]$Prompt) {
    Write-Piece '  ' -NoNewline; Write-Piece '?' 'Cyan' -NoNewline; Write-Piece " $Prompt " -NoNewline; Write-Piece '[y/N]' 'DarkGray' -NoNewline
    if ($Yes) {
        Write-Piece ' y'
        return $true
    }
    if (-not [Environment]::UserInteractive -or [Console]::IsInputRedirected) {
        Fail "No console to answer prompts. Rerun with -Yes (or `$env:NERU_YES=1) to accept them all."
    }
    Write-Piece ' ' -NoNewline
    $reply = [Console]::ReadLine()
    return $reply -match '^\s*(y|yes)\s*$'
}

# ------------------------------------------------------------------- flags --

# Validated by hand rather than with ValidateSet: PowerShell re-checks that
# attribute on every later assignment, and the local-build path sets 'source'.
if ($Channel -notin '', 'stable', 'nightly') { Fail "-Channel must be stable or nightly, got '$Channel'." }
if ($Purge -and -not $Uninstall) { Fail '-Purge only makes sense with -Uninstall.' }
if ($Version) {
    if ($Version -match '^\d') { $Version = "v$Version" }
    if ($Version -notmatch '^v\d') { Fail "-Version must look like v1.52.0, got '$Version'." }
    if ($Channel -eq 'nightly') { Fail '-Version pins a stable release; it cannot be combined with -Channel nightly.' }
    $Channel = 'stable'
}
if ($From) {
    if ($Channel -or $Version) { Fail '-From installs a local build; it cannot be combined with -Channel or -Version.' }
    if (-not (Test-Path (Join-Path $From 'bin\neru.exe'))) { Fail "$From\bin\neru.exe not found. Build the tree with 'just dist' first." }
    $From = (Resolve-Path $From).Path
    $Channel = 'source'
}

# ---------------------------------------------------------------- platform --

$arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    'Arm64' { 'arm64' }
    'X64' { 'amd64' }
    default { Fail "Unsupported architecture '$_' (releases ship amd64 and arm64)." }
}
$asset = "neru-windows-$arch.zip"

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\neru'
$script:exe = Join-Path $installDir 'neru.exe'
$manifest = Join-Path $installDir 'install-manifest'
$shortcutPath = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Neru.lnk'

# ------------------------------------------------------------------ header --

Write-Host ''
Write-Piece ($(if ($Uninstall) { '  Neru uninstaller' } else { '  Neru installer' })) 'White'
Write-KV 'System' "Windows $arch"
if ($From) {
    Write-KV 'Source' "$From (local build)"
} elseif (-not $Uninstall) {
    $channelLabel = if ($Channel) { $Channel } else { 'auto (keeps the installed channel, else stable)' }
    if ($Version) { $channelLabel += " ($Version)" }
    Write-KV 'Channel' $channelLabel
}

# ---------------------------------------------------- what is installed? --

Write-Step 'Checking your system'

$onPath = Get-Command neru.exe -ErrorAction SilentlyContinue
if ($onPath -and $onPath.Source -ne $exe) {
    Write-Warn "Another neru.exe is on PATH at $($onPath.Source). This installer manages $exe only."
}

# Invoke-Neru runs the installed binary and returns its stdout as one string,
# leaving the exit code in $script:neruExit. Under Windows PowerShell 5.1 the
# strict error preference turns any stderr line from a native command into a
# terminating error, and neru writes "not running" and the like to stderr, so
# the preference is relaxed for the call and stderr is dropped.
$script:neruExit = 0
function Invoke-Neru {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = & $exe @Arguments 2>$null | Out-String
        $script:neruExit = $LASTEXITCODE
        return $output
    } catch {
        $script:neruExit = 1
        return ''
    } finally {
        $ErrorActionPreference = $previous
    }
}

function Get-InstalledVersion {
    if (-not (Test-Path $exe)) { return '' }
    # Invoke-Neru hands back one multi-line string, so split before matching:
    # ^ and $ would otherwise anchor to the whole output and never match.
    foreach ($line in ((Invoke-Neru --version) -split "`r?`n")) {
        if ($line -match '^Neru version (.+)$') { return $Matches[1].Trim() }
    }
    return ''
}

# A release tag is exactly vX.Y.Z; a source build carries git describe's
# -N-gSHA or -dirty suffix behind it.
$installedVersion = Get-InstalledVersion
$installedChannel = ''
if (Test-Path $exe) {
    $installedChannel = if (-not $installedVersion) { 'unknown' }
    elseif ($installedVersion -like 'nightly*') { 'nightly' }
    elseif ($installedVersion -match '^v\d+\.\d+\.\d+$') { 'stable' }
    else { 'source' }
    Write-Ok "Found Neru $installedVersion ($installedChannel, $exe)"
} else {
    Write-Info "No existing Neru at $exe"
}

function Stop-InstalledNeru {
    # Windows refuses to overwrite a running executable, so anything running
    # from $exe has to go first. Returns whether a login task was registered
    # and whether a bare daemon was running.
    $result = @{ Service = $false; Daemon = $false }
    if (-not (Test-Path $exe)) { return $result }
    if ((Invoke-Neru services status) -match '^Service installed') {
        $result.Service = $true
        Invoke-Neru services uninstall | Out-Null
    }
    Invoke-Neru status | Out-Null
    if ($script:neruExit -eq 0) {
        $result.Daemon = $true
        Invoke-Neru stop | Out-Null
    }
    Get-Process -Name neru -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -eq $exe } |
        ForEach-Object { $_ | Stop-Process -Force -ErrorAction SilentlyContinue }
    Start-Sleep -Milliseconds 300
    return $result
}

# --------------------------------------------------------------- uninstall --

$marker = '# neru shell completion (managed by install.ps1)'
$profilePath = $PROFILE.CurrentUserAllHosts
function Test-UserPathEntry {
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
    try {
        if ($key.GetValueNames() -notcontains 'Path') { return $false }
        return @([string]$key.GetValue('Path', '', 'DoNotExpandEnvironmentNames') -split ';') -contains $installDir
    } finally { $key.Close() }
}
function Test-ProfileCompletion {
    return (Test-Path $profilePath) -and (Select-String -Path $profilePath -SimpleMatch $marker -Quiet)
}

if ($Uninstall) {
    # An interrupted uninstall may have removed the executable and shortcut
    # already, so the PATH entry and profile block count as leftovers too.
    if (-not (Test-Path $exe) -and -not (Test-Path $shortcutPath) -and -not (Test-UserPathEntry) -and -not (Test-ProfileCompletion)) {
        Write-Host ''
        Write-Host '  Nothing to uninstall.'
        Stop-Installer 0
    }
    Write-Step 'Removing Neru'
    Write-Note 'The login task, executable, PATH entry, Start Menu shortcut and completion go.'
    if ($Purge) {
        Write-Note 'Config, data and logs are removed too, each after its own confirmation.'
    } else {
        Write-Note 'Config, data and logs are kept (add -Purge to remove them).'
    }
    if (-not (Ask 'Uninstall Neru?')) {
        Write-Host ''
        Write-Host '  Nothing changed.'
        Stop-Installer 0
    }
    $stopped = Stop-InstalledNeru
    if ($stopped.Service) { Write-Ok 'Login task removed' }
    Remove-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name Neru -ErrorAction SilentlyContinue
    if (Test-Path $shortcutPath) { Remove-Item $shortcutPath -Force; Write-Ok "Removed $shortcutPath" }
    if (Test-Path $installDir) { Remove-Item $installDir -Recurse -Force; Write-Ok "Removed $installDir" }

    # PATH entry, edited the same way it was added.
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
    try {
        if ($key.GetValueNames() -contains 'Path') {
            $kind = $key.GetValueKind('Path')
            $entries = @([string]$key.GetValue('Path', '', 'DoNotExpandEnvironmentNames') -split ';' | Where-Object { $_ -ne '' })
            if ($entries -contains $installDir) {
                $key.SetValue('Path', (($entries | Where-Object { $_ -ne $installDir }) -join ';'), $kind)
                Write-Ok 'Removed the user PATH entry (open a new terminal to pick it up)'
            }
        }
    } finally {
        $key.Close()
    }

    # The completion block in the profile, marker line plus the line after it.
    if (Test-ProfileCompletion) {
        $kept = New-Object System.Collections.Generic.List[string]
        $skip = 0
        foreach ($line in (Get-Content $profilePath)) {
            if ($skip -gt 0) { $skip--; continue }
            if ($line -eq $marker) { $skip = 1; continue }
            $kept.Add($line)
        }
        Set-Content -Path $profilePath -Value $kept
        Write-Ok "Removed completion from $profilePath"
    }

    if ($Purge) {
        Write-Step 'Removing config, data and logs'
        foreach ($d in @((Join-Path $env:APPDATA 'neru'), (Join-Path $env:LOCALAPPDATA 'neru'))) {
            if (-not (Test-Path $d)) { continue }
            if (Ask "Delete $d`?") { Remove-Item $d -Recurse -Force; Write-Ok "Removed $d" } else { Write-Info "Kept $d" }
        }
    }

    Write-Host ''
    Write-Piece '  Neru is uninstalled.' 'Green'
    if (-not $Purge) { Write-Note 'Config, data and logs were kept. Rerun with -Uninstall -Purge to remove them.' }
    Write-Host ''
    Stop-Installer 0
}

# ----------------------------------------------------------------- channel --

if (-not $Channel) {
    $Channel = if ($installedChannel -in 'stable', 'nightly') { $installedChannel } else { 'stable' }
}

if ($installedChannel -and $installedChannel -ne $Channel) {
    Write-Host ''
    switch -Wildcard ("$installedChannel`:$Channel") {
        '*:source' {
            Write-Warn "This replaces the installed $installedChannel build with your local build from $From."
            Write-Note 'A later run without -From goes back to a release and asks first.'
        }
        'stable:nightly' {
            Write-Warn 'You have the stable release installed and asked for nightly.'
            Write-Note 'Nightly is rebuilt from every push to main. It may carry regressions, half-finished'
            Write-Note 'features and config keys that later change. Every future run of this script keeps'
            Write-Note 'you on nightly unless you pass -Channel stable.'
        }
        'nightly:stable' {
            Write-Warn 'You have a nightly build installed and asked for stable.'
            Write-Note 'Stable is older than your nightly, so options that only exist on main are rejected'
            Write-Note 'by config validation until they ship in a release.'
        }
        default {
            Write-Warn "The installed Neru is a $installedChannel build (from 'just install', most likely)."
            Write-Note "Continuing replaces it with the $Channel release."
        }
    }
    if (-not (Ask "Switch to $Channel`?")) {
        Write-Host ''
        Write-Host '  Nothing changed.'
        Stop-Installer 0
    }
}

# ------------------------------------------------------------------- fetch --

$tmp = Join-Path ([IO.Path]::GetTempPath()) "neru-install-$([Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    # $src is the release layout to install from: bin\neru.exe and share\man.
    # A downloaded zip and a `just dist` tree share it.
    if ($From) {
        $src = $From
        $targetLabel = 'local build'
    } else {
        if ($Channel -eq 'nightly') {
            $tag = 'nightly'
        } elseif ($Version) {
            $tag = $Version
        } else {
            try {
                $tag = (Invoke-RestMethod -Uri "$apiBase/releases/latest" -Headers @{ 'User-Agent' = 'neru-installer' }).tag_name
            } catch {
                Fail "Could not determine the latest release from $apiBase/releases/latest`: $_"
            }
            if (-not $tag) { Fail 'The latest release has no tag name.' }
        }

        if ($Channel -eq 'stable' -and $installedVersion -eq $tag -and -not $Force) {
            Write-Host ''
            Write-Piece "  Neru $tag is already installed and up to date." 'Green'
            Write-Note 'Pass -Force to reinstall it anyway.'
            Write-Host ''
            Stop-Installer 0
        }
        $targetLabel = if ($Channel -eq 'nightly') { 'the latest nightly build' } else { "Neru $tag" }

        Write-Step "Fetching $targetLabel"
        $url = "$ghBase/releases/download/$tag/$asset"
        Write-Info $url
        $zip = Join-Path $tmp $asset
        Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
        Invoke-WebRequest -Uri "$url.sha256" -OutFile "$zip.sha256" -UseBasicParsing
        Write-Ok "Downloaded $asset"

        $expected = ((Get-Content "$zip.sha256" -Raw) -split '\s+')[0].ToLower()
        $actual = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
        if ($expected -ne $actual) { Fail "Checksum mismatch for $asset. Refusing to install a corrupted download." }
        Write-Ok 'Checksum verified'

        $src = Join-Path $tmp 'unpacked'
        Expand-Archive -Path $zip -DestinationPath $src -Force
    }
    $newExe = Join-Path $src 'bin\neru.exe'
    if (-not (Test-Path $newExe)) { Fail 'The release archive did not contain bin\neru.exe.' }

    # ------------------------------------------------------------- install --

    if ($installedChannel) {
        Write-Step "Updating $(if ($installedVersion) { $installedVersion } else { 'Neru' }) to $targetLabel"
    } else {
        Write-Step "Installing $targetLabel"
    }

    $stopped = Stop-InstalledNeru
    $serviceWasInstalled = $stopped.Service
    $daemonWasRunning = $stopped.Daemon
    $script:serviceUnloaded = $serviceWasInstalled
    if ($serviceWasInstalled) { Write-Info 'Paused the login task for the update' }
    if ($daemonWasRunning) { Write-Info 'Stopped the running daemon' }

    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    Copy-Item -Path $newExe -Destination $exe -Force
    $manSrc = Join-Path $src 'share'
    if (Test-Path $manSrc) {
        $manDst = Join-Path $installDir 'share'
        if (Test-Path $manDst) { Remove-Item $manDst -Recurse -Force }
        Copy-Item -Path $manSrc -Destination $manDst -Recurse
    }
    Write-Ok "neru.exe $([char]0x2192) $installDir"
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# The per-user Environment key is edited directly rather than through setx,
# which truncates at 1024 characters and expands other tools' %VAR% entries.
$pathState = 'present'
$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
try {
    $kind = [Microsoft.Win32.RegistryValueKind]::ExpandString
    $current = ''
    if ($key.GetValueNames() -contains 'Path') {
        $current = [string]$key.GetValue('Path', '', 'DoNotExpandEnvironmentNames')
        $kind = $key.GetValueKind('Path')
    }
    $entries = @($current -split ';' | Where-Object { $_ -ne '' })
    if ($entries -notcontains $installDir) {
        $key.SetValue('Path', (($entries + $installDir) -join ';'), $kind)
        $pathState = 'added'
    }
} finally {
    $key.Close()
}
if ($pathState -eq 'added') {
    # Tell running processes (Explorer above all) to reread the environment so
    # a shell opened afterwards sees the entry without a sign-out.
    if (-not ('Neru.Env' -as [type])) {
        Add-Type -Namespace Neru -Name Env -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg,
    UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout,
    out UIntPtr lpdwResult);
'@
    }
    $result = [UIntPtr]::Zero
    [void][Neru.Env]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 0x2, 5000, [ref]$result)
    Write-Ok "User PATH $([char]0x2192) $installDir added"
} else {
    Write-Ok 'User PATH already includes the install directory'
}
if (($env:Path -split ';') -notcontains $installDir) { $env:Path = "$env:Path;$installDir" }

# This, not PATH, is what makes Neru searchable from the taskbar.
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $shortcutPath) | Out-Null
$shortcut = (New-Object -ComObject WScript.Shell).CreateShortcut($shortcutPath)
$shortcut.TargetPath = $exe
$shortcut.Arguments = 'launch'
$shortcut.WorkingDirectory = $installDir
$shortcut.IconLocation = $exe
$shortcut.Description = 'Neru - keyboard-driven navigation'
$shortcut.WindowStyle = 7 # minimized: hides the console flash of a console-subsystem exe
$shortcut.Save()
Write-Ok 'Start Menu shortcut (searchable from the taskbar)'

$completionState = 'skipped'
if (-not $NoCompletions) {
    if (Test-ProfileCompletion) {
        $completionState = 'present'
        Write-Ok 'PowerShell completion already in your profile'
    } else {
        Write-Step 'PowerShell completion'
        Write-Note "Tab completion loads from your profile: $profilePath"
        # A profile is a script, so a Restricted or AllSigned policy would
        # block it and print an error at every new session. Offer the usual
        # per-user policy first rather than plant that error.
        $policy = Get-ExecutionPolicy
        $policyOk = $policy -notin 'Restricted', 'AllSigned'
        if (-not $policyOk) {
            Write-Warn "Your execution policy is $policy, which stops PowerShell from loading a profile."
            if (Ask 'Set it to RemoteSigned for your user account so local scripts and profiles run?') {
                Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned -Force
                $policyOk = $true
                Write-Ok 'Execution policy set to RemoteSigned for your user'
            } else {
                Write-Info 'Skipped completion; the profile would not load under this policy.'
            }
        }
        if ($policyOk -and (Ask 'Add PowerShell tab completion for neru to your profile?')) {
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $profilePath) | Out-Null
            Add-Content -Path $profilePath -Value "`n$marker`nif (Get-Command neru -ErrorAction SilentlyContinue) { neru completion powershell | Out-String | Invoke-Expression }"
            $completionState = 'added'
            Write-Ok 'Added (takes effect in a new session)'
        } else {
            Write-Info 'Skipped. Later: neru completion powershell | Out-String | Invoke-Expression'
        }
    }
}

# ----------------------------------------------------------------- service --

$serviceState = 'none'
if (-not $NoService) {
    if ($serviceWasInstalled) {
        $script:serviceUnloaded = $false
        Invoke-Neru services install | Out-Null
        if ($script:neruExit -eq 0) {
            $serviceState = 'restarted'
            Write-Ok 'Login task resumed on the new version'
        } else {
            $serviceState = 'failed'
            Write-Warn 'Could not resume the login task. Run: neru services install'
        }
    } elseif (-not $daemonWasRunning) {
        Write-Step 'Login task'
        Write-Note 'Neru runs as a background daemon. Registering it starts it now and at every login.'
        if (Ask 'Start Neru now and at every login?') {
            Invoke-Neru services install | Out-Null
            if ($script:neruExit -eq 0) {
                $serviceState = 'installed'
                # An older installer wrote a Run key for the same purpose; left
                # in place it would launch a second daemon at every login.
                Remove-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name Neru -ErrorAction SilentlyContinue
                Write-Ok 'Registered and running'
            } else {
                $serviceState = 'failed'
                Write-Warn 'Registration failed. Retry later with: neru services install'
            }
        } else {
            Write-Info 'Skipped. Start it any time with: neru launch'
        }
    }
}

# ---------------------------------------------------------------- manifest --

$newVersion = Get-InstalledVersion
@(
    "channel=$Channel"
    "version=$newVersion"
    "binary=$exe"
    "shortcut=$shortcutPath"
    "completion=$(if ($completionState -in 'added', 'present') { $PROFILE.CurrentUserAllHosts } else { '' })"
    "installed_at=$((Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ'))"
) | Set-Content -Path $manifest

# ----------------------------------------------------------------- summary --

Write-Host ''
Write-Piece "  Neru $newVersion is installed." 'Green' -NoNewline
Write-Piece " ($Channel)" 'DarkGray'
Write-Host ''
Write-Piece '  Next steps' 'White'
$script:n = 0
function Write-Next([string]$Text) { $script:n++; Write-Piece "  $($script:n)." 'DarkGray' -NoNewline; Write-Piece " $Text" }
if ($serviceState -notin 'installed', 'restarted') {
    if ($daemonWasRunning) {
        Write-Next 'Start the daemon again: neru launch (it was stopped for the update).'
    } else {
        Write-Next 'Start it: neru launch, or neru services install to start at login. The Start Menu entry works too.'
    }
}
if ($pathState -eq 'added') { Write-Next "Open a new terminal so 'neru' resolves by name." }
Write-Next 'Configure: neru config init writes a starter config.toml.'
Write-Host ''
Write-Note 'Windows support is beta; see docs/CROSS_PLATFORM.md for what works today.'
Write-Note "Manage the task with 'neru services status|stop|restart'. Rerun this script to update,"
Write-Note 'add -Channel stable|nightly to switch, or -Uninstall to remove.'
Write-Host ''
}

$exitCode = 0
try {
    Invoke-NeruInstaller
} catch {
    if ($_.Exception.Message -match '^NERU_EXIT:(\d+)$') {
        $exitCode = [int]$Matches[1]
    } else {
        Write-Host ''
        Write-Piece "  $([char]0x2717) Unexpected failure: $($_.Exception.Message)" 'Red'
        Write-Note "  at $($_.InvocationInfo.PositionMessage -replace '\s+', ' ')"
        Write-Note "Please report this with the lines above at $ghBase/issues."
        Restore-LoginTask
        $exitCode = 1
    }
}
# Run as a file (-File, or just install) the exit code matters and exiting is
# safe. Piped into iex there is no file, and exit would close the session.
if ($MyInvocation.MyCommand.Path) { exit $exitCode }
