// Config Playwright E2E (Issue #46): levanta el dev server de Vite (puerto
// 5173, strictPort) y corre los specs de tests/e2e contra el frontend con el
// bridge de Wails mockeado (ver tests/e2e/helpers/wails-mock.js).
import { defineConfig } from '@playwright/test';

export default defineConfig({
    testDir: './tests/e2e',
    timeout: 30000,
    expect: { timeout: 5000 },
    fullyParallel: true,
    retries: process.env.CI ? 2 : 0,
    workers: process.env.CI ? 1 : undefined,
    reporter: 'list',
    use: {
        baseURL: 'http://localhost:5173',
        // "system" resuelve a dark de forma determinista (theme.js usa
        // matchMedia prefers-color-scheme).
        colorScheme: 'dark',
        trace: 'retain-on-failure',
    },
    webServer: {
        command: 'npm run dev',
        cwd: './frontend',
        url: 'http://localhost:5173',
        reuseExistingServer: !process.env.CI,
        timeout: 60000,
    },
});
