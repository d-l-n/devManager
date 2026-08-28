// Tema compartido: ciclo y aplicaci├│n con persistencia (Task 7).
import { api } from './api.js';

export const THEME_CYCLE = ['light', 'dark', 'oled'];

export function isValidTheme(theme) {
    return THEME_CYCLE.includes(theme);
}

export function currentTheme() {
    return document.documentElement.dataset.theme || 'dark';
}

// Valida Ôêê light|dark|oled, aplica data-theme y persiste.
// El eco settings:changed de Go solo actualiza estado local (sin bucle).
export function applyTheme(theme) {
    const t = isValidTheme(theme) ? theme : 'dark';
    document.documentElement.dataset.theme = t;
    api.setSetting('theme', t);
}
