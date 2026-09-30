import { describe, it, expect, vi, afterEach } from 'vitest';
import {
    analyticsHeadline, coverageClass, coverageHeadline, coverageRows, fixtureSummary,
    flakyRows, formatBytes, formatMs, formatPct, formatRelative, formatSeconds,
    matrixGrid, matrixHeadline, perfRows, scheduleRows, statusLabel, thresholdRows,
    trendBars, validateSchedule, visualHeadline, visualRows, worstFiles,
} from '../testadv.js';
import { mount } from '../panels/testadv.js';

describe('testadv format helpers', () => {
    it('formatea porcentajes y latencias', () => {
        expect(formatPct(66.666)).toBe('66.7%');
        expect(formatPct(undefined)).toBe('0%');
        expect(formatMs(0)).toBe('—');
        expect(formatMs(80.5)).toBe('80.5 ms');
        expect(formatMs(2500)).toBe('2.5 s');
        expect(formatSeconds(0)).toBe('—');
        expect(formatSeconds(45)).toBe('45s');
        expect(formatSeconds(125)).toBe('2m 5s');
    });

    it('formatea bytes', () => {
        expect(formatBytes(512)).toBe('512 B');
        expect(formatBytes(2048)).toBe('2 KB');
        expect(formatBytes(5 * 1024 * 1024)).toBe('5 MB');
    });

    it('formatRelative distingue futuro y pasado', () => {
        const now = new Date('2026-09-30T10:00:00Z');
        expect(formatRelative(new Date('2026-09-30T10:30:00Z'), now)).toBe('in 30m');
        expect(formatRelative(new Date('2026-09-30T09:00:00Z'), now)).toBe('1h ago');
        expect(formatRelative(null, now)).toBe('—');
    });
});

describe('testadv coverage helpers', () => {
    const summary = {
        lines: { covered: 60, total: 100, pct: 60 },
        statements: { covered: 60, total: 100, pct: 60 },
        functions: { covered: 10, total: 10, pct: 100 },
        branches: { covered: 0, total: 0, pct: 100 },
    };

    it('coverageClass clasifica contra el umbral', () => {
        expect(coverageClass(90, 80)).toBe('good');
        expect(coverageClass(75, 80)).toBe('warn');
        expect(coverageClass(40, 80)).toBe('bad');
    });

    it('coverageRows marca métricas vacías', () => {
        const rows = coverageRows(summary);
        expect(rows).toHaveLength(4);
        expect(rows[0]).toMatchObject({ label: 'Lines', pct: 60, empty: false });
        expect(rows[3]).toMatchObject({ label: 'Branches', empty: true });
    });

    it('coverageHeadline resume el veredicto del umbral', () => {
        expect(coverageHeadline(null, 80).text).toBe('No coverage report found');
        const head = coverageHeadline({ available: true, summary: { ...summary, pass: false } }, 80);
        expect(head.text).toBe('60% lines — below threshold (80%)');
        expect(head.state).toBe('bad');
        const ok = coverageHeadline({ available: true, summary: { ...summary, pass: true } }, 50);
        expect(ok.state).toBe('good');
    });

    it('worstFiles normaliza el top de ficheros', () => {
        const files = worstFiles({ worst: [{ path: '/a.js', lines: { pct: 10.55, covered: 1, total: 10 } }] });
        expect(files[0]).toEqual({ path: '/a.js', pct: 10.6, covered: 1, total: 10 });
        expect(worstFiles(null)).toEqual([]);
    });
});

describe('testadv perf helpers', () => {
    it('perfRows omite métricas sin datos', () => {
        expect(perfRows(null)).toEqual([]);
        const rows = perfRows({ source: 'k6', requests: 100, rps: 12.34, p95Ms: 80, errorRate: 1.2, vus: 0 });
        expect(rows.find((r) => r.label === 'Requests').value).toBe('100');
        expect(rows.find((r) => r.label === 'RPS').value).toBe('12.3');
        expect(rows.find((r) => r.label === 'p95').value).toBe('80 ms');
        expect(rows.find((r) => r.label === 'VUs').value).toBe('—');
    });

    it('thresholdRows marca pass/fail', () => {
        const rows = thresholdRows([{ name: 'p95', value: 80.5, limit: 80, pass: false }]);
        expect(rows[0].pass).toBe(false);
        expect(rows[0].text).toContain('p95');
    });
});

