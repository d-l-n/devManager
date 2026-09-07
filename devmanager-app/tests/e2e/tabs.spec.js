// Issue #46 — caso 2: navegación de tabs.
// Click en cada tab visible → su panel queda active.
import { test, expect } from '@playwright/test';
import { install, seedProjects } from './helpers/wails-mock.js';

test.describe('tab navigation', () => {
    test('each visible tab activates its panel', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');

        // El seed deja visibles todas las tabs: logs, scripts, git (isRepo),
        // deps (hasPackageManager), playwright (enabled), evidence, obscura, backlog.
        const visibleTabs = await page.locator('.tab:visible').evaluateAll(
            (btns) => btns.map((b) => b.dataset.tab));
        expect(visibleTabs.length).toBeGreaterThanOrEqual(8);

        for (const name of visibleTabs) {
            await test.step(`tab ${name}`, async () => {
                // Locators por data-tab: ".tab:visible" se re-resuelve tras cada
                // click y rompería las aserciones de los índices previos.
                const btn = page.locator(`.tab[data-tab="${name}"]`);
                await btn.click();
                await expect(btn).toHaveClass(/active/);
                await expect(btn).toHaveAttribute('aria-selected', 'true');
                await expect(page.locator(`#panel-${name}`)).toHaveClass(/active/);
                // Solo un panel activo a la vez
                const activeCount = await page.locator('.panel.active').count();
                expect(activeCount).toBe(1);
            });
        }
    });
});
