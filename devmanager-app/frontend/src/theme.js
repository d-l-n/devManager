// Tema compartido: aplicación con persistencia y seguimiento del sistema.
import { api } from './api.js';

export const THEME_CYCLE = ['light', 'dark'];
export const SETTINGS_THEMES = ['light', 'dark', 'oled', 'system'];

const systemThemeQuery = window.matchMedia('(prefers-color-scheme: dark)');
let themePreference = 'dark';

export function isValidTheme(theme) {
    return SETTINGS_THEMES.includes(theme);
}

export function currentTheme() {
    return document.documentElement.dataset.theme || 'dark';
}

function resolvedTheme(theme) {
    return theme === 'system' ? (systemThemeQuery.matches ? 'dark' : 'light') : theme;
}

// Valida light|dark|oled|system, aplica el tema resuelto y opcionalmente persiste.
export function applyTheme(theme, { persist = true } = {}) {
    themePreference = isValidTheme(theme) ? theme : 'dark';
    document.documentElement.dataset.theme = resolvedTheme(themePreference);
    if (persist) api.setSetting('theme', themePreference);
}

systemThemeQuery.addEventListener('change', () => {
    if (themePreference === 'system') {
        document.documentElement.dataset.theme = resolvedTheme(themePreference);
    }
});
