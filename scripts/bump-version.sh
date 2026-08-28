#!/bin/bash

# Version Bumping Script for devManager
# Usage: ./scripts/bump-version.sh [patch|minor|major] [description]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're in the right directory
if [ ! -f "devmanager-app/wails.json" ]; then
    print_error "Please run this script from the devManager root directory"
    exit 1
fi

# Get current version
CURRENT_VERSION=$(grep -o '"productVersion": "[^"]*"' devmanager-app/wails.json | cut -d'"' -f4)
if [ -z "$CURRENT_VERSION" ]; then
    print_error "Could not determine current version"
    exit 1
fi

print_status "Current version: $CURRENT_VERSION"

# Parse version bump type
BUMP_TYPE=${1:-"patch"}
DESCRIPTION=${2:-"Automated version bump"}

# Validate bump type
case $BUMP_TYPE in
    patch|minor|major)
        ;;
    *)
        print_error "Invalid bump type. Use: patch, minor, or major"
        exit 1
        ;;
esac

# Calculate new version
IFS='.' read -ra VERSION_PARTS <<< "$CURRENT_VERSION"
MAJOR=${VERSION_PARTS[0]}
MINOR=${VERSION_PARTS[1]}
PATCH=${VERSION_PARTS[2]}

case $BUMP_TYPE in
    patch)
        NEW_PATCH=$((PATCH + 1))
        NEW_VERSION="$MAJOR.$MINOR.$NEW_PATCH"
        ;;
    minor)
        NEW_MINOR=$((MINOR + 1))
        NEW_VERSION="$MAJOR.$NEW_MINOR.0"
        ;;
    major)
        NEW_MAJOR=$((MAJOR + 1))
        NEW_VERSION="$NEW_MAJOR.0.0"
        ;;
esac

print_status "New version will be: $NEW_VERSION"

# Confirm with user
echo
read -p "Do you want to continue with version bump to $NEW_VERSION? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    print_warning "Version bump cancelled"
    exit 0
fi

# Function to update version in a file
update_version() {
    local file=$1
    local old_pattern=$2
    local new_version=$3
    
    if [ -f "$file" ]; then
        if grep -q "$old_pattern" "$file"; then
            sed -i.bak "s/$old_pattern/$old_pattern$new_version/g" "$file"
            rm "$file.bak"
            print_success "Updated $file"
        else
            print_warning "Pattern not found in $file"
        fi
    else
        print_warning "File not found: $file"
    fi
}

# Update version in all files
print_status "Updating version in all files..."

# Update frontend/index.html
update_version "devmanager-app/frontend/index.html" 'v[0-9]*\.[0-9]*\.[0-9]*' "$NEW_VERSION"

# Update frontend/package.json
update_version "devmanager-app/frontend/package.json" '"version": "[^"]*"' "\"$NEW_VERSION\""

# Update wails.json
update_version "devmanager-app/wails.json" '"productVersion": "[^"]*"' "\"$NEW_VERSION\""

# Update changelog
print_status "Updating changelog..."
CHANGELOG_ENTRY="## [$NEW_VERSION] - $(date +%Y-%m-%d)

### 🔄 Version Bump
- Version bumped from $CURRENT_VERSION to $NEW_VERSION
- $DESCRIPTION

### 📦 Downloads
This release includes binaries for:
- Windows (amd64, arm64)
- macOS (amd64, arm64)
- Linux (amd64, arm64)

---

"

# Insert new entry at the beginning of changelog (after the header)
if [ -f "CHANGELOG.md" ]; then
    temp_file=$(mktemp)
    head -n 4 CHANGELOG.md > "$temp_file"
    echo "$CHANGELOG_ENTRY" >> "$temp_file"
    tail -n +5 CHANGELOG.md >> "$temp_file"
    mv "$temp_file" CHANGELOG.md
    print_success "Updated CHANGELOG.md"
else
    print_warning "CHANGELOG.md not found"
fi

# Create release notes file
RELEASE_FILE="RELEASE_$NEW_VERSION.md"
print_status "Creating release notes: $RELEASE_FILE"

cat > "$RELEASE_FILE" << EOF
# Release Instructions for devManager $NEW_VERSION

## Automated Release Process

The release has been tagged and pushed to GitHub. Here's what happens next:

### 1. GitHub Actions Build Triggered ✅
- The tag \`$NEW_VERSION\` has been pushed to GitHub
- This automatically triggers the \`build-multiplatform.yml\` workflow
- The workflow will build binaries for all platforms:
  - Windows (amd64, arm64)
  - macOS (amd64, arm64)
  - Linux (amd64, arm64)

### 2. Monitor the Build
You can monitor the build progress at:
\`\`\`
https://github.com/d-l-n/devManager/actions
\`\`\`

Look for the workflow named "Build Multi-Platform" triggered by the $NEW_VERSION tag.

### 3. Create the GitHub Release

Once the builds complete successfully:

#### Option A: Using GitHub Web UI (Recommended)
1. Go to: https://github.com/d-l-n/devManager/releases/new
2. Select tag: \`$NEW_VERSION\`
3. Target: \`master\`
4. Release title: \`devManager $NEW_VERSION\`
5. Description: Use the release notes from CHANGELOG.md
6. Check "This is a pre-release" ❌ (leave unchecked)
7. Click "Publish release"

#### Option B: Using GitHub CLI
\`\`\`bash
gh release create $NEW_VERSION \\
  --title "devManager $NEW_VERSION" \\
  --notes "\$(sed -n "/## \[$NEW_VERSION\]/,/## \[/p" CHANGELOG.md | head -n -1)"
\`\`\`

## Release Notes

\`\`\`
$(sed -n "/## \[$NEW_VERSION\]/,/## \[/p" CHANGELOG.md | head -n -1)
\`\`\`

## Verification Steps

After the release is created:

1. [ ] Check that all 6 platform binaries are attached
2. [ ] Download and test Windows binary
3. [ ] Verify macOS binary (if possible)
4. [ ] Check Linux binary (if possible)
5. [ ] Confirm release notes are displayed correctly

## Next Steps

1. Monitor the GitHub Actions build
2. Create the release once builds complete
3. Test the downloaded binaries
4. Announce the release if needed

---

**Note**: The build process may take 10-15 minutes to complete as it builds for all platforms.
EOF

print_success "Created release notes: $RELEASE_FILE"

# Git operations
print_status "Committing changes..."

# Add all changed files
git add -A

# Commit changes
git commit -m "feat: bump version to $NEW_VERSION

- Update version from $CURRENT_VERSION to $NEW_VERSION
- Update changelog with release notes
- Create release documentation
- $DESCRIPTION

Co-Authored-By: CODA <coda@globant.com>"

# Create tag
print_status "Creating git tag..."
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION - $DESCRIPTION"

print_success "Version bump completed successfully!"
echo
print_status "Next steps:"
echo "1. Review the changes: git log --oneline -1"
echo "2. Push to GitHub: git push origin master && git push origin $NEW_VERSION"
echo "3. Monitor the build at: https://github.com/d-l-n/devManager/actions"
echo "4. Create the GitHub release once builds complete"
echo
print_status "Release notes prepared in: $RELEASE_FILE"