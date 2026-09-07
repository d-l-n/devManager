// Issue #46 — caso 3: menú contextual.
// Right-click en proyecto → ítems del menú visibles → acción dispara la API.
import { test, expect } from '@playwright/test';
import { install, seedProjects } from './helpers/wails-mock.js';

test.describe('context menu', () => {
    test('shows items and executes action (Open Terminal)', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');

        await page.locator('#project-list li').first().click({ button: 'right' });
        const menu = page.locator('.context-menu');
        await expect(menu).toBeVisible();

        // Ítems esperados (showProjectContextMenu)
        for (const label of ['Restart Server', 'Stop Server', 'Open in Browser', 'Open in Explorer', 'Open Terminal', 'Open in VS Code', 'Run Tests', 'Pin', 'Edit Project', 'Remove Project']) {
            await expect(menu.getByRole('menuitem', { name: label })).toBeVisible();
        }

        // Acción segura (mock no-op) → se llamó a la API y el menú se cierra
        await menu.getByRole('menuitem', { name: 'Open Terminal' }).click();
        await expect(menu).toBeHidden();
        const called = await page.evaluate(() =>
            (window.__mockCalls || []).some(([method]) => method === 'OpenTerminal'));
        expect(called).toBe(true);
    });

    test('Pin action toggles and refreshes list', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');

        await page.locator('#project-list li').first().click({ button: 'right' });
        await page.getByRole('menuitem', { name: 'Pin' }).click();
        await expect(page.locator('.context-menu')).toBeHidden();
        // TogglePin llama a la API y el refresh reordena (pinned primero)
        const called = await page.evaluate(() =>
            (window.__mockCalls || []).some(([method]) => method === 'TogglePin'));
        expect(called).toBe(true);
    });
});
