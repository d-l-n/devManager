# Style, dialogs, and sidebar hierarchy

## Goal

Let users choose an interface style independently of the existing color theme, replace browser-native confirmation and alert popups with app-native dialogs, and clarify the app identity in the sidebar.

## Preferences

Persist a new `style` setting alongside `theme`, `monitor_polling`, and `toasts_enabled`.

- `standard` is the default and keeps the existing appearance.
- `brutalist` changes the visual language only.
- Existing settings files without `style` resolve to `standard`.

The frontend applies two attributes to the document root:

- `data-theme` continues to resolve `light`, `dark`, `oled`, or the current system color scheme.
- `data-style` applies `standard` or `brutalist` independently.

The brutalist layer uses the active theme variables for color. It applies sharp or near-sharp corners, solid borders, offset shadows, stronger label contrast, and direct action treatment across panels, forms, context menus, buttons, and dialogs. It does not change product behavior or the selected color theme.

## Settings

Add an “Interface style” group beneath Appearance. Its radio choices are Standard and Brutalist, each with a concise description. Selecting a style applies and persists it immediately, matching the current behavior for color themes.

## App-native dialogs

Create one reusable dialog module with two modes:

- Confirmation: title, explanatory message, Cancel, and a named confirm action. The confirm button is destructive when the action cannot be undone.
- Alert: title, message, and a Close acknowledgement.

Confirmation replaces the native browser calls for quitting, restarting, removing projects, deleting backlog items, opening Trace Viewer, and terminating process trees. Form validation and save failures use the alert mode.

The dialog supports Escape and backdrop click as cancellation, returns a promise to the caller, restores focus to the trigger on close, and gives Cancel initial focus for destructive actions.

## Sidebar header

Make the sidebar header a two-row identity block. The first row contains the logo, product name, and Add Project action. The project count appears directly below it as supporting context. This keeps the name and logo prominent while the count remains easy to scan.

## Verification

- Extend Go settings tests to cover valid, invalid, defaulted, and persisted style values.
- Add frontend coverage or focused checks for the dialog promise result and style application.
- Build the Vite frontend and run the Go suite.
- Manually inspect standard and brutalist styles under light, dark, and OLED colors, plus the system-color resolution path.