describe('testadv analytics helpers', () => {
    it('analyticsHeadline sin runs', () => {
        expect(analyticsHeadline(null).text).toBe('No test runs recorded yet');
        expect(analyticsHeadline({ totalRuns: 0 }).text).toBe('No test runs recorded yet');
    });

    it('analyticsHeadline con runs', () => {
        const head = analyticsHeadline({ totalRuns: 4, passRate: 87.5, avgDurationMs: 12000, stableRuns: 3 });
        expect(head.text).toBe('87.5% pass rate');
        expect(head.detail).toContain('12s');
        expect(head.detail).toContain('3/4');
    });

    it('flakyRows y trendBars normalizan', () => {
        const flaky = flakyRows({ flakyTests: [{ name: 'login', failures: 2, runs: 3, rate: 66.666 }] });
        expect(flaky[0].rate).toBe(66.7);
        const bars = trendBars({ trend: [{ passRate: 85.5, runs: 2 }, { passRate: 0, runs: 1 }] });
        expect(bars[0].height).toBe(86);
        expect(bars[1].height).toBe(2);
        expect(bars[0].label).toBe('85.5% (2 runs)');
    });
});

describe('testadv matrix helpers', () => {
    const cells = [
        { browser: 'chromium', env: 'dev', status: 'passed', runs: 2 },
        { browser: 'chromium', env: 'prod', status: 'failed', runs: 1 },
        { browser: 'firefox', env: 'dev', status: 'pending', runs: 0 },
        { browser: 'firefox', env: 'prod', status: 'pending', runs: 0 },
    ];

    it('matrixGrid agrupa por navegador y rellena huecos', () => {
        const grid = matrixGrid(cells);
        expect(grid.browsers).toEqual(['chromium', 'firefox']);
        expect(grid.envs).toEqual(['dev', 'prod']);
        expect(grid.rows[0].cells[0].status).toBe('passed');
        expect(grid.rows[1].cells[1].status).toBe('pending');
        const partial = matrixGrid([
            { browser: 'chromium', env: 'dev', status: 'passed' },
            { browser: 'firefox', env: 'dev', status: 'pending' },
        ]);
        expect(partial.envs).toEqual(['dev']);
        expect(partial.rows[0].cells).toHaveLength(1);
        // Celda ausente en la lista → pending (hueco rellenado, nunca undefined).
        expect(matrixGrid([]).rows).toEqual([]);
        const gap = matrixGrid([
            { browser: 'chromium', env: 'dev', status: 'passed' },
            { browser: 'chromium', env: 'prod', status: 'pending' },
        ]);
        expect(gap.rows[0].cells[1].status).toBe('pending');
    });

    it('matrixHeadline resume el avance', () => {
        expect(matrixHeadline({ done: 2, total: 4, passRate: 50 })).toBe('2/4 combinations run · 50% passing');
        expect(matrixHeadline({ done: 0, total: 0 })).toBe('No browsers/environments configured');
    });

    it('statusLabel traduce estados', () => {
        expect(statusLabel('passed')).toBe('pass');
        expect(statusLabel('failed')).toBe('fail');
        expect(statusLabel('changed')).toBe('changed');
        expect(statusLabel(undefined)).toBe('pending');
    });
});

describe('testadv visual helpers', () => {
    it('visualHeadline agrega conteos', () => {
        expect(visualHeadline(null).state).toBe('empty');
        const head = visualHeadline({ available: true, total: 5, changed: 1, new: 1, missing: 0 });
        expect(head.text).toBe('5 shots · 1 changed · 1 new');
        expect(head.state).toBe('bad');
    });

    it('visualRows sólo lista lo accionable', () => {
        const rows = visualRows({
            results: [
                { name: 'home.png', status: 'same', diff: { percent: 0 } },
                { name: 'login.png', status: 'changed', diff: { percent: 12.34 } },
                { name: 'old.png', status: 'missing' },
                { name: 'broken.png', status: 'changed', diff: { sizeMismatch: true } },
            ],
        });
        expect(rows.map((r) => r.name)).toEqual(['login.png', 'old.png', 'broken.png']);
        expect(rows[0].detail).toBe('12.3% diff');
        expect(rows[2].detail).toBe('size mismatch');
    });
});

