# Release v{{VERSION}}

## Highlights

<!-- Brief summary of the most important changes in this release -->

## What's Changed

### New Features
<!-- List of new features added in this release -->

### Bug Fixes
<!-- List of bugs fixed in this release -->

### Documentation
<!-- Documentation improvements -->

### Maintenance
<!-- Internal improvements, dependency updates, etc. -->

## Breaking Changes
<!-- Any breaking changes that require user action -->

## Installation

### Homebrew (macOS/Linux)
```bash
brew install drapon/tap/envy
# or upgrade
brew upgrade envy
```

### Direct Download
```bash
# macOS (Intel)
curl -sSL https://github.com/drapon/envy/releases/download/v{{VERSION}}/envy_{{VERSION}}_darwin_amd64.tar.gz | tar xz
sudo mv envy /usr/local/bin/

# macOS (Apple Silicon)
curl -sSL https://github.com/drapon/envy/releases/download/v{{VERSION}}/envy_{{VERSION}}_darwin_arm64.tar.gz | tar xz
sudo mv envy /usr/local/bin/

# Linux (x86_64)
curl -sSL https://github.com/drapon/envy/releases/download/v{{VERSION}}/envy_{{VERSION}}_linux_amd64.tar.gz | tar xz
sudo mv envy /usr/local/bin/
```

### Go Install
```bash
go install github.com/drapon/envy@v{{VERSION}}
```

## Checksums

```
SHA256 checksums:
{{CHECKSUMS}}
```

## Acknowledgments

Thanks to all contributors who made this release possible!

## Full Changelog

**Full Changelog**: https://github.com/drapon/envy/compare/v{{PREVIOUS_VERSION}}...v{{VERSION}}