# Changelog

All notable changes to devManager will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.1] - 2026-09-01

### 🚀 New Features
- **3 New UI Themes**: Added glassmorphism (frosted glass), retro (CRT neon glow), and dracula (purple/pink palette) themes
- **Style Previews**: Visual thumbnails in settings for theme selection
- **Smooth Transitions**: Crossfade animations when switching styles/themes
- **ARIA Accessibility**: Added ARIA roles to tabs, project list, context menu, toast container, and filter chips

### 🐛 Bug Fixes
- Remove obsolete `-debug` flag from wails build commands across all platforms
- Fix settings header cleanup, style section icon, retro button alignment
- Fix duplicate key collisions in keyboard shortcuts
- Reject concurrent server starts via starting flag to prevent double launch
- Fix port flag regex to match compact `-p<port>` format
- Fix mojibake bytes in port redirect log line
- Bound proc cache with LRU eviction for system monitor
- Don't let Django override detected Node server config
- Suppress toasts for unhandled global JS errors

### ♻️ Refactoring & Cleanup
- Replace ~130 lines of runtime tray icon geometry code with static `.ico` embed
- Delete 8 dead frontend files and 2 dead Go packages (~700 lines removed)
- Remove `blang/semver` dependency
- Batch `updateDots()` calls with `Promise.all` and reduce polling interval
- Remove AI agent/skills config from repo (`.agents/`, `.claude/`, `skills-lock.json`)

### 🔧 Technical Changes
- Update `libwebkit2gtk-4.0-dev` to `4.1-dev` in CI (Ubuntu 24.04 compat)
- Add `shell: bash` to Windows CI steps (PowerShell lacks `ls`)
- Update Go version to 1.25 in all workflows
- Sync version references across `wails.json`, `package.json`, and `index.html`

### 📦 Downloads
This release includes binaries for:
- Windows (amd64, arm64)
- macOS (amd64, arm64)
- Linux (amd64, arm64)

---

## [1.0.1] - 2025-01-28

### 🎨 UI/UX Improvements
- **Enhanced Brutalist Style**: Improved brutalist theme implementation
- **Consistent Button Styling**: All log panel buttons now have uniform appearance
- **Better Hover Effects**: Buttons show elevation and color only on hover
- **Active State Refinement**: Active buttons maintain consistent base appearance

### 🐛 Bug Fixes
- Fixed inconsistent button styling in logs panel
- Resolved brutalist theme hover state issues
- Improved button active state visual feedback

### 🔧 Technical Changes
- Updated CSS for better brutalist theme consistency
- Enhanced button state management in brutalist-enhanced.css
- Improved visual hierarchy in log toolbar

### 📦 Downloads
This release includes binaries for:
- Windows (amd64, arm64)
- macOS (amd64, arm64)
- Linux (amd64, arm64)

---

## [1.0.0] - 2025-01-28

### 🚀 Initial Release
- Complete project management system
- Multi-platform support (Windows, macOS, Linux)
- Real-time log monitoring
- Playwright integration
- Git repository management
- Script execution panel
- Evidence tracking
- Backlog management
- System monitoring
- Brutalist theme support
- Dark/Light/OLED themes
- Cross-platform build system

### 📋 Core Features
- **Project Management**: Add, edit, organize development projects
- **Server Control**: Start, stop, restart development servers
- **Log Monitoring**: Real-time log viewing with filtering and tools
- **Playwright Testing**: Integrated test runner and reporting
- **Git Integration**: Repository status and common operations
- **Script Execution**: Run custom commands and package.json scripts
- **Evidence Collection**: Screenshot and test artifact management
- **Backlog Tracking**: Task and issue management
- **System Monitor**: Port management and process monitoring

### 🎨 Themes
- **Brutalist**: Bold, high-contrast design with hard shadows
- **Dark**: Standard dark theme
- **Light**: Clean light theme
- **OLED**: Pure black theme for OLED displays

### 🛠️ Technical Stack
- **Backend**: Go with Wails v2.15.0
- **Frontend**: Vanilla JavaScript with Vite
- **Build System**: GitHub Actions multi-platform builds
- **UI**: Custom CSS with component-based architecture