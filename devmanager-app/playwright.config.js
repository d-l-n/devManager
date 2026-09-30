// Config Playwright E2E (Issue #46): levanta el dev server de Vite en un puerto
// dedicado (5273) —distinto del 5173 del `wails dev`/desarrollo normal— y corre
// los specs de tests/e2e contra el frontend con el bridge de Wails mockeado
// (ver tests/e2e/helpers/wails-mock.js).
import { defineConfig } from '@playwright/test';

// Puerto propio de la suite: con baseURL 5173 + reuseExistingServer, cualquier
// otro dev server de la máquina en ese puerto hacía que toda la suite corriera
// contra otra aplicación (resultados falsos, no un error).
const E2E_PORT = 5273;

export default defineConfig({
    testDir: './tests/e2e',
    timeout: 30000,
    expect: { timeout: 5000 },
    fullyParallel: true,
    retries: process.env.CI ? 2 : 0,
    workers: process.env.CI ? 1 : undefined,
    reporter: 'list',
    use: {
        baseURL: `http://localhost:${E2E_PORT}`,
        // "system" resuelve a dark de forma determinista (theme.js usa
        // matchMedia prefers-color-scheme).
        colorScheme: 'dark',
        trace: 'retain-on-failure',
    },
    webServer: {
        // strictPort está activo en frontend/vite.config.js: si 5273 está
        // ocupado, Vite falla y Playwright avisa en vez de testear otra app.
        command: `npm run dev -- --port ${E2E_PORT}`,
        cwd: './frontend',
        url: `http://localhost:${E2E_PORT}`,
        // Nunca reusar un server ajeno: si el puerto está ocupado queremos el
        // error, no un verde falso.
        reuseExistingServer: false,
        timeout: 60000,
    },
});
