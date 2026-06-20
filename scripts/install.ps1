<#
.SYNOPSIS
    Senda installer for Windows.

.DESCRIPTION
    Downloads a prebuilt Senda release archive, verifies its SHA-256 checksum,
    and installs senda.exe (desktop app) and senda-cli.exe.

.EXAMPLE
    irm https://raw.githubusercontent.com/this-senda/senda/main/scripts/install.ps1 | iex

.PARAMETER Version
    Install a specific version (e.g. 0.1.0). Defaults to the latest release.

.PARAMETER InstallDir
    Target directory. Defaults to %LOCALAPPDATA%\Programs\Senda.

.PARAMETER NoCli
    Skip installing senda-cli.exe.
#>
[CmdletBinding()]
param(
    [string]$Version = $env:SENDA_VERSION,
    [string]$InstallDir = $env:SENDA_INSTALL_DIR,
    [switch]$NoCli
)

$ErrorActionPreference = "Stop"
$Repo = "this-senda/senda"

function Info($m) { Write-Host "» $m" -ForegroundColor DarkGray }
function Ok($m)   { Write-Host "✓ $m" -ForegroundColor Green }
function Warn($m) { Write-Host "! $m" -ForegroundColor Yellow }
function Die($m)  { Write-Host "✗ $m" -ForegroundColor Red; exit 1 }

# --- detect architecture ----------------------------------------------------
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { Die "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}
if ($arch -ne "amd64") {
    Die "no prebuilt Windows/$arch binary yet — build from source (see the README)"
}

# --- resolve version --------------------------------------------------------
if (-not $Version) {
    Info "resolving latest release…"
    $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" `
        -Headers @{ "User-Agent" = "senda-installer" }
    $Version = $rel.tag_name
}
$Version = $Version -replace '^v', ''
Info "installing Senda v$Version for windows/$arch"

# --- install dir ------------------------------------------------------------
if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\Senda"
}
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# --- download + verify ------------------------------------------------------
# Release zips are named senda_<version>_windows-<arch>.zip and contain
# senda.exe and senda-cli.exe at the archive root.
$asset = "senda_${Version}_windows-${arch}.zip"
$base  = "https://github.com/$Repo/releases/download/v$Version"
$tmp   = Join-Path ([System.IO.Path]::GetTempPath()) ("senda_" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
    $zip = Join-Path $tmp $asset
    Info "downloading $asset…"
    Invoke-WebRequest -Uri "$base/$asset" -OutFile $zip -UseBasicParsing

    try {
        $sumsText = (Invoke-WebRequest -Uri "$base/checksums.txt" -UseBasicParsing).Content
        $line = $sumsText -split "`n" | Where-Object { $_ -match [regex]::Escape($asset) } | Select-Object -First 1
        if ($line) {
            Info "verifying checksum…"
            $expected = ($line -split '\s+')[0].ToLower()
            $actual = (Get-FileHash -Algorithm SHA256 -Path $zip).Hash.ToLower()
            if ($expected -ne $actual) { Die "checksum mismatch for $asset" }
            Ok "checksum verified"
        } else {
            Warn "no checksum entry for $asset — skipping verification"
        }
    } catch {
        Warn "checksum file unavailable — skipping verification"
    }

    # --- extract + install --------------------------------------------------
    Expand-Archive -Path $zip -DestinationPath $tmp -Force

    Copy-Item -Force (Join-Path $tmp "senda-desktop.exe") (Join-Path $InstallDir "senda-desktop.exe")
    Ok "installed $InstallDir\senda-desktop.exe"

    if (-not $NoCli -and (Test-Path (Join-Path $tmp "senda-cli.exe"))) {
        Copy-Item -Force (Join-Path $tmp "senda-cli.exe") (Join-Path $InstallDir "senda-cli.exe")
        Ok "installed $InstallDir\senda-cli.exe"
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

# --- add to user PATH -------------------------------------------------------
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ';') -notcontains $InstallDir) {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    Warn "added $InstallDir to your user PATH — restart your terminal to pick it up"
}

Write-Host ""
Write-Host "Senda v$Version installed." -ForegroundColor Green -NoNewline
Write-Host " Run 'senda' to launch, or 'senda-cli -h' for the runner."
