# Release Workflow Guide

This guide explains how to create releases for claude-code-proxy.

## Files Created

1. **`CHANGELOG.md`** - Main changelog following [Keep a Changelog](https://keepachangelog.com/) format
2. **`.github/RELEASE_TEMPLATE.md`** - Template and checklist for creating releases
3. **`.github/workflows/release.yml`** - Automated GitHub Actions workflow (updated)

## Quick Release Process

### 1. Update CHANGELOG.md

Before creating a release, move unreleased changes to a new version section:

```bash
# Edit CHANGELOG.md
vim CHANGELOG.md
```

Change:
```markdown
## [Unreleased]
### Added
- New feature X
- New feature Y
```

To:
```markdown
## [Unreleased]

## [1.2.0] - 2025-11-02
### Added
- New feature X
- New feature Y
```

### 2. Commit and Push

```bash
git add CHANGELOG.md
git commit -m "docs: Update CHANGELOG for v1.2.0"
git push
```

### 3. Create and Push Tag

```bash
git tag -a v1.2.0 -m "Release v1.2.0: Brief description"
git push origin v1.2.0
```

### 4. Automated Release

The GitHub Action will automatically:
- ✅ Run all tests
- ✅ Build binaries for all platforms (Linux, macOS, Windows, ARM, x86)
- ✅ Create checksums
- ✅ Extract release notes from CHANGELOG.md
- ✅ Create GitHub release with installation instructions
- ✅ Upload all binaries to the release

## What the Workflow Does

### If CHANGELOG.md has section for this version:
1. Extracts your curated changelog content
2. Appends installation instructions
3. Uses your changelog as release notes

### If no CHANGELOG.md section found:
1. Falls back to auto-generated release notes from PRs
2. Adds installation instructions

## Example CHANGELOG.md Structure

```markdown
# Changelog

## [Unreleased]
### Added
- Feature still in development

## [1.2.0] - 2025-11-02
### Added
- Dynamic model detection from provider API
- Support for new reasoning models

### Fixed
- Token counting issue in streaming mode

### Changed
- Improved error handling

## [1.1.0] - 2025-10-31
### Added
- Reasoning model support

## [1.0.0] - 2025-10-26
### Added
- Initial release
```

## Conventional Commits (Recommended)

Using conventional commits makes it easier to categorize changes:

```bash
# Features (go in "Added" section)
git commit -m "feat: Add support for Azure OpenAI"

# Bug fixes (go in "Fixed" section)
git commit -m "fix: Resolve streaming timeout issue"

# Documentation (go in "Changed" section if user-facing)
git commit -m "docs: Update installation guide"

# Tests (usually not in changelog)
git commit -m "test: Add unit tests for model detection"

# Refactoring (go in "Changed" if user-facing)
git commit -m "refactor: Simplify config loading"
```

## Versioning Strategy

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR** (2.0.0): Breaking changes
  - Example: Changed config file format
  - Example: Removed deprecated API

- **MINOR** (1.X.0): New features, backward-compatible
  - Example: Added support for new provider
  - Example: Added new CLI flag

- **PATCH** (1.1.X): Bug fixes, backward-compatible
  - Example: Fixed token counting bug
  - Example: Fixed crash on startup

## Troubleshooting

### Release workflow failed?

Check:
1. Did you run `make build-all` locally first to verify builds work?
2. Did you update CHANGELOG.md with the version number?
3. Did the tag format match `v*.*.*` (e.g., `v1.2.0` not `1.2.0`)?

### Release notes look wrong?

1. Check that CHANGELOG.md has a section `## [1.2.0]` matching your tag `v1.2.0`
2. The section should be between the version heading and the next version heading
3. Make sure there's proper markdown formatting

### Want to test locally?

```bash
# Test changelog extraction
VERSION="1.2.0"
sed -n "/## \[${VERSION}\]/,/## \[/p" CHANGELOG.md | sed '$d' | tail -n +2

# Test builds
make build-all
cd dist && sha256sum * > checksums.txt
```

## Manual Release (if needed)

If you need to create a release manually:

```bash
# Create release via GitHub CLI
gh release create v1.2.0 \
  --title "v1.2.0 - Release Title" \
  --notes "$(sed -n '/## \[1.2.0\]/,/## \[/p' CHANGELOG.md | sed '$d' | tail -n +2)" \
  dist/*
```

Or use the GitHub web interface:
1. Go to: https://github.com/nielspeter/claude-code-proxy/releases/new
2. Choose tag: `v1.2.0`
3. Copy content from CHANGELOG.md for that version
4. Upload binaries from `dist/` folder
5. Publish

## Tips

1. **Keep Unreleased section active**: Always have an `[Unreleased]` section at the top for ongoing work
2. **Update CHANGELOG as you go**: Don't wait until release time to document changes
3. **Link to issues/PRs**: Add links like `(#123)` for context
4. **Be concise**: Focus on user-facing changes, not internal refactoring (unless significant)
5. **Test first**: Always run tests and build locally before tagging

## Questions?

See:
- `.github/RELEASE_TEMPLATE.md` for detailed checklist
- `CHANGELOG.md` for examples of good changelog entries
- [Keep a Changelog](https://keepachangelog.com/) for format guidelines
