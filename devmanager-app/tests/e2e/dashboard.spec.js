// Issue #64 — vista Dashboard global.
// Segment Dashboard → cards por proyecto, resumen consolidado, acciones
// rápidas (start/stop individual + Start all / Stop all).
import { test, expect } from '@playwright/test';
import { install, seedProjects } from './helpers/wails-mock.js';

test.describe('dashboard view', () => {
    test('renders one card per project with state and port', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');
        await expect(page.locator('#project-list li')).toHaveCount(3);

        await page.click('#view-dashboard');
        await expect(page.locator('#dashboard-view')).toBeVisible();
        await expect(page.locator('.dash-card')).toHaveCount(3);

        // Gamma (seed índice 2) está running; Alpha stopped
        const gammaCard = page.locator('.dash-card', { hasText: 'Gamma' });
        await expect(gammaCard.locator('.badge')).toHaveClass(/running/);
        await expect(gammaCard.locator('.badge')).toHaveText('running');
        await expect(gammaCard.locator('.dash-card-port')).toHaveText(':3002');

        const alphaCard = page.locator('.dash-card', { hasText: 'Alpha' });
        await expect(alphaCard.locator('.badge')).toHaveClass(/stopped/);
    });

    test('summary shows running/stopped counts and ports in use', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');
        await page.click('#view-dashboard');

        await expect(page.locator('#dash-summary')).toContainText('1 running');
        await expect(page.locator('#dash-summary')).toContainText('2 stopped');
    });

    test('card start/stop buttons work', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');
        await page.click('#view-dashboard');

        const alphaCard = page.locator('.dash-card', { hasText: 'Alpha' });
        await alphaCard.getByRole('button', { name: 'Start' }).click();
        await expect(alphaCard.locator('.badge')).toHaveClass(/running/, { timeout: 5000 });

        await alphaCard.getByRole('button', { name: 'Stop' }).click();
        await expect(alphaCard.locator('.badge')).toHaveClass(/stopped/, { timeout: 5000 });
    });

    test('start all starts every stopped server', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');
        await page.click('#view-dashboard');

        await page.click('#dash-start-all');
        // server:state por proyecto → refresh → todas las badges running
        await expect(page.locator('.dash-card .badge.running')).toHaveCount(3, { timeout: 5000 });
        await expect(page.locator('#dash-summary')).toContainText('3 running');
    });

    test('stop all asks for confirmation and stops every server', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');
        await page.click('#view-dashboard');

        await page.click('#dash-stop-all');
        // Confirm destructivo
        await page.getByRole('button', { name: 'Stop all' }).last().click();
        await expect(page.locator('.dash-card .badge.stopped')).toHaveCount(3, { timeout: 5000 });
        await expect(page.locator('#dash-summary')).toContainText('0 running');
    });

    test('clicking a card selects the project and returns to Project view', async ({ page }) => {
        await install(page, { projects: seedProjects() });
        await page.goto('/');
        await page.click('#view-dashboard');

        await page.locator('.dash-card', { hasText: 'Beta' }).click();
        await expect(page.locator('#view-project')).toHaveClass(/active/);
        await expect(page.locator('#project-detail')).toBeVisible();
        await expect(page.locator('#project-name')).toHaveText('Beta');
    });
});
