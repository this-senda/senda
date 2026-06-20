$ErrorActionPreference = 'Stop'

# Bumped per release by packaging/update.sh.
$version  = '0.0.0'
$checksum = 'REPLACE_WITH_WINDOWS_AMD64_SHA256'

$packageName = 'senda'
$toolsDir    = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$url64       = "https://github.com/this-senda/senda/releases/download/v$version/senda_${version}_windows-amd64.zip"

$packageArgs = @{
  packageName    = $packageName
  unzipLocation  = $toolsDir
  url64bit       = $url64
  checksum64     = $checksum
  checksumType64 = 'sha256'
}

Install-ChocolateyZipPackage @packageArgs

# Choco auto-shims every .exe under the tools dir, so both `senda-desktop` and
# `senda-cli` become available on PATH after install.
