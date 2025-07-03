# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Version management system implementation
- Automatic update check feature
- `version` command
- Semantic versioning support
- GitHub Releases integration
- Automated release preparation scripts
- Progress bar display for `pull` command
- `--no-progress` flag to disable progress bar
- Progress bar display for `push` command
- Interactive overwrite confirmation with arrow key navigation
- Color-coded output for success, errors, and warnings
- Automatic detection of existing `.env` files in `init` command
- Support for environment names with dots (e.g., `production.local`)
- `--skip-empty` flag for push command (default: true)
- `--allow-duplicate` flag for handling duplicate variables
- Clean log output with `--verbose` flag for detailed logging

### Changed

- Improved message output with color coding
- `pull` command output messages integrated with color system
- `push` command output messages integrated with color system
- Default behavior for backup creation (now opt-in with `--backup` flag)
- Simplified interactive prompts using arrow key navigation
- Cache serialization improved for config and environment files

### Fixed

- Cache error "invalid cached config type" resolved
- Environment names with dots not being recognized
- Interactive mode unresponsive prompts
- Empty value validation errors in AWS Parameter Store
- Duplicate variable detection and handling

### Security

- None

## [0.1.0] - 2025-07-04

### Added

- Initial release
- Basic CLI functionality
- AWS Parameter Store/Secrets Manager synchronization
- Environment variable management
- Cache system
- Configuration management

[Unreleased]: https://github.com/drapon/envy/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/drapon/envy/releases/tag/v0.1.0
