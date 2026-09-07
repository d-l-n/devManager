// Issue #64 — criterios restantes: métricas en vivo + historial de uptime.
// Cards con CPU/RAM del árbol del server + timelines 24h por proyecto.
import { test, expect } from '@playwright/test';
import { install, seedProjects } from './helpers/wails-mock.js';

test.beforeEach(async ({ page }) => {
    await install(page, { projects: seedProjects() });
    await page.goto('/');
    await page.click('#view-dashboard');
    await expect(page.locator('.dash-card')).toHaveCount(3);
});

test.describe('dashboard metrics and history', () => {
    test('running card shows CPU/RAM chips from monitor data', async ({ page }) => {
        // Gamma está running y el mock devuelve su resRow (CPU 12.5%, 45MB)
        const gammaCard = page.locator('.dash-card', { hasText: 'Gamma' });
        await expect(gammaCard.locator('.dash-chip', { hasText: 'CPU' })).toHaveText('CPU 13%');
        await expect(gammaCard.locator('.dash-chip', { hasText: 'RAM' })).toHaveText('RAM 45 MB');
        // Stopped cards no muestran chips
        const alphaCard = page.locator('.dash-card', { hasText: 'Alpha' });
        await expect(alphaCard.locator('.dash-chip')).toHaveCount(0);
    });

    test('running card shows current uptime label', async ({ page }) => {
        // Seed: Gamma running con uptimeSeconds 120 → "up 2m 0s"
        const gammaCard = page.locator('.dash-card', { hasText: 'Gamma' });
        await expect(gammaCard.locator('.dash-card-uptime')).toHaveText(/up 2m/);
    });

    test('uptime timelines render for projects with history', async ({ page }) => {
        const rows = page.locator('.dash-uptime-row');
        await expect(rows).toHaveCount(2); // Alpha (2h) y Gamma (1h); Beta sin samples

        // Solo segmentos verdes (running) en el mock. El conteo exacto depende
        // del bucketing (12min/bucket sobre 2h+1h de samples) → rangos.
        const ups = await page.locator('.dash-spark-up').count();
        expect(ups).toBeGreaterThanOrEqual(10);
        await expect(page.locator('.dash-spark-down')).toHaveCount(0);
    });

    test('uptime percentage label reflects history', async ({ page }) => {
        // Alpha: todas las muestras running → 100% up
        const alphaRow = page.locator('.dash-uptime-row', { hasText: 'Alpha' });
        await expect(alphaRow.locator('.dash-uptime-pct')).toHaveText('100% up');
    });
});
