# GitHub Actions Workflows

This directory contains the GitHub Actions workflows for devManager's CI/CD pipeline.

## Workflows Overview

### 🚀 `release.yml` - Main Release Automation
**Trigger:** Push to tags starting with `v*` or manual dispatch

**Features:**
- Version consistency validation
- Multi-platform builds (Windows, macOS, Linux × amd64/arm64)
- Automatic GitHub release creation
- Changelog extraction from CHANGELOG.md
- Prerelease detection
- Build status summary

**Jobs:**
1. **validate** - Checks version consistency and extracts changelog
2. **build** - Builds binaries for all platforms
3. **create-release** - Creates GitHub release with artifacts
4. **notify** - Posts build summary

### 🔍 `version-check.yml` - Version Validation
**Trigger:** Pull requests and pushes to master (when version files change)

**Features:**
- Version consistency checks across all files
- Semantic versioning validation
- Changelog entry verification
- PR comments with version status
- Version bump detection

**Validations:**
- All version files match (index.html, package.json, wails.json)
- Version follows semantic versioning
- Changelog entry exists for current version
- Version is bumped when appropriate

### 🧹 `cleanup.yml` - Artifact Cleanup
**Trigger:** Weekly schedule or manual dispatch

**Features:**
- Deletes artifacts older than 7 days
- Keeps repository clean
- Runs automatically every Sunday

### 🏗️ `build-multiplatform.yml` - Legacy Build System
**Trigger:** Push to tags or manual dispatch

**Note:** This is the original build system. The new `release.yml` replaces it with enhanced features.

## File Structure

```
.github/workflows/
├── README.md              # This file
├── release.yml            # Main release automation
├── version-check.yml      # Version validation
├── cleanup.yml            # Artifact cleanup
└── build-multiplatform.yml # Legacy build system
```

## Release Process

### Automated Release (Recommended)

1. **Bump version locally:**
   ```bash
   ./scripts/bump-version.sh patch "Description of changes"
   ```

2. **Push to GitHub:**
   ```bash
   git push origin master
   git push origin v1.0.1
   ```

3. **GitHub Actions automatically:**
   - Validates version consistency
   - Builds for all platforms
   - Creates GitHub release
   - Uploads artifacts

### Manual Release with Workflow

1. **Go to GitHub Actions → Workflows → Release Automation**
2. **Click "Run workflow"**
3. **Enter version and options**
4. **Monitor progress**

## Version Validation

The `version-check.yml` workflow ensures:

### ✅ Version Consistency
All files must have the same version:
- `devmanager-app/frontend/index.html` → `v1.0.1`
- `devmanager-app/frontend/package.json` → `"1.0.1"`
- `devmanager-app/wails.json` → `"1.0.1"`

### ✅ Semantic Versioning
Versions must follow:
- `MAJOR.MINOR.PATCH` (e.g., `1.0.1`)
- `MAJOR.MINOR.PATCH-prerelease.version` (e.g., `1.1.0-alpha.1`)

### ✅ Changelog Entries
Each version should have an entry in `CHANGELOG.md`:
```markdown
## [1.0.1] - 2025-01-28

### 🚀 Features
- New feature description

### 🐛 Bug Fixes
- Bug fix description
```

## Build Matrix

The workflows build for all platforms:

| Platform | Architecture | Artifact |
|----------|-------------|----------|
| Windows | amd64 | `devmanager-windows-amd64.zip` |
| Windows | arm64 | `devmanager-windows-arm64.zip` |
| macOS | amd64 | `devmanager-macos-amd64.zip` |
| macOS | arm64 | `devmanager-macos-arm64.zip` |
| Linux | amd64 | `devmanager-linux-amd64.zip` |
| Linux | arm64 | `devmanager-linux-arm64.zip` |

## Environment Variables

### Global
- `WAILS_VERSION: v2.15.0` - Wails framework version

### Secrets
- `GITHUB_TOKEN` - Automatically provided by GitHub Actions

## Permissions Required

The workflows require these permissions:

### `release.yml`
- `contents: write` - To create releases

### `version-check.yml`
- `pull-requests: write` - To comment on PRs
- `contents: read` - To read files

### `cleanup.yml`
- `actions: read` - To list and delete artifacts

## Troubleshooting

### Common Issues

1. **Version inconsistency:**
   ```
   ❌ HTML version (v1.0.0) doesn't match package version (1.0.1)
   ```
   **Solution:** Use `./scripts/bump-version.sh` to update all files consistently.

2. **Missing changelog entry:**
   ```
   ⚠️ No changelog entry found for version 1.0.1
   ```
   **Solution:** Add entry to `CHANGELOG.md` before pushing.

3. **Build failures:**
   ```
   ❌ Build failed for windows-amd64
   ```
   **Solution:** Check workflow logs for specific error messages.

4. **Release creation failed:**
   ```
   ❌ Release creation failed
   ```
   **Solution:** Ensure tag exists and has proper permissions.

### Debugging Steps

1. **Check workflow logs:** GitHub Actions → Workflow run → Jobs
2. **Validate locally:** Run `./scripts/check-release.sh`
3. **Verify versions:** Check all version files manually
4. **Test build:** Try building locally with `wails build`

## Best Practices

### Before Releasing
1. **Test locally:** Build and test the application
2. **Update changelog:** Document all changes
3. **Check versions:** Ensure consistency across files
4. **Review changes:** Double-check PR content

### After Releasing
1. **Monitor builds:** Watch GitHub Actions progress
2. **Test binaries:** Download and test artifacts
3. **Verify release:** Check GitHub release page
4. **Announce:** Share release with users

### Maintenance
1. **Regular cleanup:** Old artifacts are cleaned weekly
2. **Workflow updates:** Keep dependencies current
3. **Monitor failures:** Fix issues promptly
4. **Documentation:** Keep this README updated

## Migration from Legacy System

To migrate from `build-multiplatform.yml` to `release.yml`:

1. **Test new workflow:** Use manual dispatch first
2. **Compare results:** Ensure same artifacts are produced
3. **Update documentation:** Point to new workflow
4. **Remove legacy:** Delete `build-multiplatform.yml` when confident

## Future Enhancements

Potential improvements:
- [ ] Slack/Discord notifications for releases
- [ ] Automatic binary testing
- [ ] Integration with package managers
- [ ] Release draft creation for review
- [ ] Performance benchmarking
- [ ] Security scanning
- [ ] Dependency update automation