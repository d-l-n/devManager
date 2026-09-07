// Issue #46 — caso 4: notificaciones toast.
// Evento 'notify' desde Go → toast visible → auto-dismiss ~4s (4000ms + fade).
import { test, expect } from '@playwright/test';
import { install, seedProjects } from './helpers/wails-mock.js';

test.describe('toast notifications', () => {
    test('notify event shows toast and auto-dismisses', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');
        await expect(page.locator('#project-list li')).toHaveCount(3);

        // Emitir evento "desde Go" como lo hace el backend real
        await page.evaluate(() =>
            window.__emitGo('notify', { title: 'Hello', message: 'From Go mock', level: 'success' }));

        const toast = page.locator('#toast-container .toast');
        await expect(toast).toHaveCount(1);
        await expect(page.locator('.toast-title')).toHaveText('Hello');
        await expect(page.locator('.toast-msg')).toHaveText('From Go mock');

        // Auto-dismiss 4000ms + fade 220ms
        await expect(toast).toHaveCount(0, { timeout: 6000 });
    });

    test('click dismisses toast immediately', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');
        await expect(page.locator('#project-list li')).toHaveCount(3);

        await page.evaluate(() =>
            window.__emitGo('notify', { title: 'Click me', message: '', level: 'info' }));
        const toast = page.locator('#toast-container .toast');
        await expect(toast).toHaveCount(1);
        await toast.click();
        await expect(toast).toHaveCount(0, { timeout: 1000 });
    });
});