describe('testadv schedules', () => {
    it('scheduleRows calcula el estado de vencimiento', () => {
        const now = new Date('2026-09-30T10:00:00Z');
        const rows = scheduleRows([
            { id: 'a', name: 'nightly', spec: 'daily 03:00', enabled: true, nextRun: '2026-09-30T09:00:00Z', lastResult: 'passed' },
            { id: 'b', name: 'weekly', spec: 'weekly mon 09:00', enabled: false, nextRun: '2026-10-05T09:00:00Z' },
        ], now);
        expect(rows[0].due).toBe(true);
        expect(rows[0].next).toBe('1h ago');
        expect(rows[0].lastResult).toBe('passed');
        expect(rows[1].due).toBe(false);
        expect(rows[1].next).toBe('in 5d');
    });

    it('validateSchedule replica la validación del backend', () => {
        expect(validateSchedule('nightly', 'daily 03:00')).toEqual([]);
        expect(validateSchedule('nightly', 'every 15m')).toEqual([]);
        expect(validateSchedule('nightly', 'weekly mon,wed 09:00')).toEqual([]);
        expect(validateSchedule('', 'daily 03:00')).toEqual(['Name is required']);
        expect(validateSchedule('x', '')).toEqual(['Spec is required']);
        expect(validateSchedule('x', 'every 5s')).toContain('Minimum interval is 1m');
        expect(validateSchedule('x', 'daily 25:00')).toEqual(['Use "daily HH:MM"']);
        expect(validateSchedule('x', 'cada rato')).toHaveLength(1);
    });

    it('fixtureSummary agrega sets', () => {
        expect(fixtureSummary([{ files: 2, bytes: 100 }, { files: 1, bytes: 50 }]))
            .toEqual({ sets: 2, files: 3, bytes: 150 });
        expect(fixtureSummary(null)).toEqual({ sets: 0, files: 0, bytes: 0 });
    });
});

// ---- panel ----

function summaryFixture(overrides = {}) {
    return {
        project: 'Mi App',
        activeEnv: 'dev',
        envs: ['dev', 'prod'],
        browsers: ['chromium', 'firefox'],
        threshold: 80,
        coverage: {
            available: true,
            path: '/p/coverage/coverage-summary.json',
            summary: {
                lines: { covered: 60, total: 100, pct: 60 },
                statements: { covered: 60, total: 100, pct: 60 },
                functions: { covered: 10, total: 10, pct: 100 },
                branches: { covered: 5, total: 10, pct: 50 },
                pass: false,
                threshold: 80,
            },
            worst: [{ path: '/p/src/flojo.js', lines: { pct: 12.5, covered: 1, total: 8 } }],
        },
        perfAvailable: true,
        perf: { source: 'k6', path: '/p/test-results/k6-summary.json', requests: 100, rps: 20, p95Ms: 80, errorRate: 1, durationSeconds: 5, vus: 10 },
        perfChecks: [{ name: 'p95', value: 80, limit: 75, pass: false }],
        thresholds: { p95MaxMs: 75 },
        visual: { available: true, total: 3, same: 2, changed: 1, new: 0, missing: 0, shots: 3, results: [{ name: 'login.png', status: 'changed', diff: { percent: 12 } }] },
        analytics: {
            totalRuns: 3, passRate: 90, avgDurationMs: 2000, stableRuns: 2, totalPassed: 27, totalFailed: 3,
            trend: [{ passRate: 85, runs: 2 }, { passRate: 100, runs: 1 }],
            flakyTests: [{ name: 'login', failures: 1, runs: 3, rate: 33.3 }],
            lastFailure: '2026-09-30T09:00:00Z',
        },
        schedules: {
            schedules: [{ id: 'nightly', name: 'Nightly', spec: 'daily 03:00', command: 'npx playwright test', enabled: true, nextRun: '2026-10-01T03:00:00Z', lastResult: 'passed' }],
            due: [],
        },
        fixtureDirs: ['test-data'],
        fixtures: [{ dir: 'test-data', count: 1, files: 1, bytes: 20, entries: [] }],
        matrix: [
            { browser: 'chromium', env: 'dev', status: 'passed', runs: 2 },
            { browser: 'chromium', env: 'prod', status: 'pending', runs: 0 },
            { browser: 'firefox', env: 'dev', status: 'failed', runs: 1 },
            { browser: 'firefox', env: 'prod', status: 'pending', runs: 0 },
        ],
        matrixProgress: { done: 2, total: 4, passRate: 50 },
        ...overrides,
    };
}

