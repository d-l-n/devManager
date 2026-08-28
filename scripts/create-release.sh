#!/bin/bash

# Create GitHub Release Script for devManager
# Usage: ./scripts/create-release.sh [tag] [token]

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

# Get parameters
TAG=${1:-""}
TOKEN=${2:-"$GITHUB_TOKEN"}

# If no tag provided, try to get the latest tag
if [ -z "$TAG" ]; then
    TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    if [ -z "$TAG" ]; then
        print_error "No tag found. Please provide a tag as first argument."
        echo "Usage: $0 [tag] [github_token]"
        exit 1
    fi
fi

if [ -z "$TOKEN" ]; then
    print_error "GitHub token is required"
    echo "Usage: $0 [tag] [github_token]"
    echo "Or set GITHUB_TOKEN environment variable"
    exit 1
fi

print_status "Creating release for tag: $TAG"

# Check if tag exists locally
if ! git rev-parse "$TAG" >/dev/null 2>&1; then
    print_error "Tag $TAG does not exist locally"
    exit 1
fi

# Extract release notes from changelog
CHANGELOG_FILE="CHANGELOG.md"
if [ -f "$CHANGELOG_FILE" ]; then
    RELEASE_NOTES=$(sed -n "/## \[$TAG\]/,/## \[/p" "$CHANGELOG_FILE" | head -n -1)
    if [ -z "$RELEASE_NOTES" ]; then
        print_warning "No release notes found in CHANGELOG.md for tag $TAG"
        RELEASE_NOTES="Release $TAG"
    fi
else
    print_warning "CHANGELOG.md not found, using basic release notes"
    RELEASE_NOTES="Release $TAG"
fi

# Create release data
RELEASE_DATA=$(cat <<EOF
{
  "tag_name": "$TAG",
  "target_commitish": "master",
  "name": "devManager $TAG",
  "body": $(echo "$RELEASE_NOTES" | jq -Rs .),
  "draft": false,
  "prerelease": false
}
EOF
)

print_status "Creating GitHub release..."

# Create the release using GitHub API
RESPONSE=$(curl -s -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/d-l-n/devManager/releases \
  -d "$RELEASE_DATA")

# Check if release was created successfully
if echo "$RESPONSE" | jq -e '.html_url' >/dev/null 2>&1; then
    RELEASE_URL=$(echo "$RESPONSE" | jq -r '.html_url')
    print_success "Release created successfully!"
    print_status "Release URL: $RELEASE_URL"
    
    # Check if assets are being uploaded
    ASSETS_URL=$(echo "$RESPONSE" | jq -r '.assets_url')
    print_status "Assets will be uploaded by GitHub Actions to: $RELEASE_URL"
    
    echo
    print_status "Next steps:"
    echo "1. Monitor the build at: https://github.com/d-l-n/devManager/actions"
    echo "2. Wait for GitHub Actions to complete and upload binaries"
    echo "3. Verify all binaries are attached to the release"
    echo "4. Download and test the binaries"
    
else
    print_error "Failed to create release"
    echo "Response: $RESPONSE"
    exit 1
fi