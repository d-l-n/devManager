import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock api.js before importing theme
vi.mock('../api.js', () => ({
    api: {
        setSetting: vi.fn().mockResolvedValue([]),
        getSettings: vi.fn().mockResolvedValue({}),
    },
}));

import {
    SETTINGS_THEMES, SETTINGS_STYLES, STYLE_ACCENT_VAR, STYLE_DEFAULT_ACCENT,
    isValidTheme, isValidStyle, isValidHexColor,
    applyStyle, applyTheme, setOledMode, getOledMode,
    setAccentOverrides, getAccentOverrides, getEffectiveAccent,
} from '../theme.js';

// ---- Constants ----

describe('SETTINGS_THEMES', () => {
    it('contains light, dark, system', () => {
        expect(SETTINGS_THEMES).toContain('light');
        expect(SETTINGS_THEMES).toContain('dark');
        expect(SETTINGS_THEMES).toContain('system');
    });
});

describe('SETTINGS_STYLES', () => {
    it('contains all 5 styles', () => {
        expect(SETTINGS_STYLES).toEqual(['standard', 'brutalist', 'glassmorphism', 'retro', 'dracula']);
    });
});

describe('STYLE_ACCENT_VAR', () => {
    it('maps each style to a CSS custom property', () => {
        expect(STYLE_ACCENT_VAR.standard).toBe('--accent');
        expect(STYLE_ACCENT_VAR.brutalist).toBe('--brutalist-accent');
        expect(STYLE_ACCENT_VAR.glassmorphism).toBe('--accent');
        expect(STYLE_ACCENT_VAR.retro).toBe('--retro-green');
        expect(STYLE_ACCENT_VAR.dracula).toBe('--dracula-purple');
    });
});

