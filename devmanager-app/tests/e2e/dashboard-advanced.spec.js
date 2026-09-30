// Issue #64 — criterios restantes: búsqueda/filtros, resultados de tests
// agregados, Run all tests, alertas consolidadas, performance (CPU/RAM 24h)
// y toggles de secciones persistidos en Settings.
import { test, expect } from '@playwright/test';
import { install, seedProjects } from './helpers/wails-mock.js';

test.beforeEach(async ({ page }) => {
    await install(page, { projects: seedProjects() });
    await page.goto('/');
    await page.click('#view-dashboard');
    await expect(page.locator('.dash-card')).toHaveCount(3);
});

test.describe('dashboard search and filters', () => {
    test('search filters cards by name', async ({ page }) => {
        await page.fill('#dash-search', 'alp');
        await expect(page.locator('.dash-card', { hasText: 'Alpha' })).toBeVisible();
        await expect(page.locator('.dash-card', { hasText: 'Beta' })).toBeHidden();
        await expect(page.locator('.dash-card', { hasText: 'Gamma' })).toBeHidden();
        await expect(page.locator('#dash-filter-empty')).toBeHidden();

        // Sin matches → nota de filtro vacío
        await page.fill('#dash-search', 'zzz');
        await expect(page.locator('#dash-filter-empty')).toBeVisible();

        // Limpiar restaura todas
        await page.fill('#dash-search', '');
        await expect(page.locator('#dash-filter-empty')).toBeHidden();
        await expect(page.locator('.dash-card:visible')).toHaveCount(3);
    });

    test('status filter chips narrow the cards', async ({ page }) => {
        // Gamma es el único running en el seed
        await page.click('#dash-filters [data-dash-filter="running"]');
        await expect(page.locator('.dash-card:visible')).toHaveCount(1);
        await expect(page.locator('.dash-card', { hasText: 'Gamma' })).toBeVisible();

        await page.click('#dash-filters [data-dash-filter="stopped"]');
        await expect(page.locator('.dash-card:visible')).toHaveCount(2);

        await page.click('#dash-filters [data-dash-filter="all"]');
        await expect(page.locator('.dash-card:visible')).toHaveCount(3);
    });

    test('failed tests filter matches projects with failed test state', async ({ page }) => {
        await page.evaluate(() => window.__mockState.pwStates.set(2, 'failed'));
        await page.click('#dash-refresh');

        await page.click('#dash-filters [data-dash-filter="failed"]');
        await expect(page.locator('.dash-card:visible')).toHaveCount(1);
        await expect(page.locator('.dash-card', { hasText: 'Gamma' })).toBeVisible();
    });
});

test.describe('dashboard test results aggregation', () => {
    test('cards show test chips and summary aggregates pass/fail', async ({ page }) => {
        await page.evaluate(() => {
            window.__mockState.pwStates.set(0, 'passed');
            window.__mockState.pwStates.set(2, 'failed');
        });
        await page.click('#dash-refresh');

        const alphaCard = page.locator('.dash-card', { hasText: 'Alpha' });
        await expect(alphaCard.locator('.dash-chip.test-ok')).toHaveText('tests passed');
        const gammaCard = page.locator('.dash-card', { hasText: 'Gamma' });
        await expect(gammaCard.locator('.dash-chip.err')).toHaveText('tests failed');

        await expect(page.locator('#dash-summary')).toContainText('tests: 1 passed · 1 failed');
    });
});

test.describe('dashboard quick actions', () => {
    test('run all tests only targets running servers', async ({ page }) => {
        await page.click('#dash-tests-all');

        // Solo Gamma (running) recibe RunTests; el mock pasa a passed.
        const calls = await page.evaluate(() => window.__mockCalls.filter((c) => c[0] === 'RunTests'));
        expect(calls).toEqual([['RunTests', 2]]);

        const gammaCard = page.locator('.dash-card', { hasText: 'Gamma' });
        await expect(gammaCard.locator('.dash-chip', { hasText: 'tests passed' })).toBeVisible({ timeout: 5000 });
    });
});

test.describe('dashboard alerts', () => {
    test('shows port conflict and failed test alerts', async ({ page }) => {
        await page.evaluate(() => {
            window.__mockState.pwStates.set(2, 'failed');
            window.__mockState.portRows = [{
                index: 0, name: 'Alpha', port: 3000, state: 'foreign',
                ownerName: 'node.exe', ownerPID: 999,
            }];
        });
        await page.click('#dash-refresh');

        const alerts = page.locator('.dash-alert');
        await expect(alerts).toHaveCount(2);
        await expect(alerts.first()).toContainText('Port conflict :3000');
        await expect(alerts.first()).toContainText('node.exe');
        await expect(alerts.last()).toContainText('Tests failed: Gamma');
        await expect(page.locator('#dash-alerts-empty')).toBeHidden();
    });

    test('shows empty state when everything is healthy', async ({ page }) => {
        await expect(page.locator('.dash-alert')).toHaveCount(0);
        await expect(page.locator('#dash-alerts-empty')).toBeVisible();
    });
});

test.describe('dashboard performance section', () => {
    test('renders CPU/RAM stats for projects with samples', async ({ page }) => {
        const rows = page.locator('.dash-perf-row');
        await expect(rows).toHaveCount(2); // Alpha (2h) y Gamma (1h); Beta sin samples

        const alpha = page.locator('.dash-perf-row', { hasText: 'Alpha' });
        await expect(alpha.locator('.dash-spark-cpu').first()).toBeAttached();
        await expect(alpha.locator('.dash-perf-stats')).toContainText('avg');
        await expect(alpha.locator('.dash-perf-stats')).toContainText('peak');
        await expect(alpha.locator('.dash-perf-stats')).toContainText('RAM');

        // Beta no tiene samples con CPU → sin fila
        await expect(page.locator('.dash-perf-row', { hasText: 'Beta' })).toHaveCount(0);
    });
});

test.describe('dashboard layout toggles', () => {
    test('menu opens and hides sections, persisting via SetSetting', async ({ page }) => {
        await page.click('#dash-layout');
        await expect(page.locator('#dash-layout-menu')).toBeVisible();

        // Uncheck "Uptime" → sección oculta + SetSetting persistido
        await page.uncheck('[data-dash-section-toggle="uptime"]');
        await expect(page.locator('[data-dash-section="uptime"]')).toBeHidden();
        const calls = await page.evaluate(() => window.__mockCalls.filter((c) => c[0] === 'SetSetting'));
        expect(calls.some((c) => c[1] === 'dashboard_section.uptime' && c[2] === 'false')).toBe(true);

        // Re-check restaura visibilidad
        await page.check('[data-dash-section-toggle="uptime"]');
        await expect(page.locator('[data-dash-section="uptime"]')).toBeVisible();

        // Click fuera cierra el menú
        await page.click('#dash-summary');
        await expect(page.locator('#dash-layout-menu')).toBeHidden();
    });
});
