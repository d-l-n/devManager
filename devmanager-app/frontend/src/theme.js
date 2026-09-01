// Tema compartido: aplicación con persistencia y seguimiento del sistema.
import { api } from './api.js';

export const THEME_CYCLE = ['light', 'dark', 'system'];
export const SETTINGS_THEMES = ['light', 'dark', 'system'];
export const SETTINGS_STYLES = ['standard', 'brutalist', 'glassmorphism', 'retro', 'dracula'];

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
