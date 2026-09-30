# Changelog

All notable changes to devManager will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### 🚀 New Features
- **Advanced Notification System (#66)**: external platforms (Slack, Discord, Telegram Bot API, Microsoft Teams/Power Automate) with per-platform min-priority + event filters, alert rules (`all` or per-event with priority floor), 10/min per-platform rate limiting, retries with linear backoff (0-3), delivery history (last 200, `notify-history.jsonl`) and settings UI with test-fire button; global gate via `external_notifications` (default on) and lifecycle hooks on server start/stop/error
- **Integrated Development Tools (#68)**: new Tools tab with sandboxed file browser (paths clamped to project root), file editor (1 MB cap, dirty tracking), file search by name + content (node_modules/.git/dist excluded, 500 results cap), persistent snippets (`snippets.json`) runnable as scripts, and a Ctrl+K command palette mixing quick actions with saved snippets
- **Touch Support & Internationalization (#69)**: i18n with English/Spanish/Arabic (RTL) via `language` setting, locale-aware date/number/bytes formatting, `data-i18n` DOM translation pass, touch mode (`touch_optimized`) with 44px targets, 16px inputs to avoid iOS zoom and swipe-to-switch-tabs navigation (direction-aware under RTL)

- **Workflows, Webhooks & CI-CD Integration (#65, MVP)**: per-project workflows (`manual`/`schedule`/`event` triggers, `command`/`notify`/`webhook`/`ci_trigger` steps with retry + timeout), sequential executor with persistent run history (cap 100/workflow), 60s schedule ticker with running-dedupe, event fan-out (`server_started`/`server_stopped`/`tests_finished`/`webhook_received`), outgoing webhooks (`POST {event,project,at,payload}`), incoming localhost listener (`POST /hook/{workflowId}`, default `127.0.0.1:9876`, settings `workflow_webhook_port`/`workflow_webhook_enabled`), CI/CD per project (GitHub dispatch, GitLab pipeline, Jenkins build; tokens via `env:NAME`, masked in reads) and a Workflows tab with inline editor, runs, webhooks and CI panels

## [2.1.0] - 2026-09-07

### 🚀 New Features
- **Built-in GitHub Releases Updater (#58)**: version check against latest GitHub release, update info shown in settings
- **Dependency Inspector (#61)**: new deps package (manifest parse + manager detect), outdated detection + security audit, deps dashboard tab with outdated/audit views
- **Git Core Tools (#63)**: diff, branches, and tags tools
- **Per-Project User Creation**: auto-detect command per project, tab visibility control
- **Per-Project Tab Customization**: hide/order tabs per project
- **Per-Style and Global Accent Color Customization (#48)**: custom accent color per style + global override
- **Unit Test Suite (#42)**: vitest setup with tests for theme and toast
- **Themed Monitor Screen + Custom Checkboxes (#51 #52)**

### 🐛 Bug Fixes
- Fix issues #43-#56: validation, rate limiting, duplicate project handling
- Build popups rewritten using temp `.ps1` files (no more truncated popup blocks)
- Fix retro accent colors, settings reactivity, build popup improvements
- Fix stale retro accent test asserts — `STYLE_ACCENT_VAR.retro` is `--retro-accent`
- Hide child consoles, lazy-load deps, popup tweaks
- Use vite build for debug frontend (no `build:dev` script)
- Remove flaky ANSI colors from `build.bat`

### ♻️ Refactoring & Cleanup
- Split `app.go` god object into 9 thematic files (#41)
- Unit tests for `app.go` pure functions and theme detection (#39 #40)

### 🔧 Technical Changes
- Fix CI matrix expansion: `fromJson` must use `matrix.include` (direct expression no longer evaluated by GitHub Actions)
- `-skipbindings` on headless Linux builds — temp binding-generation binary SIGSEGVs in xvfb/dbus session
- Inject `-X main.Version=<tag>` at build time so release binaries report the real version
- Update READMEs — recent features, real repo structure, `projects.json` schema

### 📦 Downloads
This release includes binaries for:
- Windows (amd64, arm64)
- macOS (amd64, arm64)
- Linux (amd64, arm64)

---

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