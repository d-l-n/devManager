// Issue #46 — caso 5: cambio de tema.
// Toggle del botón → html[data-theme] cambia (CSS variables del root).
import { test, expect } from '@playwright/test';
import { install, seedProjects } from './helpers/wails-mock.js';

test.describe('theme switching', () => {
    test('button toggles dark ↔ light and persists via SetSetting', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');

        // Default del mock: dark
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

        await page.click('#btn-theme');
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');

        await page.click('#btn-theme');
        await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

        // La preferencia se persistió vía el bridge
        const settingCalls = await page.evaluate(() =>
            (window.__mockCalls || []).filter(([method, key]) => method === 'SetSetting' && key === 'theme'));
        expect(settingCalls.length).toBe(2);
        // El valor persistido tras los dos toggles vuelve a ser dark
        const last = await page.evaluate(() => window.__mockState.settings.theme);
        expect(last).toBe('dark');
    });
});
