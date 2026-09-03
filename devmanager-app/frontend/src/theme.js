// Tema compartido: aplicación con persistencia y seguimiento del sistema.
import { api } from './api.js';

export const THEME_CYCLE = ['light', 'dark', 'system'];
export const SETTINGS_THEMES = ['light', 'dark', 'system'];
export const SETTINGS_STYLES = ['standard', 'brutalist', 'glassmorphism', 'retro', 'dracula'];

// Maps each style to the CSS custom property that controls its accent color.
export const STYLE_ACCENT_VAR = {
    standard:     '--accent',
    brutalist:    '--brutalist-accent',
    glassmorphism:'--accent',
    retro:        '--retro-green',
    dracula:      '--dracula-purple',
};

// Default accent colors per style (used when user resets to default).
export const STYLE_DEFAULT_ACCENT = {
    standard:     '#6366f1',
    brutalist:    '#ff006e',
    glassmorphism:'#6366f1',
    retro:        '#39ff14',
    dracula:      '#bd93f9',
};

const systemThemeQuery = window.matchMedia('(prefers-color-scheme: dark)');
let themePreference = 'dark';
let oledMode = false;

export function isValidTheme(theme) {
    return SETTINGS_THEMES.includes(theme);
}
export function isValidStyle(style) { return SETTINGS_STYLES.includes(style); }
let _styleTransitionTimer = null;
export function applyStyle(style, { persist = true } = {}) {
    const value = isValidStyle(style) ? style : 'standard';
    const root = document.documentElement;
    // Skip transition if style hasn't actually changed
    if (root.dataset.style === value) {
        if (persist) api.setSetting('style', value);
        return;
    }
    // Brief crossfade transition for smooth style switching
    root.classList.add('style-transitioning');
    clearTimeout(_styleTransitionTimer);
    _styleTransitionTimer = setTimeout(() => {
        root.classList.remove('style-transitioning');
    }, 350);
    root.dataset.style = value;
    if (persist) api.setSetting('style', value);
    // Re-apply accent override for the new style
    applyAccentOverrides();
}

export function currentTheme() {
    return document.documentElement.dataset.theme || 'dark';
}

function resolvedTheme(theme) {
    if (theme === 'system') {
        return systemThemeQuery.matches ? 'dark' : 'light';
    }
    return theme;
}

// Apply OLED mode if enabled and theme is dark
function getActualTheme(theme) {
    const resolved = resolvedTheme(theme);
    return (resolved === 'dark' && oledMode) ? 'oled' : resolved;
}

// Valida light|dark|system, aplica el tema resuelto y opcionalmente persiste.
let _themeTransitionTimer = null;
export function applyTheme(theme, { persist = true } = {}) {
    themePreference = isValidTheme(theme) ? theme : 'dark';
    const newTheme = getActualTheme(themePreference);
    const root = document.documentElement;
    // Skip transition if theme hasn't actually changed
    if (root.dataset.theme === newTheme) {
        if (persist) api.setSetting('theme', themePreference);
        return;
    }
    // Brief crossfade transition for smooth theme switching
    root.classList.add('style-transitioning');
    clearTimeout(_themeTransitionTimer);
    _themeTransitionTimer = setTimeout(() => {
        root.classList.remove('style-transitioning');
    }, 350);
    root.dataset.theme = newTheme;
    if (persist) api.setSetting('theme', themePreference);
}

// Set OLED mode and re-apply current theme
export function setOledMode(enabled, { persist = true } = {}) {
    oledMode = !!enabled;
    document.documentElement.dataset.theme = getActualTheme(themePreference);
    if (persist) api.setSetting('oled_mode', oledMode);
}

// Get current OLED mode state
export function getOledMode() {
    return oledMode;
}

systemThemeQuery.addEventListener('change', () => {
    if (themePreference === 'system') {
        document.documentElement.dataset.theme = getActualTheme(themePreference);
    }
});

// ---- Accent color overrides (Issue #48) ----

let _accentOverrides = {};
let _accentGlobal = false;
let _accentGlobalColor = '';

// Apply accent overrides to the root element via CSS custom properties.
// If global mode is on, the global color overrides every style.
// Otherwise, each style uses its own override (or the default).
export function applyAccentOverrides() {
    const root = document.documentElement;
    const currentStyle = root.dataset.style || 'standard';

    // Determine which color to use
    let accentColor = null;
    if (_accentGlobal && _accentGlobalColor) {
        accentColor = _accentGlobalColor;
    } else if (_accentOverrides[currentStyle]) {
        accentColor = _accentOverrides[currentStyle];
    }

    const cssVar = STYLE_ACCENT_VAR[currentStyle] || '--accent';
    if (accentColor) {
        root.style.setProperty(cssVar, accentColor);
    } else {
        root.style.removeProperty(cssVar);
    }
}

// Set accent overrides from settings object
export function setAccentOverrides(overrides, globalMode, globalColor) {
    _accentOverrides = overrides || {};
    _accentGlobal = !!globalMode;
    _accentGlobalColor = globalColor || '';
    applyAccentOverrides();
}

// Get current accent overrides state
export function getAccentOverrides() {
    return {
        overrides: { ..._accentOverrides },
        global: _accentGlobal,
        globalColor: _accentGlobalColor,
    };
}

// Get the current effective accent color for a given style
export function getEffectiveAccent(style) {
    if (_accentGlobal && _accentGlobalColor) return _accentGlobalColor;
    if (_accentOverrides[style]) return _accentOverrides[style];
    return STYLE_DEFAULT_ACCENT[style] || '#6366f1';
}

// Hex color validation (matches Go isValidHexColor)
export function isValidHexColor(s) {
    if (!s || s[0] !== '#' || s.length < 4) return false;
    for (let i = 1; i < s.length; i++) {
        const c = s.charCodeAt(i);
        if (!((c >= 48 && c <= 57) || (c >= 65 && c <= 70) || (c >= 97 && c <= 102))) return false;
    }
    const l = s.length;
    return l === 4 || l === 5 || l === 7 || l === 9;
}