describe('STYLE_DEFAULT_ACCENT', () => {
    it('has hex values for all styles', () => {
        for (const style of SETTINGS_STYLES) {
            expect(STYLE_DEFAULT_ACCENT[style]).toMatch(/^#[0-9a-fA-F]{6}$/);
        }
    });
});

// ---- isValidTheme / isValidStyle ----

describe('isValidTheme', () => {
    it('returns true for valid themes', () => {
        expect(isValidTheme('light')).toBe(true);
        expect(isValidTheme('dark')).toBe(true);
        expect(isValidTheme('system')).toBe(true);
    });
    it('returns false for invalid themes', () => {
        expect(isValidTheme('neon')).toBe(false);
        expect(isValidTheme('')).toBe(false);
        expect(isValidTheme('OLED')).toBe(false);
    });
});

describe('isValidStyle', () => {
    it('returns true for valid styles', () => {
        expect(isValidStyle('standard')).toBe(true);
        expect(isValidStyle('brutalist')).toBe(true);
        expect(isValidStyle('glassmorphism')).toBe(true);
        expect(isValidStyle('retro')).toBe(true);
        expect(isValidStyle('dracula')).toBe(true);
    });
    it('returns false for invalid styles', () => {
        expect(isValidStyle('minimal')).toBe(false);
        expect(isValidStyle('')).toBe(false);
    });
});

// ---- isValidHexColor ----

describe('isValidHexColor', () => {
    it('accepts valid 3-char hex', () => {
        expect(isValidHexColor('#fff')).toBe(true);
        expect(isValidHexColor('#abc')).toBe(true);
    });
    it('accepts valid 4-char hex (with alpha)', () => {
        expect(isValidHexColor('#fffa')).toBe(true);
    });
    it('accepts valid 6-char hex', () => {
        expect(isValidHexColor('#ff5500')).toBe(true);
        expect(isValidHexColor('#6366f1')).toBe(true);
    });
    it('accepts valid 8-char hex (with alpha)', () => {
        expect(isValidHexColor('#ff5500aa')).toBe(true);
    });
    it('rejects invalid hex', () => {
        expect(isValidHexColor('')).toBe(false);
        expect(isValidHexColor('#')).toBe(false);
        expect(isValidHexColor('#ff')).toBe(false);
        expect(isValidHexColor('#ffff')).toBe(true); // 4-char is valid (#RGBA)
        expect(isValidHexColor('#gggggg')).toBe(false);
        expect(isValidHexColor('red')).toBe(false);
        expect(isValidHexColor(null)).toBe(false);
        expect(isValidHexColor(undefined)).toBe(false);
    });
});

// ---- applyStyle / applyTheme (DOM-level) ----

describe('applyStyle', () => {
    beforeEach(() => {
        document.documentElement.dataset.style = '';
    });

    it('sets data-style on root element', () => {
        applyStyle('retro', { persist: false });
        expect(document.documentElement.dataset.style).toBe('retro');
    });

    it('falls back to standard for invalid style', () => {
        applyStyle('nonexistent', { persist: false });
        expect(document.documentElement.dataset.style).toBe('standard');
    });
});

describe('applyTheme', () => {
    beforeEach(() => {
        document.documentElement.dataset.theme = '';
    });

    it('sets data-theme on root element', () => {
        applyTheme('light', { persist: false });
        expect(document.documentElement.dataset.theme).toBe('light');
    });

    it('falls back to dark for invalid theme', () => {
        applyTheme('invalid', { persist: false });
        expect(document.documentElement.dataset.theme).toBe('dark');
    });
});

describe('setOledMode / getOledMode', () => {
    beforeEach(() => {
        document.documentElement.dataset.theme = '';
        setOledMode(false, { persist: false });
    });

    it('starts with OLED off', () => {
        expect(getOledMode()).toBe(false);
    });

    it('enables OLED mode', () => {
        setOledMode(true, { persist: false });
        expect(getOledMode()).toBe(true);
    });

    it('toggles theme to oled when dark + OLED enabled', () => {
        applyTheme('dark', { persist: false });
        setOledMode(true, { persist: false });
        expect(document.documentElement.dataset.theme).toBe('oled');
    });
});

// ---- Accent overrides ----

describe('accent overrides', () => {
    beforeEach(() => {
        document.documentElement.dataset.style = 'standard';
        document.documentElement.style.cssText = '';
        setAccentOverrides({}, false, '');
    });

    it('applyAccentOverrides sets CSS variable for current style', () => {
        setAccentOverrides({ standard: '#ff0000' }, false, '');
        const val = document.documentElement.style.getPropertyValue('--accent');
        expect(val).toBe('#ff0000');
    });

    it('applyAccentOverrides removes property when no override', () => {
        setAccentOverrides({}, false, '');
        const val = document.documentElement.style.getPropertyValue('--accent');
        expect(val).toBe('');
    });

    it('global mode overrides per-style', () => {
        document.documentElement.dataset.style = 'retro';
        setAccentOverrides({ retro: '#aaa' }, true, '#00ff00');
        const val = document.documentElement.style.getPropertyValue('--retro-green');
        expect(val).toBe('#00ff00');
    });

    it('getAccentOverrides returns current state', () => {
        setAccentOverrides({ standard: '#111' }, true, '#222');
        const state = getAccentOverrides();
        expect(state.overrides.standard).toBe('#111');
        expect(state.global).toBe(true);
        expect(state.globalColor).toBe('#222');
    });

    it('getEffectiveAccent returns override for current style', () => {
        setAccentOverrides({ brutalist: '#aaa' }, false, '');
        expect(getEffectiveAccent('brutalist')).toBe('#aaa');
    });

    it('getEffectiveAccent returns global color when global mode on', () => {
        setAccentOverrides({}, true, '#bbb');
        expect(getEffectiveAccent('brutalist')).toBe('#bbb');
    });

    it('getEffectiveAccent returns default when no override', () => {
        expect(getEffectiveAccent('standard')).toBe('#6366f1');
        expect(getEffectiveAccent('retro')).toBe('#39ff14');
    });
});
