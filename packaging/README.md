# Packaging & distribution

This directory holds everything needed to ship Senda to users through the common
package managers. All of it is driven by the GitHub Release assets produced by
[`.github/workflows/release.yml`](../.github/workflows/release.yml).

## Release artifact convention

When a semver tag (`vX.Y.Z`, or `vX.Y.Z-rc.N` for a prerelease) is pushed, the
release workflow builds and publishes:

| Asset                                    | Platform          |
| ---------------------------------------- | ----------------- |
| `senda_<version>_linux-amd64.tar.gz`     | Linux x86-64      |
| `senda_<version>_linux-arm64.tar.gz`     | Linux ARM64       |
| `senda_<version>_darwin-amd64.tar.gz`    | macOS Intel       |
| `senda_<version>_darwin-arm64.tar.gz`    | macOS Apple Si.   |
| `senda_<version>_windows-amd64.zip`      | Windows x86-64    |
| `checksums.txt`                          | SHA-256 sums      |

Each archive contains two binaries at its root: `senda` (the desktop app) and
`senda-cli` (the headless runner).

## End-user install paths

### Shell installer (Linux / macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/this-senda/senda/main/scripts/install.sh | sh
```

### PowerShell installer (Windows)

```powershell
irm https://raw.githubusercontent.com/this-senda/senda/main/scripts/install.ps1 | iex
```

### Homebrew (macOS / Linux)

```sh
brew install this-senda/tap/senda
```

### winget (Windows — primary)

```powershell
winget install this-senda.Senda
```

### Chocolatey (Windows — alternative)

```powershell
choco install senda
```

## Cutting a release (maintainers)

1. Tag a commit **on `main`** and push: `git tag v0.1.0 && git push origin v0.1.0`.
   The release workflow builds every platform, publishes the assets above, and —
   for stable (non-prerelease) tags — **auto-generates the Homebrew cask** and
   pushes it to `this-senda/homebrew-tap` (`Casks/senda.rb`). Homebrew needs no
   manual step.
2. Bump the Chocolatey + winget manifests from the published checksums:

   ```sh
   packaging/update.sh 0.1.0
   ```

   This rewrites the version + SHA-256 values in:
   - `chocolatey/senda.nuspec` + `chocolatey/tools/chocolateyinstall.ps1`
   - `winget/this-senda.Senda*.yaml`
3. Publish the two manifests:
   - **winget** — submit `winget/` to
     [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs), e.g.
     `wingetcreate submit packaging/winget` or open a PR placing the three YAML
     files under `manifests/t/this-senda/Senda/0.1.0/`.
   - **Chocolatey** — `cd chocolatey && choco pack && choco push senda.0.1.0.nupkg`.

The `scripts/install.sh` and `scripts/install.ps1` installers need no per-release
edits — they resolve the latest release (or `$SENDA_VERSION`) at runtime.
