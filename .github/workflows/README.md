# GitHub Actions Workflows

This directory contains the GitHub Actions workflows for the devManager project.

## Workflows

### 1. `build-multiplatform.yml`
**Trigger**: Push to tags matching `v*` (e.g., `v1.0.0`, `v2.1.3`) or manual dispatch

**Platforms**:
- Windows (amd64, arm64)
- macOS (amd64, arm64) 
- Linux (amd64, arm64)

**Features**:
- Builds release versions of the Wails application
- Creates GitHub releases with all platform artifacts
- Generates release notes automatically
- Artifacts are retained for 30 days

### 2. `build-dev.yml`
**Trigger**: Push to `main`/`develop` branches or pull requests to `main`

**Platforms**:
- Windows (amd64)
- macOS (amd64)
- Linux (amd64)

**Features**:
- Builds development/debug versions
- Faster builds with debug symbols
- Artifacts retained for 7 days
- No GitHub release creation

## Build Process

### Prerequisites
- Go 1.25+
- Node.js 20+
- Wails v2.15.0
- Platform-specific dependencies:
  - Ubuntu: `libgtk-3-dev`, `libwebkit2gtk-4.0-dev`
  - macOS: Xcode command line tools
  - Windows: No additional dependencies required

### Build Steps
1. Checkout repository
2. Set up Go and Node.js environments
3. Install Wails CLI
4. Install platform-specific dependencies
5. Install frontend npm dependencies
6. Build Wails application for target platform/architecture
7. Package and upload artifacts

### Artifact Naming
- Release builds: `devmanager-{platform}-{arch}.zip`
- Development builds: `devmanager-dev-{platform}-{arch}`

### Manual Builds
You can also trigger builds manually:
1. Go to the Actions tab in your GitHub repository
2. Select "Build Multi-Platform" workflow
3. Click "Run workflow"
4. Optionally specify a version number

## Local Development

To build locally:

```bash
# Install Wails
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0

# Build for current platform
cd devmanager-app
wails build

# Build for specific platform
wails build -platform windows -arch amd64
wails build -platform darwin -arch arm64
wails build -platform linux -arch amd64
```

## Release Process

1. Update version in `wails.json` if needed
2. Commit and push changes
3. Create and push a tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
4. GitHub Actions will automatically build and create a release

## Troubleshooting

### Build Failures
- Check that all dependencies are properly installed
- Verify Node.js and Go versions match requirements
- Ensure frontend builds successfully with `npm run build`

### Platform-Specific Issues
- **Windows**: Make sure Windows SDK is installed
- **macOS**: Ensure Xcode command line tools are installed
- **Linux**: Install GTK and WebKit development packages

### Cross-Compilation
- Wails handles most cross-compilation automatically
- CGO is enabled for all builds
- Some platforms may require additional toolchains