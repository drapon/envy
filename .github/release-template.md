## envy {{ .Tag }} Release Notes

{{ .Date }}

### New Features
<!-- List new features added -->

### Bug Fixes
<!-- List bugs fixed -->

### Improvements
<!-- Performance improvements, refactoring, etc. -->

### Breaking Changes
<!-- List any backward-incompatible changes -->

### Documentation
<!-- Documentation updates -->

### Security
<!-- Security-related fixes -->

---

## Installation

### Homebrew (macOS/Linux)
```bash
brew tap drapon/envy
brew install envy
```

### Binary Download
Binaries for each platform can be downloaded from the "Assets" section of this release page.

### Docker
```bash
docker pull ghcr.io/drapon/envy:{{ .Tag }}
```

### Go install
```bash
go install github.com/drapon/envy/cmd/envy@{{ .Tag }}
```

---

## Upgrade Guide

### Upgrading from Previous Version

#### Homebrew
```bash
brew upgrade envy
```

#### Binary
1. Download the new version binary
2. Replace the existing binary
3. Verify with `envy version`

#### Docker
```bash
docker pull ghcr.io/drapon/envy:{{ .Tag }}
```

### Configuration Migration
<!-- Configuration file changes required for version upgrade -->

No configuration file format changes in this version.

### Notes
<!-- Important notes for upgrading -->

---

## Change Details

### Commit Statistics
- Commits: {{ .Commits }}
- Contributors: {{ .Contributors }}

### Changed Files
<!-- List of major files changed -->

---

## Acknowledgments

Thanks to all contributors who made this release possible!

### Contributors
<!-- List of contributors -->

---

## Upcoming Features

Features planned for the next release:
- [ ] Feature 1
- [ ] Feature 2
- [ ] Feature 3

See the [roadmap](https://github.com/drapon/envy/blob/main/steps/99_Roadmap.md) for details.

---

## Known Issues

<!-- List any known issues -->

---

## Support

If you encounter issues or have questions:

1. Create an [Issue](https://github.com/drapon/envy/issues)
2. Ask in [Discussions](https://github.com/drapon/envy/discussions)
3. Check the [Documentation](https://github.com/drapon/envy/tree/main/docs)

---

**Full Changelog**: https://github.com/drapon/envy/compare/{{ .PreviousTag }}...{{ .Tag }}