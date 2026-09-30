// Panel de testing avanzado (Issue #70): cobertura, performance, regresión
// visual, analítica de runs, matriz navegador x entorno, schedules y datos de
// test.
//
// Integración (pendiente, a propósito): este módulo NO edita index.html ni
// main.js para no colisionar con #67. Para activarlo:
//   1. index.html: <button class="tab" data-tab="testadv" role="tab" ...>Testing</button>
//      y <section id="panel-testadv" class="panel" role="tabpanel"></section>
//   2. main.js: import + const testadvPanel = mountTestAdv(ctx) + ctx.panels
//      + añadir 'testadv' al array `known` de applyTabOrder().
// Los bindings viven aquí (no en api.js) por el mismo motivo: en cuanto se
// muevan a api.js, el panel los detecta vía ctx.api sin cambios.
//
// El panel construye TODO su markup dentro del contenedor, así que el diff de
// wiring es de 3 líneas y no reordena nada del HTML existente.

import {
    analyticsHeadline, coverageHeadline, coverageClass, coverageRows, fixtureSummary,
    flakyRows, formatBytes, formatMs, formatPct, formatRelative,
    matrixGrid, matrixHeadline, perfRows, scheduleRows, statusLabel, thresholdRows,
    trendBars, validateSchedule, visualHeadline, visualRows, worstFiles,
} from '../testadv.js';

const DEFAULT_THRESHOLD = 80;

// Estilos propios con prefijo .ta-: se inyectan desde el panel para no tocar
// los CSS de temas (theme.css/dracula.css/...) que están en edición por #67.
const STYLE_ID = 'ta-styles';
const STYLES = `
.ta-wrap { display: flex; flex-direction: column; gap: 8px; overflow-y: auto; padding: 8px; }
.ta-toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.ta-toolbar .ta-headline { flex: 1; min-width: 160px; }
.ta-threshold { width: 70px; }
.ta-badge { border-radius: 10px; padding: 2px 8px; font-size: 12px; border: 1px solid var(--border, #444); }
.ta-badge.good, .ta-matrix-cell.passed, .ta-bar-fill.good, .ta-trend-fill.good { background: #1f8a4c; color: #fff; }
.ta-badge.warn, .ta-matrix-cell.pending, .ta-bar-fill.warn, .ta-trend-fill.warn { background: #b8860b; color: #fff; }
.ta-badge.bad, .ta-matrix-cell.failed, .ta-bar-fill.bad, .ta-trend-fill.bad { background: #a32020; color: #fff; }
.ta-bar-row { display: grid; grid-template-columns: 90px 1fr 64px; align-items: center; gap: 6px; margin: 2px 0; }
.ta-bar-track { height: 10px; background: rgba(128,128,128,.25); border-radius: 5px; overflow: hidden; }
.ta-bar-fill { height: 100%; border-radius: 5px; }
.ta-bar-value { text-align: right; font-size: 12px; }
.ta-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(96px, 1fr)); gap: 6px; margin: 4px 0; }
.ta-cell { display: flex; flex-direction: column; }
.ta-line { padding: 2px 0; }
.ta-checks { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 4px; }
.ta-trend { display: flex; align-items: flex-end; gap: 3px; height: 56px; margin: 6px 0; }
.ta-trend-bar { flex: 1; height: 100%; display: flex; align-items: flex-end; background: rgba(128,128,128,.15); border-radius: 3px; }
.ta-trend-fill { width: 100%; border-radius: 3px; }
.ta-matrix { display: flex; flex-direction: column; gap: 3px; margin: 4px 0; }
.ta-matrix-row { display: flex; gap: 3px; }
.ta-matrix-head { width: 88px; font-size: 12px; }
.ta-matrix-cell { flex: 1; text-align: center; font-size: 12px; border-radius: 4px; padding: 3px 0; background: rgba(128,128,128,.2); }
.ta-form { display: flex; gap: 6px; flex-wrap: wrap; align-items: center; margin: 6px 0; }
.ta-form .text-input { min-width: 110px; flex: 1; }
.ta-errors { color: #e05656; font-size: 12px; }
.ta-schedule { display: flex; justify-content: space-between; align-items: center; gap: 8px; border-top: 1px solid rgba(128,128,128,.2); padding: 4px 0; }
.ta-schedule-info { display: flex; flex-direction: column; gap: 2px; }
.ta-actions { display: flex; gap: 4px; }
.ta-stamp { margin-left: 6px; }
`;

