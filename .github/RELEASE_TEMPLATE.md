# Release Template

Use this template when preparing a new release.

## Pre-Release Checklist

- [ ] Update version number in relevant files
- [ ] Update CHANGELOG.md with new version
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `golangci-lint run`
- [ ] Build for all platforms: `make build-all`
- [ ] Test the `ccp` wrapper locally
- [ ] Update documentation if needed

## CHANGELOG.md Template

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- New feature description

### Changed
- Changed feature description

### Fixed
- Bug fix description

### Removed
- Removed feature description

### Security
- Security fix description
```

## Creating the Release

### 1. Update CHANGELOG.md
Move items from `[Unreleased]` section to new version section:

```bash
# Example for v1.2.0
vim CHANGELOG.md
# Move unreleased changes to new [1.2.0] section
# Add release date
```

### 2. Commit the changelog
```bash
git add CHANGELOG.md
git commit -m "docs: Update CHANGELOG for v1.2.0"
git push
```

### 3. Create Git Tag
```bash
git tag -a v1.2.0 -m "Release v1.2.0: Brief description"
git push origin v1.2.0
```

### 4. Create GitHub Release
```bash
gh release create v1.2.0 \
  --title "v1.2.0 - Release Title" \
  --notes-file <(sed -n '/## \[1.2.0\]/,/## \[/p' CHANGELOG.md | head -n -1)
```

Or manually via GitHub UI:
1. Go to https://github.com/nielspeter/claude-code-proxy/releases/new
2. Select tag: `v1.2.0`
3. Copy release notes from CHANGELOG.md
4. Publish release

## Semantic Versioning Guidelines

- **MAJOR** (X.0.0): Breaking changes, incompatible API changes
- **MINOR** (1.X.0): New features, backward-compatible
- **PATCH** (1.1.X): Bug fixes, backward-compatible

### Examples

- `v1.2.0`: Added new model provider support ✅ MINOR
- `v1.1.1`: Fixed streaming bug ✅ PATCH
- `v2.0.0`: Changed config file format (breaking) ✅ MAJOR

## Conventional Commits

Use these prefixes for commits (helps with changelog generation):

- `feat:` - New feature (MINOR version bump)
- `fix:` - Bug fix (PATCH version bump)
- `docs:` - Documentation only
- `refactor:` - Code refactoring
- `test:` - Adding tests
- `chore:` - Maintenance tasks
- `perf:` - Performance improvements
- `ci:` - CI/CD changes

### Examples
```bash
feat: Add support for Azure OpenAI provider
fix: Resolve streaming token count issue
docs: Update installation instructions
test: Add unit tests for model detection
```

## Post-Release Tasks

- [ ] Verify release appears on GitHub
- [ ] Test installation from release binaries
- [ ] Announce release (if applicable)
- [ ] Update any dependent projects
- [ ] Start new `[Unreleased]` section in CHANGELOG.md
