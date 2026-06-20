#!/usr/bin/env bash
# Bump the Chocolatey and winget package definitions to a released version by
# pulling the published SHA-256 checksums from the GitHub release.
#
# Usage:
#   packaging/update.sh 0.1.0
#
# Run this AFTER the release workflow has published v<version> (so checksums.txt
# exists). It rewrites:
#   - packaging/chocolatey/senda.nuspec
#   - packaging/chocolatey/tools/chocolateyinstall.ps1
#   - packaging/winget/this-senda.Senda*.yaml
#
# Homebrew is NOT handled here — the release workflow auto-generates and pushes
# the cask to this-senda/homebrew-tap on every stable release.
set -euo pipefail

VERSION="${1:-}"
[ -n "$VERSION" ] || { echo "usage: $0 <version>  (e.g. 0.1.0)" >&2; exit 1; }
VERSION="${VERSION#v}"

REPO="this-senda/senda"
HERE="$(cd "$(dirname "$0")" && pwd)"
SUMS_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"

echo "» fetching ${SUMS_URL}"
SUMS="$(curl -fsSL "$SUMS_URL")"

sum_for() { printf '%s\n' "$SUMS" | grep " $1\$" | awk '{print $1}'; }

WINDOWS_AMD64="$(sum_for "senda_${VERSION}_windows-amd64.zip")"
[ -n "$WINDOWS_AMD64" ] || { echo "missing checksum for senda_${VERSION}_windows-amd64.zip" >&2; exit 1; }

# --- Chocolatey -------------------------------------------------------------
NUSPEC="${HERE}/chocolatey/senda.nuspec"
sed -i.bak -E "s#<version>[^<]*</version>#<version>${VERSION}</version>#" "$NUSPEC"
rm -f "${NUSPEC}.bak"
echo "✓ updated ${NUSPEC}"

CHOCO_INSTALL="${HERE}/chocolatey/tools/chocolateyinstall.ps1"
sed -i.bak -E "s/\\\$version  = '[^']*'/\$version  = '${VERSION}'/" "$CHOCO_INSTALL"
sed -i.bak "s/REPLACE_WITH_WINDOWS_AMD64_SHA256/${WINDOWS_AMD64}/" "$CHOCO_INSTALL"
rm -f "${CHOCO_INSTALL}.bak"
echo "✓ updated ${CHOCO_INSTALL}"

# --- winget -----------------------------------------------------------------
# The placeholder version (0.0.0) appears in PackageVersion and the installer
# URL; replace every occurrence. (ManifestVersion 1.6.0 is untouched.)
for wf in "${HERE}"/winget/this-senda.Senda*.yaml; do
  sed -i.bak "s/0\.0\.0/${VERSION}/g" "$wf"
  rm -f "${wf}.bak"
done
sed -i.bak "s/REPLACE_WITH_WINDOWS_AMD64_SHA256/${WINDOWS_AMD64}/" \
  "${HERE}/winget/this-senda.Senda.installer.yaml"
rm -f "${HERE}/winget/this-senda.Senda.installer.yaml.bak"
echo "✓ updated ${HERE}/winget/*.yaml"

echo
echo "Done. Review the diff, then:"
echo "  • (cd packaging/chocolatey && choco pack && choco push senda.${VERSION}.nupkg)"
echo "  • submit packaging/winget/ to microsoft/winget-pkgs"
echo "    (e.g. 'wingetcreate submit packaging/winget' or open a PR under"
echo "     manifests/t/this-senda/Senda/${VERSION}/)"
