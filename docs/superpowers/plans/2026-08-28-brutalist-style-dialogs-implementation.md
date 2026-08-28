# Brutalist Style, Dialogs, and Sidebar Hierarchy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a selectable brutalist interface style, app-native confirmation and alert dialogs, and a prominent sidebar identity header.

**Architecture:** Persist `style` separately from `theme`. The frontend applies `data-style` independently of `data-theme`; a single promise-based module supplies confirmations and alerts to every caller.

**Tech Stack:** Go 1.25, Wails v2, vanilla JavaScript, CSS, Vite 5.

**Spec:** `docs/superpowers/specs/2026-08-28-brutalist-style-dialogs-design.md`

## Global Constraints

- `data-theme` resolves `light`, `dark`, `oled`, and `system`; `data-style` resolves `standard` and `brutalist`.
- Missing `style` defaults to `standard`.
- Brutalist CSS uses the current theme variables and introduces no fixed palette.
- Escape and backdrop cancel confirmations; destructive ones focus Cancel.
- Do not stage `projects.json`.

---

### Task 1: Persist and apply interface style

**Files:** `internal/config/settings.go`, `internal/config/settings_test.go`, `app.go`, `frontend/src/theme.js`, `frontend/src/dialogs/settings.js`.

**Produces:** `Settings.Style`, backend key `style`, and `applyStyle(style, { persist })`.

- [ ] Write the failing configuration test.

```go
func TestStyleDefaultsAndSanitizes(t *testing.T) {
    path := filepath.Join(t.TempDir(), "settings.json")
    if got := LoadSettings(path); got.Style != "standard" { t.Fatal(got.Style) }
    os.WriteFile(path, []byte(`{"theme":"dark","style":"rounded"}`), 0o644)
    if got := LoadSettings(path); got.Style != "standard" { t.Fatal(got.Style) }
}
```

- [ ] Run `go test ./internal/config -run TestStyleDefaultsAndSanitizes -count=1`; expect failure because `Style` is absent.
- [ ] Add `Style string 'json:"style"'`, `validStyle`, defaults/sanitization, and `App.SetSetting("style", ...)` accepting `standard|brutalist`.
- [ ] Implement frontend style application.

```js
export const SETTINGS_STYLES = ['standard', 'brutalist'];
export function applyStyle(style, { persist = true } = {}) {
  const value = SETTINGS_STYLES.includes(style) ? style : 'standard';
  document.documentElement.dataset.style = value;
  if (persist) api.setSetting('style', value);
}
```

- [ ] Add Standard and Brutalist radios with descriptions in Settings; initialize and synchronize them through `settings:changed`.
- [ ] Run `go test ./internal/config -count=1`; expect pass. Commit the preference layer.

### Task 2: Add app-native message dialog

**Files:** create `frontend/src/dialogs/message.js`; modify `frontend/src/theme.css`, `frontend/src/main.js`.

**Produces:** `mountMessageDialog()` with `confirm(options): Promise<boolean>`, `alert(options): Promise<void>`, and `getElement()`.

- [ ] Add a failing DOM fixture that calls `confirm`, clicks `[data-message-cancel]`, and asserts its promise resolves `false`.
- [ ] Build before creating `message.js`; expect an unresolved-import failure.
- [ ] Implement a semantic overlay and card. `confirm` accepts `{ title, message, confirmLabel, destructive, trigger }`; `alert` accepts `{ title, message, trigger }`.
- [ ] On open, store the trigger/focused element. Backdrop/Escape resolve confirmations `false`; destructive confirmation focuses Cancel; close restores focus.
- [ ] Mount one dialog at app startup and set `ctx.messageDialog` before mounting panels.
- [ ] Build with Vite; expect success. Commit dialog infrastructure.

### Task 3: Migrate all browser dialogs

**Files:** `frontend/src/main.js`, `frontend/src/dialogs/project.js`, `frontend/src/dialogs/backlog-item.js`, `frontend/src/panels/backlog.js`, `frontend/src/panels/evidence.js`, `frontend/src/panels/monitor.js`.

**Consumes:** `ctx.messageDialog`.

- [ ] Replace native confirmations with awaited calls. Use title/action pairs: Quit app/Quit; Restart app/Restart; Remove project/Remove; Delete backlog item/Delete; Open Trace Viewer/Open; Terminate process/Terminate.

```js
const ok = await ctx.messageDialog.confirm({
  title: 'Remove project',
  message: `Remove “${project.name}” from the manager? Local files will not be deleted.`,
  confirmLabel: 'Remove', destructive: true, trigger: event.currentTarget,
});
if (!ok) return;
```

- [ ] Add `removeProjectFlow(index, trigger)` to share project removal between context-menu and Delete key.
- [ ] Replace project validation/save errors and backlog save errors with `await messageDialog.alert({ title, message, trigger })`.
- [ ] Run `rg -n "\b(confirm|alert)\(" devmanager-app/frontend/src`; expect no results. Build Vite and manually test Cancel and confirm for the six flows. Commit the migration.

### Task 4: Apply brutalist layer and sidebar hierarchy

**Files:** `frontend/index.html`, `frontend/src/theme.css`.

**Consumes:** `data-style="brutalist"`.

- [ ] Reorganize the header into `.sidebar-brand-row` containing logo/name and put `#project-count` below it; keep Add Project as its separate action.
- [ ] Add style-scoped CSS using only variables.

```css
:root[data-style="brutalist"] .btn,
:root[data-style="brutalist"] .icon-btn,
:root[data-style="brutalist"] .settings-card,
:root[data-style="brutalist"] .dialog-card,
:root[data-style="brutalist"] .message-card { border-radius: 0; border: 2px solid var(--border); }
:root[data-style="brutalist"] .message-card,
:root[data-style="brutalist"] .settings-card { box-shadow: 6px 6px 0 var(--border); }
```

- [ ] Extend the same scoped treatment to inputs, tabs, project selections, context menus, and cards.
- [ ] Run Vite build, `go test ./...`, and `node C:\Users\dylan\.agents\skills\impeccable\scripts\detect.mjs --json devmanager-app/frontend/index.html devmanager-app/frontend/src/theme.css devmanager-app/frontend/src/dialogs/message.js`.
- [ ] Manually inspect standard/brutalist under light, dark, OLED, and system. Commit the visual layer.
