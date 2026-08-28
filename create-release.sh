#!/bin/bash

# Create GitHub Release Script
# Usage: ./create-release.sh <tag> <token>

TAG=${1:-"v1.0.5"}
TOKEN=${2:-"$GITHUB_TOKEN"}

if [ -z "$TOKEN" ]; then
    echo "Error: GitHub token is required"
    echo "Usage: $0 <tag> <github_token>"
    exit 1
fi

RELEASE_DATA=$(cat <<EOF
{
  "tag_name": "$TAG",
  "target_commitish": "master",
  "name": "devManager $TAG",
  "body": "## devManager $TAG\n\n### 🚀 Features\n- Improved build system stability\n- Enhanced cross-platform compatibility\n\n### 🐛 Bug Fixes\n- Fixed GitHub Actions build errors\n- Resolved Go version compatibility issues\n- Fixed artifact preparation for all platforms\n\n### 🔧 Technical Changes\n- Updated Go version to 1.25 for Wails v2.15.0 compatibility\n- Removed problematic dependency overrides\n- Improved build verification and error handling\n- Enhanced cross-platform build process\n\n### 📦 Downloads\nThis release includes binaries for:\n- Windows (amd64, arm64)\n- macOS (amd64, arm4)\n- Linux (amd64, arm64)\n\n### 🙏 Thanks\nSpecial thanks to the automated build system improvements!",
  "draft": false,
  "prerelease": false
}
EOF
)

echo "Creating release $TAG..."

curl -X POST \
  -H "Authorization: token $TOKEN" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/d-l-n/devManager/releases \
  -d "$RELEASE_DATA"

echo -e "\n\nRelease creation initiated!"
echo "The GitHub Actions workflow will build and attach the binaries automatically."