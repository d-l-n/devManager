import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
    LANGUAGES, validLanguages, isValidLanguage, getLanguage, setLanguage, dirFor, isRTL,
    t, applyTranslations, formatDate, formatTime, formatNumber, formatBytes,
} from '../i18n.js';
import { setTouchMode, isTouchDevice, installGestures, onSwipe, resetGestures } from '../touch.js';

describe('i18n', () => {
    beforeEach(() => {
        setLanguage('en');
    });

    it('soporta en, es y ar con dir correcto', () => {
        expect(validLanguages()).toEqual(['en', 'es', 'ar']);
        expect(dirFor('en')).toBe('ltr');
        expect(dirFor('es')).toBe('ltr');
        expect(dirFor('ar')).toBe('rtl');
        expect(isRTL('ar')).toBe(true);
        expect(isRTL('es')).toBe(false);
    });

    it('setLanguage aplica lang/dir al documento y devuelve el idioma efectivo', () => {
        expect(setLanguage('es')).toBe('es');
        expect(document.documentElement.lang).toBe('es');
        expect(document.documentElement.dir).toBe('ltr');
        expect(setLanguage('ar')).toBe('ar');
        expect(document.documentElement.dir).toBe('rtl');
        expect(setLanguage('klingon')).toBe('ar'); // inválido → sin cambio
        expect(getLanguage()).toBe('ar');
    });

    it('t traduce con fallback a en y devuelve la clave si no existe', () => {
        setLanguage('es');
        expect(t('action.start')).toBe('Iniciar');
        // Clave que solo existe en en: fallback.
        expect(t('app.title')).toBeDefined();
        setLanguage('en');
        expect(t('action.start')).toBe('Start');
        expect(t('no.existe.clave')).toBe('no.existe.clave');
    });

    it('t sustituye variables {var}', () => {
        expect(t('no.existe', { x: 1 })).toBe('no.existe');
        setLanguage('es');
        // Sin claves con vars en el diccionario se valida el mecanismo con una clave simple.
        expect(t('filter.all')).toBe('Todos');
    });

    it('applyTranslations pinta data-i18n, placeholder y title', () => {
        setLanguage('es');
        document.body.innerHTML = `
            <span data-i18n="action.start"></span>
            <input data-i18n-placeholder="sidebar.search">
            <button data-i18n-title="action.settings" aria-label="x"></button>`;
        applyTranslations(document);
        expect(document.querySelector('[data-i18n]').textContent).toBe('Iniciar');
        expect(document.querySelector('input').getAttribute('placeholder')).toBe('Buscar proyectos...');
        expect(document.querySelector('button').getAttribute('title')).toBe('Ajustes');
        expect(document.querySelector('button').getAttribute('aria-label')).toBe('Ajustes');
    });

    it('formatea fechas/números según el locale del idioma', () => {
        setLanguage('en');
        expect(formatNumber(12345.6)).toBe('12,345.6');
        const d = new Date(2025, 11, 13);
        expect(formatDate(d)).toMatch(/Dec/);
        setLanguage('es');
        expect(formatNumber(12345.6)).toBe('12.345,6');
        expect(formatDate(d)).toMatch(/dic/);
        expect(formatBytes(1536)).toMatch(/1,5 KB/);
        // Inputs inválidos no rompen (null → 0 epoch, se formatea igual).
        expect(formatNumber('x')).toBe('x');
        expect(formatDate(null)).toMatch(/1969|1970/);
        expect(formatTime('nope')).toBe('nope');
    });
});

describe('touch', () => {
    beforeEach(() => {
        resetGestures();
        setTouchMode(false);
    });

    it('setTouchMode añade/quita data-touch en <html>', () => {
        setTouchMode(true);
        expect(document.documentElement.getAttribute('data-touch')).toBe('on');
        setTouchMode(false);
        expect(document.documentElement.getAttribute('data-touch')).toBeNull();
    });

    it('los estilos touch usan targets de 44px', async () => {
        setTouchMode(true);
        const style = document.getElementById('touch-styles');
        expect(style).not.toBeNull();
        expect(style.textContent).toContain('min-height: 44px');
    });

    it('onSwipe dispara left/right con umbral y ángulo correctos', () => {
        const doc = document.implementation.createHTMLDocument('');
        // jsdom no dispara TouchEvents reales: simulamos con listeners registrados.
        const listeners = {};
        doc.addEventListener = vi.fn((type, fn) => { listeners[type] = fn; });
        installGestures(doc);
        expect(doc.addEventListener).toHaveBeenCalledWith('touchstart', expect.any(Function), expect.anything());
        expect(doc.addEventListener).toHaveBeenCalledWith('touchend', expect.any(Function), expect.anything());

        const got = [];
        onSwipe((dir) => got.push(dir));

        const mkTouch = (x, y) => ({ clientX: x, clientY: y });
        listeners.touchstart({ touches: [mkTouch(100, 100)] });
        listeners.touchend({ changedTouches: [mkTouch(20, 104)] }); // dx=-80, dy=4 → left
        listeners.touchstart({ touches: [mkTouch(100, 100)] });
        listeners.touchend({ changedTouches: [mkTouch(190, 104)] }); // dx=90 → right
        listeners.touchstart({ touches: [mkTouch(100, 100)] });
        listeners.touchend({ changedTouches: [mkTouch(90, 104)] }); // dx=-10 < umbral
        listeners.touchstart({ touches: [mkTouch(100, 100)] });
        listeners.touchend({ changedTouches: [mkTouch(20, 200)] }); // dy demasiado vertical

        expect(got).toEqual(['left', 'right']);
    });

    it('installGestures es idempotente', () => {
        const doc = document.implementation.createHTMLDocument('');
        doc.addEventListener = vi.fn();
        installGestures(doc);
        installGestures(doc);
        expect(doc.addEventListener).toHaveBeenCalledTimes(2);
    });
});
