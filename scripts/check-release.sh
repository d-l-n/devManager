#!/bin/bash

# Check Release Status Script for devManager
# Usage: ./scripts/check-release.sh [tag]

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

# Get tag parameter
TAG=${1:-""}

# If no tag provided, get the latest tag
if [ -z "$TAG" ]; then
    TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    if [ -z "$TAG" ]; then
        print_error "No tag found. Please provide a tag as first argument."
        echo "Usage: $0 [tag]"
        exit 1
    fi
fi

print_status "Checking release status for tag: $TAG"

# Check if tag exists locally
if ! git rev-parse "$TAG" >/dev/null 2>&1; then
    print_error "Tag $TAG does not exist locally"
    exit 1
fi

# Check if tag exists on remote
print_status "Checking remote tag..."
if ! git ls-remote --tags origin "$TAG" | grep -q "$TAG"; then
    print_warning "Tag $TAG not found on remote. Push it with: git push origin $TAG"
else
    print_success "Tag $TAG found on remote"
fi

# Check GitHub Actions workflow status
print_status "Checking GitHub Actions workflow status..."

# Get the commit hash for the tag
COMMIT_HASH=$(git rev-list -n 1 "$TAG")

# Check GitHub Actions API (requires gh CLI or token)
if command -v gh >/dev/null 2>&1; then
    print_status "Using GitHub CLI to check workflow status..."
    
    # Get workflows for the specific commit
    WORKFLOWS=$(gh run list --commit "$COMMIT_HASH" --limit 5 --json databaseId,status,conclusion,createdAt,displayTitle 2>/dev/null || echo "")
    
    if [ -n "$WORKFLOWS" ]; then
        echo "$WORKFLOWS" | jq -r '.[] | "  • \(.displayTitle) - \(.status) (\(.conclusion // "running"))"'
        
        # Check if build workflow completed successfully
        BUILD_STATUS=$(echo "$WORKFLOWS" | jq -r '.[] | select(.displayTitle | contains("Build")) | .conclusion')
        
        if [ "$BUILD_STATUS" = "success" ]; then
            print_success "Build workflow completed successfully"
        elif [ "$BUILD_STATUS" = "failure" ]; then
            print_error "Build workflow failed"
        elif [ -z "$BUILD_STATUS" ]; then
            print_warning "Build workflow not found or still running"
        else
            print_status "Build workflow status: $BUILD_STATUS"
        fi
    else
        print_warning "Could not retrieve workflow information"
    fi
else
    print_warning "GitHub CLI not found. Install it to check workflow status."
    print_status "Manual check: https://github.com/d-l-n/devManager/actions"
fi

# Check if release exists on GitHub
print_status "Checking GitHub release..."

if command -v gh >/dev/null 2>&1; then
    RELEASE_INFO=$(gh release view "$TAG" 2>/dev/null || echo "")
    
    if [ -n "$RELEASE_INFO" ]; then
        print_success "Release found on GitHub"
        
        # Get release assets
        ASSETS=$(echo "$RELEASE_INFO" | jq -r '.assets[] | "  • \(.name) (\(.size // "unknown") bytes)"' 2>/dev/null || echo "")
        
        if [ -n "$ASSETS" ]; then
            print_status "Release assets:"
            echo "$ASSETS"
            
            # Count assets
            ASSET_COUNT=$(echo "$RELEASE_INFO" | jq -r '.assets | length' 2>/dev/null || echo "0")
            print_status "Total assets: $ASSET_COUNT"
            
            if [ "$ASSET_COUNT" -eq 6 ]; then
                print_success "All 6 platform binaries are attached"
            else
                print_warning "Expected 6 assets, found $ASSET_COUNT"
            fi
        else
            print_warning "No assets found in release"
        fi
    else
        print_warning "Release not found on GitHub"
        print_status "Create it with: gh release create $TAG --title \"devManager $TAG\" --notes \"Release $TAG\""
    fi
else
    print_warning "GitHub CLI not found. Manual check: https://github.com/d-l-n/devManager/releases"
fi

# Check local files
print_status "Checking local version consistency..."

# Get version from files
HTML_VERSION=$(grep -o 'v[0-9]*\.[0-9]*\.[0-9]*' devmanager-app/frontend/index.html 2>/dev/null || echo "not found")
PACKAGE_VERSION=$(grep -o '"version": "[^"]*"' devmanager-app/frontend/package.json 2>/dev/null | cut -d'"' -f4 || echo "not found")
WAILS_VERSION=$(grep -o '"productVersion": "[^"]*"' devmanager-app/wails.json 2>/dev/null | cut -d'"' -f4 || echo "not found")

echo "  • HTML version: $HTML_VERSION"
echo "  • Package version: $PACKAGE_VERSION"
echo "  • Wails version: $WAILS_VERSION"

# Check if all versions match
EXPECTED_VERSION="${TAG#v}"  # Remove 'v' prefix

if [ "$HTML_VERSION" = "$TAG" ] && [ "$PACKAGE_VERSION" = "$EXPECTED_VERSION" ] && [ "$WAILS_VERSION" = "$EXPECTED_VERSION" ]; then
    print_success "All version files are consistent"
else
    print_warning "Version files are inconsistent"
    print_status "Expected: $TAG / $EXPECTED_VERSION"
fi

echo
print_status "Release check completed for $TAG"
echo
print_status "Quick links:"
echo "  • GitHub Actions: https://github.com/d-l-n/devManager/actions"
echo "  • Releases: https://github.com/d-l-n/devManager/releases"
echo "  • Tag: https://github.com/d-l-n/devManager/releases/tag/$TAG"