function mountPanel(apiOverrides = {}) {
    const section = document.createElement('section');
    section.id = 'panel-testadv';
    document.body.appendChild(section);
    const api = {
        getTestAdvSummary: vi.fn(async () => summaryFixture()),
        promoteVisualBaseline: vi.fn(async () => 1),
        saveTestSchedule: vi.fn(async () => []),
        deleteTestSchedule: vi.fn(async () => []),
        markTestScheduleRun: vi.fn(async () => []),
        appendTestRun: vi.fn(async () => []),
        ...apiOverrides,
    };
    const ctx = { api, selectedIndex: () => 0 };
    const panel = mount(ctx);
    return { panel, api, section };
}

describe('testadv panel', () => {
    afterEach(() => {
        document.getElementById('panel-testadv')?.remove();
        document.getElementById('ta-styles')?.remove();
        document.body.innerHTML = '';
    });

    it('sin contenedor no rompe (avisa por consola)', () => {
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
        const panel = mount({ api: {}, selectedIndex: () => 0 });
        expect(warn).toHaveBeenCalled();
        expect(panel.refresh()).toBeUndefined();
        warn.mockRestore();
    });

    it('pinta el resumen completo y las barras de cobertura', async () => {
        const { panel, api, section } = mountPanel();
        await panel.onProjectChanged({ name: 'Mi App', path: '/p' });

        expect(api.getTestAdvSummary).toHaveBeenCalledWith(0, 80);
        const text = section.textContent;
        expect(section.querySelector('.ta-headline').textContent).toContain('Mi App');
        expect(section.querySelector('.ta-headline').textContent).toContain('60% lines');
        expect(section.querySelectorAll('.ta-bar-fill').length).toBe(4);
        expect(text).toContain('flojo.js');
        expect(text).toContain('p95');
        expect(text).toContain('login.png');
        expect(text).toContain('33.3%');
        expect(text).toContain('2/4 combinations run');
        expect(section.querySelectorAll('.ta-matrix-cell').length).toBe(4);
        expect(section.querySelector('.ta-matrix-cell.passed').textContent).toBe('pass');
        expect(section.querySelector('.ta-matrix-cell.failed').textContent).toBe('fail');
        expect(text).toContain('Nightly');
        expect(text).toContain('daily 03:00');
        expect(text).toContain('test-data — 1 file(s)');
    });

    it('sin proyecto no llama al backend', async () => {
        const { panel, api } = mountPanel();
        await panel.onProjectChanged(null);
        expect(api.getTestAdvSummary).not.toHaveBeenCalled();
    });

    it('el umbral se valida y dispara refresco', async () => {
        const { panel, api, section } = mountPanel();
        await panel.onProjectChanged({ name: 'Mi App', path: '/p' });
        const input = section.querySelector('.ta-threshold');
        input.value = '55';
        input.dispatchEvent(new Event('change'));
        await new Promise((r) => setTimeout(r, 0));
        expect(api.getTestAdvSummary).toHaveBeenLastCalledWith(0, 55);
        input.value = '999';
        input.dispatchEvent(new Event('change'));
        expect(input.value).toBe('80');
        expect(panel.getState().threshold).toBe(80);
    });

    it('valida el formulario de schedule antes de llamar al backend', async () => {
        const { panel, api, section } = mountPanel();
        await panel.onProjectChanged({ name: 'Mi App', path: '/p' });
        const name = section.querySelector('.ta-sched-name');
        const spec = section.querySelector('.ta-sched-spec');
        const add = section.querySelector('.ta-sched-add');

        name.value = 'Nightly';
        spec.value = 'every 5s';
        add.click();
        await new Promise((r) => setTimeout(r, 0));
        expect(section.querySelector('.ta-sched-errors').textContent).toContain('Minimum interval is 1m');
        expect(api.saveTestSchedule).not.toHaveBeenCalled();

        spec.value = 'daily 03:00';
        add.click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.saveTestSchedule).toHaveBeenCalledWith(0, {
            id: '', name: 'Nightly', spec: 'daily 03:00', command: '', enabled: true,
        });
        expect(name.value).toBe('');
    });

    it('registra un run con browser/env', async () => {
        const { panel, api, section } = mountPanel();
        await panel.onProjectChanged({ name: 'Mi App', path: '/p' });
        section.querySelector('.ta-run-browser').value = 'firefox';
        section.querySelector('.ta-run-env').value = 'prod';
        section.querySelector('.ta-run-passed').value = '5';
        section.querySelector('.ta-run-failed').value = '1';
        section.querySelector('.ta-run-add').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.appendTestRun).toHaveBeenCalledTimes(1);
        const [index, rec] = api.appendTestRun.mock.calls[0];
        expect(index).toBe(0);
        expect(rec).toMatchObject({ browser: 'firefox', env: 'prod', passed: 5, failed: 1 });
        expect(typeof rec.startedAt).toBe('string');
    });

    it('marca y elimina schedules', async () => {
        const { panel, api, section } = mountPanel();
        await panel.onProjectChanged({ name: 'Mi App', path: '/p' });
        const schedule = section.querySelector('.ta-schedule');
        const [passBtn, failBtn, delBtn] = schedule.querySelectorAll('.ta-actions .btn');
        passBtn.click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.markTestScheduleRun).toHaveBeenCalledWith(0, 'nightly', 'passed');
        failBtn.click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.markTestScheduleRun).toHaveBeenLastCalledWith(0, 'nightly', 'failed');
        delBtn.click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.deleteTestSchedule).toHaveBeenCalledWith(0, 'nightly');
    });

    it('promover baseline pide confirmación', async () => {
        const confirm = vi.fn(async () => false);
        const section = document.createElement('section');
        section.id = 'panel-testadv';
        document.body.appendChild(section);
        const api = {
            getTestAdvSummary: vi.fn(async () => summaryFixture()),
            promoteVisualBaseline: vi.fn(async () => 2),
            saveTestSchedule: vi.fn(), deleteTestSchedule: vi.fn(),
            markTestScheduleRun: vi.fn(), appendTestRun: vi.fn(),
        };
        const panel = mount({ api, selectedIndex: () => 0, messageDialog: { confirm } });
        await panel.onProjectChanged({ name: 'Mi App', path: '/p' });
        const btn = [...section.querySelectorAll('.btn')].find((b) => b.textContent === 'Promote baseline');
        btn.click();
        await Promise.resolve();
        expect(confirm).toHaveBeenCalled();
        expect(api.promoteVisualBaseline).not.toHaveBeenCalled();

        confirm.mockImplementation(async () => true);
        btn.click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.promoteVisualBaseline).toHaveBeenCalledWith(0);
    });

    it('muestra los errores del backend sin romper', async () => {
        const { panel, api, section } = mountPanel({
            getTestAdvSummary: vi.fn(async () => { throw new Error('backend caído'); }),
        });
        await panel.onProjectChanged({ name: 'Mi App', path: '/p' });
        expect(section.querySelector('.ta-headline').textContent).toContain('backend caído');
        expect(api.promoteVisualBaseline).not.toHaveBeenCalled();
    });

    it('sin backend disponible explica el motivo', async () => {
        const section = document.createElement('section');
        section.id = 'panel-testadv';
        document.body.appendChild(section);
        const panel = mount({ selectedIndex: () => 0 });
        await panel.onProjectChanged({ name: 'Mi App', path: '/p' });
        expect(section.querySelector('.ta-sched-errors').textContent).toContain('Backend not available');
    });
});
