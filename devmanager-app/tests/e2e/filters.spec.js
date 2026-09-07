// Issue #46 — caso 6: chips de filtro.
// Filtro Running → solo proyectos running en la lista; Stopped → solo stopped.
import { test, expect } from '@playwright/test';
import { install, seedProjects } from './helpers/wails-mock.js';

test.describe('filter chips', () => {
    test('running filter shows only running projects', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');

        // Seed: Alpha/Beta stopped, Gamma running
        await expect(page.locator('#project-list li')).toHaveCount(3);

        await page.click('.filter-chip[data-filter="running"]');
        await expect(page.locator('#project-list li')).toHaveCount(1);
        await expect(page.locator('#project-list')).toContainText('Gamma');
        await expect(page.locator('#project-list')).not.toContainText('Alpha');
        await expect(page.locator('#project-list')).not.toContainText('Beta');

        // El chip queda activo
        await expect(page.locator('.filter-chip[data-filter="running"]')).toHaveClass(/active/);
    });

    test('stopped filter shows only stopped projects', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');

        await page.click('.filter-chip[data-filter="stopped"]');
        await expect(page.locator('#project-list li')).toHaveCount(2);
        await expect(page.locator('#project-list')).toContainText('Alpha');
        await expect(page.locator('#project-list')).toContainText('Beta');
        await expect(page.locator('#project-list')).not.toContainText('Gamma');
    });

    test('stopping a running project under running filter removes it from list', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');

        await page.click('.filter-chip[data-filter="running"]');
        await expect(page.locator('#project-list li')).toHaveCount(1);

        // Seleccionar Gamma y detener su servidor
        await page.locator('#project-list li', { hasText: 'Gamma' }).click();
        await page.click('#btn-stop');

        // serverStates cambia → refreshStatus → lista filtrada se actualiza
        await expect(page.locator('#project-list li')).toHaveCount(0);
        await expect(page.locator('#projects-empty')).toBeVisible();
        await expect(page.locator('#projects-empty')).toHaveText('No running projects');
    });
});
