// Issue #46 — caso 1: ciclo de vida de proyecto.
// Add project → Start server → badge running → Stop → Remove (context menu).
import { test, expect } from '@playwright/test';
import { install, seedProjects } from './helpers/wails-mock.js';

test.describe('project lifecycle', () => {
    test('add → start → badge running → stop → remove', async ({ page }) => {
        await install(page, { projects: [] });
        await page.goto('/');

        // ---- Add ----
        await page.click('#btn-add');
        await page.fill('#pf-name', 'Demo');
        await page.fill('#pf-path', 'C:/dev/demo');
        await page.getByRole('button', { name: 'OK' }).click();
        await expect(page.locator('#project-list')).toContainText('Demo');
        // onSaved(-1) auto-selecciona el último proyecto
        await expect(page.locator('#project-count')).toHaveText('1 project(s)');

        // ---- Start ----
        await page.click('#btn-start');
        await expect(page.locator('#state-badge')).toHaveClass(/running/, { timeout: 5000 });
        await expect(page.locator('#state-badge')).toHaveText('running');
        await expect(page.locator('#btn-stop')).toBeEnabled();
        await expect(page.locator('#btn-start')).toBeDisabled();

        // ---- Stop ----
        await page.click('#btn-stop');
        await expect(page.locator('#state-badge')).toHaveClass(/stopped/);
        await expect(page.locator('#btn-start')).toBeEnabled();

        // ---- Remove vía context menu ----
        await page.locator('#project-list li').first().click({ button: 'right' });
        await page.getByRole('menuitem', { name: 'Remove Project' }).click();
        // Confirm dialog destructivo: el botón confirma con el label "Remove"
        await page.getByRole('button', { name: 'Remove', exact: true }).click();
        await expect(page.locator('#project-list li')).toHaveCount(0);
        await expect(page.locator('#projects-empty')).toBeVisible();
        await expect(page.locator('#project-count')).toHaveText('0 project(s)');
    });

    test('seeded project renders with running state badge', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');
        await expect(page.locator('#project-count')).toHaveText('3 project(s)');
        // Gamma (índice 2) está running en el seed
        await page.locator('#project-list li', { hasText: 'Gamma' }).click();
        await expect(page.locator('#state-badge')).toHaveClass(/running/);
        await expect(page.locator('#btn-stop')).toBeEnabled();
    });
});
