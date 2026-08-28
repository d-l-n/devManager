# Release Automation Scripts

This directory contains scripts to automate the version bumping and release process for devManager.

## Scripts Overview

### 📈 `bump-version.sh`
Automatically bumps the version number and updates all relevant files.

**Usage:**
```bash
./scripts/bump-version.sh [patch|minor|major] [description]
```

**Examples:**
```bash
# Patch release (bug fixes)
./scripts/bump-version.sh patch "Fix brutalist theme button styling"

# Minor release (new features)
./scripts/bump-version.sh minor "Add new database panel"

# Major release (breaking changes)
./scripts/bump-version.sh major "Migrate to React frontend"
```

**What it does:**
1. Updates version in `frontend/index.html`
2. Updates version in `frontend/package.json`
3. Updates version in `wails.json`
4. Adds entry to `CHANGELOG.md`
5. Creates release notes file
6. Commits changes with proper message
7. Creates git tag

### 🚀 `create-release.sh`
Creates a GitHub release from an existing tag.

**Usage:**
```bash
./scripts/create-release.sh [tag] [github_token]
```

**Examples:**
```bash
# Use latest tag
./scripts/create-release.sh

# Use specific tag
./scripts/create-release.sh v1.0.1

# Use specific tag and token
./scripts/create-release.sh v1.0.1 your_github_token
```

**What it does:**
1. Extracts release notes from `CHANGELOG.md`
2. Creates GitHub release via API
3. Provides next steps for verification

### 🔍 `check-release.sh`
Checks the status of a release and verifies everything is in order.

**Usage:**
```bash
./scripts/check-release.sh [tag]
```

**Examples:**
```bash
# Check latest tag
./scripts/check-release.sh

# Check specific tag
./scripts/check-release.sh v1.0.1
```

**What it checks:**
1. Tag existence (local and remote)
2. GitHub Actions workflow status
3. GitHub release existence and assets
4. Version consistency across files

## Typical Workflow

### For a Patch Release (v1.0.1)

1. **Make your changes:**
   ```bash
   # Work on your features/fixes
   git add .
   git commit -m "feat: improve brutalist theme button styling"
   ```

2. **Bump version:**
   ```bash
   ./scripts/bump-version.sh patch "Improve brutalist theme button styling"
   ```

3. **Push to GitHub:**
   ```bash
   git push origin master
   git push origin v1.0.1
   ```

4. **Monitor build:**
   ```bash
   ./scripts/check-release.sh v1.0.1
   ```

5. **Create release (after build completes):**
   ```bash
   ./scripts/create-release.sh v1.0.1
   ```

6. **Final verification:**
   ```bash
   ./scripts/check-release.sh v1.0.1
   ```

### For a Minor Release (v1.1.0)

Same workflow, but use:
```bash
./scripts/bump-version.sh minor "Add new database management panel"
```

### For a Major Release (v2.0.0)

Same workflow, but use:
```bash
./scripts/bump-version.sh major "Migrate to React frontend and new plugin system"
```

## Prerequisites

### Required Tools:
- **Git** (for version control)
- **jq** (for JSON processing in release script)
- **GitHub CLI** (optional, for enhanced features)

### Install GitHub CLI (recommended):
```bash
# macOS
brew install gh

# Ubuntu/Debian
sudo apt install gh

# Windows
winget install GitHub.cli

# Then authenticate
gh auth login
```

### Install jq (required):
```bash
# macOS
brew install jq

# Ubuntu/Debian
sudo apt install jq

# Windows (via Chocolatey)
choco install jq
```

## Environment Variables

### Optional:
- `GITHUB_TOKEN`: Your GitHub personal access token
  - Can be set as environment variable
  - Or passed as second argument to `create-release.sh`

### Create GitHub Token:
1. Go to GitHub Settings → Developer settings → Personal access tokens
2. Generate new token with `repo` scope
3. Set as environment variable:
   ```bash
   export GITHUB_TOKEN=your_token_here
   ```

## File Structure

```
scripts/
├── README.md              # This file
├── bump-version.sh        # Version bumping automation
├── create-release.sh      # GitHub release creation
└── check-release.sh       # Release status verification
```

## Troubleshooting

### Common Issues:

1. **Permission denied on scripts:**
   ```bash
   chmod +x scripts/*.sh
   ```

2. **Tag already exists remotely:**
   ```bash
   git tag -d v1.0.1
   git tag v1.0.1
   git push origin v1.0.1 --force  # Use with caution
   ```

3. **GitHub Actions not triggering:**
   - Ensure tag is pushed to remote
   - Check workflow file syntax
   - Verify GitHub Actions permissions

4. **Release creation fails:**
   - Check GitHub token permissions
   - Verify tag exists on remote
   - Ensure changelog has proper format

### Getting Help:

Each script has built-in help:
```bash
./scripts/bump-version.sh --help
./scripts/create-release.sh --help
./scripts/check-release.sh --help
```

## Best Practices

1. **Always test locally before pushing**
2. **Review the changelog before committing**
3. **Monitor GitHub Actions build**
4. **Test downloaded binaries before announcing**
5. **Keep release notes descriptive and user-focused**
6. **Use semantic versioning consistently**

## Integration with CI/CD

These scripts are designed to work with your existing GitHub Actions workflow:

1. **Tag push** → **GitHub Actions build** → **Manual release creation**
2. Scripts handle the version management and release notes
3. GitHub Actions handles the multi-platform builds
4. Manual step ensures quality control before public release

## Future Enhancements

Potential improvements:
- [ ] Automatic release creation after successful build
- [ ] Slack/Discord notifications for releases
- [ ] Automatic binary testing
- [ ] Release draft creation for review
- [ ] Integration with package managers (Homebrew, Chocolatey, etc.)