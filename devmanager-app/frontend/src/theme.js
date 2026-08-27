// Tema compartido: ciclo y aplicación con persistencia (Task 7).
import { api } from './api.js';

export const THEME_CYCLE = ['light', 'dark', 'oled', 'auto'];

export function isValidTheme(theme) {
    return THEME_CYCLE.includes(theme);
}

export function currentTheme() {
    return document.documentElement.dataset.theme || 'dark';
}

// Valida ∈ light|dark|oled|auto, aplica data-theme y persiste.
// El eco settings:changed de Go solo actualiza estado local (sin bucle).
export async function applyTheme(theme) {
    const t = isValidTheme(theme) ? theme : 'dark';
    
    // Si es auto, obtenemos el tema efectivo del sistema
    let effectiveTheme = t;
    if (t === 'auto') {
        try {
            effectiveTheme = await api.getEffectiveTheme();
        } catch (error) {
            console.warn('Error getting effective theme:', error);
            effectiveTheme = 'dark'; // fallback
        }
    }
    
    document.documentElement.dataset.theme = effectiveTheme;
    api.setSetting('theme', t);
}