function ensureStyles() {
    if (typeof document === 'undefined' || document.getElementById(STYLE_ID)) return;
    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = STYLES;
    document.head.appendChild(style);
}

export function createBindings(getApp) {
    return {
        getTestAdvSummary: (i, threshold) => getApp().GetTestAdvSummary(i, threshold),
        getVisualReport: (i, tolerance, maxPercent) => getApp().GetVisualReport(i, tolerance, maxPercent),
        promoteVisualBaseline: (i) => getApp().PromoteVisualBaseline(i),
        saveTestSchedule: (i, schedule) => getApp().SaveTestSchedule(i, schedule),
        deleteTestSchedule: (i, id) => getApp().DeleteTestSchedule(i, id),
        markTestScheduleRun: (i, id, result) => getApp().MarkTestScheduleRun(i, id, result),
        appendTestRun: (i, rec) => getApp().AppendTestRun(i, rec),
    };
}

// resolveApi acepta el wrapper de api.js si ya está cableado; si no, habla con
// los bindings Wails directamente (así el panel funciona antes del wiring).
export function resolveApi(ctx) {
    const fromCtx = (ctx && ctx.api) || null;
    if (fromCtx && typeof fromCtx.getTestAdvSummary === 'function') return fromCtx;
    const go = typeof window !== 'undefined' && window.go && window.go.main && window.go.main.App;
    if (!go) return null;
    return createBindings(() => go);
}

function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

function clear(node) {
    while (node.firstChild) node.removeChild(node.firstChild);
}

function badge(text, stateClass) {
    return el('span', `ta-badge ${stateClass || ''}`.trim(), text);
}

function emptyRow(text) {
    return el('div', 'dim small padded', text);
}

