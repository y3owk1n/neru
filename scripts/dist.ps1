<#
.SYNOPSIS
Assemble the Windows release layout: bin\neru.exe and share\man\man1. It is
the exact tree a release zip unpacks to, so scripts\install.ps1 -From and
CI's publish jobs both consume it. Invoked by `just dist` on Windows; the
macOS and Linux counterpart is scripts/dist.sh.

    scripts\dist.ps1 [-Bin bin\neru.exe] [-Out build\dist]
#>
[CmdletBinding()]
param(
    [string]$Bin = '',
    [string]$Out = 'build\dist'
)
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

if (-not $Bin) { $Bin = 'bin\neru.exe' }
if (-not (Test-Path $Bin)) {
    [Console]::Error.WriteLine("dist: $Bin not found; run 'just build' first")
    exit 1
}

if (Test-Path $Out) { Remove-Item $Out -Recurse -Force }
$binDir = Join-Path $Out 'bin'
$manDir = Join-Path $Out 'share\man\man1'
New-Item -ItemType Directory -Force -Path $binDir, $manDir | Out-Null
Copy-Item $Bin (Join-Path $binDir 'neru.exe')
& go run ./cmd/genman $manDir | Out-Null
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "$([char]0x2713) Release layout assembled in $Out"