export function mount(ctx) {
    const root = document.getElementById('panel-testadv');
    if (!root) {
        console.warn('[testadv] falta <section id="panel-testadv"> en index.html (ver cabecera del módulo)');
        return { onProjectChanged() {}, refresh() {} };
    }
    clear(root);
    ensureStyles();

    const state = { project: null, threshold: DEFAULT_THRESHOLD, loading: false };
    const api = resolveApi(ctx);
    const selected = () => (ctx && typeof ctx.selectedIndex === 'function' ? ctx.selectedIndex() : -1);

    // ---- markup ----
    const wrap = el('div', 'ta-wrap');
    const toolbar = el('header', 'ta-toolbar');
    const headline = el('span', 'ta-headline dim', 'Select a project');
    const thresholdInput = el('input', 'text-input ta-threshold');
    thresholdInput.type = 'number';
    thresholdInput.min = '0';
    thresholdInput.max = '100';
    thresholdInput.value = String(DEFAULT_THRESHOLD);
    thresholdInput.title = 'Coverage threshold (%)';
    thresholdInput.setAttribute('aria-label', 'Coverage threshold');
    const refreshBtn = el('button', 'btn btn-accent', 'Refresh');
    const baselineBtn = el('button', 'btn btn-primary', 'Promote baseline');
    baselineBtn.title = 'Copy test-results/visual/current/*.png over baseline';
    toolbar.append(el('span', 'strong', 'Testing advanced'), headline, thresholdInput, refreshBtn, baselineBtn);
    wrap.appendChild(toolbar);

    const sections = {};
    const section = (key, title) => {
        const group = el('div', 'group');
        group.appendChild(el('div', 'group-title', title));
        const body = el('div', `ta-section ta-${key}`);
        group.appendChild(body);
        wrap.appendChild(group);
        sections[key] = body;
        return body;
    };
    section('coverage', 'Coverage');
    section('perf', 'Performance');
    section('visual', 'Visual regression');
    section('analytics', 'Run analytics');
    section('matrix', 'Test matrix');
    section('schedules', 'Scheduled runs');
    section('fixtures', 'Test data');

    // Formulario de schedule.
    const schedForm = el('div', 'ta-form ta-sched-form');
    const schedName = el('input', 'text-input ta-sched-name');
    schedName.placeholder = 'Nightly suite';
    schedName.setAttribute('aria-label', 'Schedule name');
    const schedSpec = el('input', 'text-input mono ta-sched-spec');
    schedSpec.placeholder = 'daily 03:00';
    schedSpec.setAttribute('aria-label', 'Schedule spec');
    const schedCommand = el('input', 'text-input mono ta-sched-command');
    schedCommand.placeholder = 'npx playwright test --project=chromium';
    schedCommand.setAttribute('aria-label', 'Schedule command');
    const schedAdd = el('button', 'btn btn-success ta-sched-add', 'Add schedule');
    schedForm.append(schedName, schedSpec, schedCommand, schedAdd);
    sections.schedules.appendChild(schedForm);
    const schedErrors = el('div', 'ta-errors ta-sched-errors', '');
    schedErrors.hidden = true;
    sections.schedules.appendChild(schedErrors);

    // Formulario para registrar un run (browser x env → alimenta matriz/analítica).
    const runForm = el('div', 'ta-form ta-run-form');
    const runBrowser = el('input', 'text-input mono ta-run-browser');
    runBrowser.placeholder = 'chromium';
    runBrowser.setAttribute('aria-label', 'Run browser');
    const runEnv = el('input', 'text-input mono ta-run-env');
    runEnv.placeholder = 'dev';
    runEnv.setAttribute('aria-label', 'Run environment');
    const runPassed = el('input', 'text-input ta-run-passed');
    runPassed.type = 'number';
    runPassed.min = '0';
    runPassed.value = '0';
    runPassed.setAttribute('aria-label', 'Passed tests');
    const runFailed = el('input', 'text-input ta-run-failed');
    runFailed.type = 'number';
    runFailed.min = '0';
    runFailed.value = '0';
    runFailed.setAttribute('aria-label', 'Failed tests');
    const runAdd = el('button', 'btn btn-primary ta-run-add', 'Record run');
    runForm.append(
        el('span', 'dim small', 'Record run:'), runBrowser, runEnv, runPassed, runFailed, runAdd,
    );
    sections.matrix.appendChild(runForm);
    const runErrors = el('div', 'ta-errors ta-run-errors', '');
    runErrors.hidden = true;
    sections.matrix.appendChild(runErrors);

    wrap.appendChild(el('div', 'dim small', 'Artefactos leídos: coverage/, test-results/visual/, k6/artillery JSON, test-data/'));
    root.appendChild(wrap);

    // ---- render ----
    function showErrors(node, errs) {
        const list = Array.isArray(errs) ? errs.filter(Boolean) : errs ? [String(errs)] : [];
        clear(node);
        node.hidden = list.length === 0;
        list.forEach((msg) => node.appendChild(el('div', '', msg)));
    }

    function renderCoverage(coverage) {
        const body = sections.coverage;
        clear(body);
        const head = coverageHeadline(coverage, state.threshold);
        body.appendChild(badge(head.text, head.state === 'empty' ? '' : head.state));
        if (!coverage || !coverage.available) {
            body.appendChild(emptyRow('Run coverage in the project and refresh (nyc/c8, go test -cover, cobertura).'));
            return;
        }
        coverageRows(coverage.summary).forEach((row) => {
            const line = el('div', 'ta-bar-row');
            line.appendChild(el('span', 'ta-bar-label', row.label));
            const track = el('div', 'ta-bar-track');
            const fill = el('div', `ta-bar-fill ${coverageClass(row.pct, state.threshold)}`);
            fill.style.width = `${Math.max(0, Math.min(100, row.pct))}%`;
            track.appendChild(fill);
            line.appendChild(track);
            line.appendChild(el('span', 'ta-bar-value mono', row.empty ? '—' : formatPct(row.pct)));
            body.appendChild(line);
        });
        const worst = worstFiles(coverage);
        if (worst.length) {
            body.appendChild(el('div', 'dim small', 'Thinnest files'));
            worst.forEach((f) => {
                body.appendChild(el('div', 'ta-line mono small',
                    `${formatPct(f.pct)} · ${f.path} (${f.covered}/${f.total})`));
            });
        }
    }

    function renderPerf(summary) {
        const body = sections.perf;
        clear(body);
        if (!summary.perfAvailable) {
            body.appendChild(emptyRow('No k6-summary.json / artillery-report.json found.'));
            return;
        }
        body.appendChild(el('div', 'dim small', `${summary.perf.source} · ${summary.perf.path || ''}`));
        const grid = el('div', 'ta-grid');
        perfRows(summary.perf).forEach((row) => {
            const cell = el('div', 'ta-cell');
            cell.appendChild(el('span', 'ta-cell-label dim small', row.label));
            cell.appendChild(el('span', 'ta-cell-value mono', row.value));
            grid.appendChild(cell);
        });
        body.appendChild(grid);
        const checks = thresholdRows(summary.perfChecks);
        if (checks.length) {
            const row = el('div', 'ta-checks');
            checks.forEach((c) => row.appendChild(badge(c.text, c.pass ? 'good' : 'bad')));
            body.appendChild(row);
        }
        const notes = [];
        if (summary.thresholds) {
            if (summary.thresholds.p95MaxMs > 0) notes.push(`p95 ≤ ${formatMs(summary.thresholds.p95MaxMs)}`);
            if (summary.thresholds.p99MaxMs > 0) notes.push(`p99 ≤ ${formatMs(summary.thresholds.p99MaxMs)}`);
            if (summary.thresholds.errorRateMaxPct > 0) notes.push(`errors ≤ ${formatPct(summary.thresholds.errorRateMaxPct)}`);
            if (summary.thresholds.rpsMin > 0) notes.push(`rps ≥ ${summary.thresholds.rpsMin}`);
        }
        if (notes.length) body.appendChild(el('div', 'dim small', `Thresholds: ${notes.join(' · ')}`));
    }

    function renderVisual(visual) {
        const body = sections.visual;
        clear(body);
        const head = visualHeadline(visual);
        body.appendChild(badge(head.text, head.state === 'empty' ? '' : head.state));
        const rows = visualRows(visual);
        if (!rows.length) {
            body.appendChild(emptyRow(visual && visual.available
                ? 'No regressions detected.'
                : 'Drop baseline/current screenshots in test-results/visual/.'));
            return;
        }
        rows.forEach((r) => {
            body.appendChild(el('div', 'ta-line mono small', `${statusLabel(r.status)} · ${r.name} ${r.detail}`.trim()));
        });
    }

    function renderAnalytics(analytics) {
        const body = sections.analytics;
        clear(body);
        const head = analyticsHeadline(analytics);
        body.appendChild(el('div', 'strong', head.text));
        if (head.detail) body.appendChild(el('div', 'dim small', head.detail));
        const bars = trendBars(analytics);
        if (bars.length) {
            const track = el('div', 'ta-trend');
            bars.forEach((b) => {
                const bar = el('div', 'ta-trend-bar');
                const fill = el('div', `ta-trend-fill ${coverageClass(b.passRate, 90)}`);
                fill.style.height = `${b.height}%`;
                bar.appendChild(fill);
                bar.title = b.label;
                track.appendChild(bar);
            });
            body.appendChild(track);
        }
        const flaky = flakyRows(analytics);
        if (flaky.length) {
            body.appendChild(el('div', 'dim small', 'Flaky tests'));
            flaky.forEach((f) => {
                body.appendChild(el('div', 'ta-line small',
                    `${f.name} — failed in ${f.failures}/${f.runs} runs (${formatPct(f.rate)})`));
            });
        }
        if (analytics && analytics.lastFailure) {
            body.appendChild(el('div', 'dim small', `Last failure: ${formatRelative(new Date(analytics.lastFailure))}`));
        }
    }

    function renderMatrix(summary) {
        const body = sections.matrix;
        // Conserva el formulario de run al repintar la matriz.
        [runForm, runErrors].forEach((node) => node.remove());
        clear(body);
        body.appendChild(el('div', 'dim small', matrixHeadline(summary.matrixProgress)));
        const grid = matrixGrid(summary.matrix);
        if (!grid.rows.length) {
            body.appendChild(emptyRow('No browsers/environments configured.'));
        } else {
            const table = el('div', 'ta-matrix');
            const headRow = el('div', 'ta-matrix-row');
            headRow.appendChild(el('div', 'ta-matrix-head', ''));
            grid.envs.forEach((env) => headRow.appendChild(el('div', 'ta-matrix-head mono', env)));
            table.appendChild(headRow);
            grid.rows.forEach((row) => {
                const line = el('div', 'ta-matrix-row');
                line.appendChild(el('div', 'ta-matrix-head mono', row.browser));
                row.cells.forEach((cell) => {
                    const node = el('div', `ta-matrix-cell ${cell.status}`, statusLabel(cell.status));
                    if (cell.runs) node.title = `${cell.runs} run(s)`;
                    line.appendChild(node);
                });
                table.appendChild(line);
            });
            body.appendChild(table);
        }
        body.append(runForm, runErrors);
    }

    function renderSchedules(schedules) {
        const body = sections.schedules;
        // Conserva el formulario y el contenedor de errores.
        [schedForm, schedErrors].forEach((node) => node.remove());
        clear(body);
        const rows = scheduleRows((schedules && schedules.schedules) || []);
        const due = ((schedules && schedules.due) || []).length;
        body.appendChild(el('div', 'dim small', due
            ? `${due} schedule(s) due now`
            : rows.length ? 'Next runs scheduled' : 'No scheduled runs yet'));
        rows.forEach((row) => {
            const line = el('div', 'ta-schedule');
            const info = el('div', 'ta-schedule-info');
            info.appendChild(el('span', `strong ${row.enabled ? '' : 'dim'}`.trim(), row.name));
            info.appendChild(el('span', 'mono small', row.spec));
            if (row.command) info.appendChild(el('span', 'mono small dim', row.command));
            info.appendChild(el('span', 'small dim',
                `${row.due ? 'due' : row.next}${row.lastResult ? ` · last: ${row.lastResult}` : ''}`));
            line.appendChild(info);
            const actions = el('div', 'ta-actions');
            const passBtn = el('button', 'btn btn-success', 'Record pass');
            passBtn.addEventListener('click', () => markRun(row.id, 'passed'));
            const failBtn = el('button', 'btn btn-danger', 'Record fail');
            failBtn.addEventListener('click', () => markRun(row.id, 'failed'));
            const delBtn = el('button', 'btn btn-warn', 'Remove');
            delBtn.addEventListener('click', () => removeSchedule(row.id));
            actions.append(passBtn, failBtn, delBtn);
            line.appendChild(actions);
            body.appendChild(line);
        });
        body.append(schedForm, schedErrors);
    }

    function renderFixtures(sets) {
        const body = sections.fixtures;
        clear(body);
        const info = fixtureSummary(sets);
        if (!info.sets) {
            body.appendChild(emptyRow('No test-data/, fixtures/ or tests/fixtures/ directories found.'));
            return;
        }
        body.appendChild(el('div', 'dim small',
            `${info.sets} set(s) · ${info.files} file(s) · ${formatBytes(info.bytes)}`));
        (sets || []).forEach((set) => {
            body.appendChild(el('div', 'ta-line mono small',
                `${set.dir} — ${set.files} file(s)${set.truncated ? ' (list truncated)' : ''}`));
        });
    }

    function render(summary) {
        headline.textContent = summary && summary.project
            ? `${summary.project}${summary.activeEnv ? ` · ${summary.activeEnv}` : ''}`
            : 'Select a project';
        const cov = summary ? summary.coverage : null;
        const covHead = coverageHeadline(cov, state.threshold);
        const clock = new Date().toTimeString().slice(0, 8);
        headline.appendChild(el('span', 'ta-stamp dim small', `  (${covHead.text}, refreshed ${clock})`));
        renderCoverage(cov);
        renderPerf(summary || {});
        renderVisual(summary && summary.visual);
        renderAnalytics(summary && summary.analytics);
        renderMatrix(summary || { matrix: [], matrixProgress: {} });
        renderSchedules(summary && summary.schedules);
        renderFixtures(summary && summary.fixtures);
    }

    async function refresh() {
        const index = selected();
        if (!api) {
            render(null);
            showErrors(schedErrors, 'Backend not available (run inside the app)');
            return;
        }
        const project = state.project || null;
        if (index < 0 || !project) {
            render(null);
            return;
        }
        if (state.loading) return;
        state.loading = true;
        refreshBtn.disabled = true;
        try {
            const summary = await api.getTestAdvSummary(index, state.threshold);
            render(summary);
        } catch (err) {
            render(null);
            headline.textContent = `Error loading testing data: ${(err && err.message) || err}`;
        } finally {
            state.loading = false;
            refreshBtn.disabled = false;
        }
    }

    async function markRun(id, result) {
        if (!api) return;
        const index = selected();
        try {
            const errs = await api.markTestScheduleRun(index, id, result);
            showErrors(schedErrors, errs);
            await refresh();
        } catch (err) {
            showErrors(schedErrors, (err && err.message) || String(err));
        }
    }

    async function removeSchedule(id) {
        if (!api) return;
        const index = selected();
        try {
            const errs = await api.deleteTestSchedule(index, id);
            showErrors(schedErrors, errs);
            await refresh();
        } catch (err) {
            showErrors(schedErrors, (err && err.message) || String(err));
        }
    }

    async function addSchedule() {
        if (!api) return;
        const index = selected();
        const name = schedName.value;
        const spec = schedSpec.value;
        const local = validateSchedule(name, spec);
        if (local.length) {
            showErrors(schedErrors, local);
            return;
        }
        try {
            const errs = await api.saveTestSchedule(index, {
                id: '', name, spec, command: schedCommand.value, enabled: true,
            });
            if (!errs || !errs.length) {
                schedName.value = '';
                schedSpec.value = '';
                schedCommand.value = '';
            }
            showErrors(schedErrors, errs);
            await refresh();
        } catch (err) {
            showErrors(schedErrors, (err && err.message) || String(err));
        }
    }

    async function recordRun() {
        if (!api) return;
        const index = selected();
        const passed = parseInt(runPassed.value, 10);
        const failed = parseInt(runFailed.value, 10);
        if (!Number.isFinite(passed) || !Number.isFinite(failed) || passed < 0 || failed < 0) {
            showErrors(runErrors, 'Passed/failed must be non-negative numbers');
            return;
        }
        try {
            const errs = await api.appendTestRun(index, {
                startedAt: new Date().toISOString(),
                durationMs: 0,
                passed,
                failed,
                skipped: 0,
                browser: (runBrowser.value || 'chromium').trim(),
                env: (runEnv.value || 'dev').trim(),
                suite: '',
                failedTests: [],
            });
            showErrors(runErrors, errs);
            if (!errs || !errs.length) {
                runPassed.value = '0';
                runFailed.value = '0';
            }
            await refresh();
        } catch (err) {
            showErrors(runErrors, (err && err.message) || String(err));
        }
    }

    refreshBtn.addEventListener('click', () => refresh());
    thresholdInput.addEventListener('change', () => {
        const value = Number(thresholdInput.value);
        state.threshold = Number.isFinite(value) && value > 0 && value <= 100 ? value : DEFAULT_THRESHOLD;
        thresholdInput.value = String(state.threshold);
        refresh();
    });
    baselineBtn.addEventListener('click', async () => {
        if (!api) return;
        const index = selected();
        const confirmFn = ctx && ctx.messageDialog && ctx.messageDialog.confirm;
        try {
            if (confirmFn) {
                const ok = await ctx.messageDialog.confirm({
                    title: 'Promote baseline',
                    message: 'Copy current screenshots over the visual baseline?',
                    confirmLabel: 'Promote',
                });
                if (!ok) return;
            }
            await api.promoteVisualBaseline(index);
            await refresh();
        } catch (err) {
            headline.textContent = `Promote failed: ${(err && err.message) || err}`;
        }
    });
    schedAdd.addEventListener('click', addSchedule);
    runAdd.addEventListener('click', recordRun);

    render(null);

    return {
        onProjectChanged(project) {
            state.project = project || null;
            return refresh();
        },
        refresh,
        // Expuesto para tests/automatización.
        setThreshold(value) {
            state.threshold = value;
            thresholdInput.value = String(value);
        },
        getState() {
            return { ...state };
        },
    };
}
