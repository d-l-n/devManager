// Helpers puros del panel de testing avanzado (Issue #70). Sin dependencias ni
// acceso al DOM: todo lo que se puede testear vive aquí y el panel solo pinta.

// ---- números ----

export function round1(n) {
    const v = Number(n);
    if (!Number.isFinite(v)) return 0;
    return Math.round(v * 10) / 10;
}

export function formatPct(n) {
    return `${round1(n)}%`;
}

export function formatMs(n) {
    const v = Number(n) || 0;
    if (v === 0) return '—';
    if (v < 1000) return `${round1(v)} ms`;
    return `${round1(v / 1000)} s`;
}

export function formatSeconds(n) {
    const v = Number(n) || 0;
    if (v <= 0) return '—';
    if (v < 60) return `${round1(v)}s`;
    const m = Math.floor(v / 60);
    const s = Math.round(v % 60);
    return `${m}m ${s}s`;
}

export function formatBytes(n) {
    const v = Number(n) || 0;
    if (v < 1024) return `${v} B`;
    if (v < 1024 * 1024) return `${round1(v / 1024)} KB`;
    if (v < 1024 * 1024 * 1024) return `${round1(v / (1024 * 1024))} MB`;
    return `${round1(v / (1024 * 1024 * 1024))} GB`;
}

export function formatRelative(date, now = new Date()) {
    if (!date) return '—';
    const t = date instanceof Date ? date.getTime() : new Date(date).getTime();
    if (!Number.isFinite(t)) return '—';
    const nowMs = now instanceof Date ? now.getTime() : new Date(now).getTime();
    const diff = t - nowMs;
    const abs = Math.abs(diff);
    let value;
    if (abs < 60_000) value = `${Math.max(1, Math.round(abs / 1000))}s`;
    else if (abs < 3_600_000) value = `${Math.round(abs / 60_000)}m`;
    else if (abs < 86_400_000) value = `${Math.round(abs / 3_600_000)}h`;
    else value = `${Math.round(abs / 86_400_000)}d`;
    return diff >= 0 ? `in ${value}` : `${value} ago`;
}

// ---- cobertura ----

// coverageClass clasifica un porcentaje contra el umbral (para colores).
export function coverageClass(pct, threshold) {
    const t = Number(threshold) || 0;
    const v = Number(pct) || 0;
    if (v >= Math.max(t, 1)) return 'good';
    if (v >= t - 10) return 'warn';
    return 'bad';
}

export function coverageRows(summary) {
    if (!summary) return [];
    const metric = (label, m) => ({
        label,
        pct: round1((m && m.pct) || 0),
        covered: (m && m.covered) || 0,
        total: (m && m.total) || 0,
        empty: !m || (m.total || 0) === 0,
    });
    return [
        metric('Lines', summary.lines),
        metric('Statements', summary.statements),
        metric('Functions', summary.functions),
        metric('Branches', summary.branches),
    ];
}

export function coverageHeadline(coverage, threshold) {
    if (!coverage || !coverage.available) {
        return { text: 'No coverage report found', state: 'empty' };
    }
    const pct = round1((coverage.summary && coverage.summary.lines && coverage.summary.lines.pct) || 0);
    const state = coverageClass(pct, threshold);
    const verdict = coverage.summary && coverage.summary.pass ? 'meets threshold' : `below threshold (${threshold}%)`;
    return { text: `${pct}% lines — ${verdict}`, state };
}

export function worstFiles(coverage) {
    const worst = (coverage && coverage.worst) || [];
    return worst.map((f) => ({
        path: f.path,
        pct: round1((f.lines && f.lines.pct) || 0),
        covered: (f.lines && f.lines.covered) || 0,
        total: (f.lines && f.lines.total) || 0,
    }));
}

// ---- performance ----

export function perfRows(metrics) {
    if (!metrics || !metrics.source) return [];
    return [
        { label: 'Requests', value: String(metrics.requests || 0) },
        { label: 'RPS', value: metrics.rps ? round1(metrics.rps).toString() : '—' },
        { label: 'Avg', value: formatMs(metrics.avgMs) },
        { label: 'p50', value: formatMs(metrics.p50Ms) },
        { label: 'p90', value: formatMs(metrics.p90Ms) },
        { label: 'p95', value: formatMs(metrics.p95Ms) },
        { label: 'p99', value: formatMs(metrics.p99Ms) },
        { label: 'Errors', value: formatPct(metrics.errorRate || 0) },
        { label: 'Duration', value: formatSeconds(metrics.durationSeconds) },
        { label: 'VUs', value: metrics.vus ? String(metrics.vus) : '—' },
    ];
}

export function thresholdRows(checks) {
    return (checks || []).map((c) => ({
        name: c.name,
        pass: !!c.pass,
        text: `${c.name}: ${round1(c.value)} ${c.pass ? '≤' : 'vs'} ${round1(c.limit)}`,
    }));
}

// ---- analytics ----

export function flakyRows(analytics) {
    return ((analytics && analytics.flakyTests) || []).map((f) => ({
        name: f.name,
        failures: f.failures,
        runs: f.runs,
        rate: round1(f.rate),
    }));
}

// trendBars normaliza el trend a barras 0..100 para pintar sin SVG.
export function trendBars(analytics) {
    const trend = (analytics && analytics.trend) || [];
    return trend.map((p) => ({
        passRate: round1(p.passRate),
        height: Math.max(2, Math.min(100, Math.round(p.passRate))),
        runs: p.runs,
        label: `${round1(p.passRate)}% (${p.runs} run${p.runs === 1 ? '' : 's'})`,
    }));
}

export function analyticsHeadline(analytics) {
    if (!analytics || !analytics.totalRuns) {
        return { text: 'No test runs recorded yet', detail: '' };
    }
    const detail = `avg ${formatSeconds((analytics.avgDurationMs || 0) / 1000)} · ${analytics.stableRuns}/${analytics.totalRuns} clean runs`;
    return { text: `${round1(analytics.passRate)}% pass rate`, detail };
}

// ---- matriz ----

export function matrixGrid(cells) {
    const list = cells || [];
    const browsers = [];
    const envs = [];
    for (const c of list) {
        if (!browsers.includes(c.browser)) browsers.push(c.browser);
        if (!envs.includes(c.env)) envs.push(c.env);
    }
    return {
        browsers,
        envs,
        rows: browsers.map((browser) => ({
            browser,
            cells: envs.map((env) => list.find((c) => c.browser === browser && c.env === env) || {
                browser, env, status: 'pending', runs: 0,
            }),
        })),
    };
}

export function statusLabel(status) {
    switch (status) {
        case 'passed': return 'pass';
        case 'failed': return 'fail';
        case 'changed': return 'changed';
        case 'new': return 'new';
        case 'missing': return 'missing';
        case 'same': return 'same';
        default: return 'pending';
    }
}

export function matrixHeadline(progress) {
    const p = progress || { done: 0, total: 0, passRate: 0 };
    if (!p.total) return 'No browsers/environments configured';
    return `${p.done}/${p.total} combinations run · ${round1(p.passRate)}% passing`;
}

// ---- regresión visual ----

export function visualHeadline(visual) {
    if (!visual || !visual.available) {
        return { text: 'No screenshots found in test-results/visual', state: 'empty' };
    }
    const state = visual.changed > 0 ? 'bad' : visual.new > 0 || visual.missing > 0 ? 'warn' : 'good';
    const parts = [`${visual.total} shots`];
    if (visual.changed) parts.push(`${visual.changed} changed`);
    if (visual.new) parts.push(`${visual.new} new`);
    if (visual.missing) parts.push(`${visual.missing} missing`);
    return { text: parts.join(' · '), state };
}

export function visualRows(visual) {
    return ((visual && visual.results) || [])
        .filter((r) => r.status === 'changed' || r.status === 'missing')
        .map((r) => ({
            name: r.name,
            status: r.status,
            detail: r.diff && r.diff.sizeMismatch
                ? 'size mismatch'
                : r.diff
                    ? `${formatPct(r.diff.percent)} diff`
                    : '',
        }));
}

// ---- schedules ----

export function scheduleRows(schedules, now = new Date()) {
    return (schedules || []).map((s) => ({
        id: s.id,
        name: s.name,
        spec: s.spec,
        command: s.command || '',
        enabled: !!s.enabled,
        lastResult: s.lastResult || '',
        next: s.nextRun ? formatRelative(new Date(s.nextRun), now) : '—',
        due: !!s.nextRun && new Date(s.nextRun).getTime() <= (now instanceof Date ? now.getTime() : new Date(now).getTime()),
    }));
}

// validateSchedule replica la validación del backend para dar feedback inmediato.
export function validateSchedule(name, spec) {
    const errs = [];
    if (!String(name || '').trim()) errs.push('Name is required');
    const raw = String(spec || '').trim().toLowerCase();
    const fields = raw.split(/\s+/).filter(Boolean);
    if (!raw) {
        errs.push('Spec is required');
        return errs;
    }
    if (fields[0] === 'every') {
        if (fields.length !== 2 || !/^\d+(m|h|s)$/.test(fields[1] || '')) {
            errs.push('Use "every 15m", "every 2h"');
        } else if (fields[1].endsWith('s') && !fields[1].endsWith('ms')) {
            errs.push('Minimum interval is 1m');
        }
    } else if (fields[0] === 'daily') {
        if (fields.length !== 2 || !/^([01]?\d|2[0-3]):[0-5]\d$/.test(fields[1] || '')) {
            errs.push('Use "daily HH:MM"');
        }
    } else if (fields[0] === 'weekly') {
        if (fields.length !== 3 || !/^([01]?\d|2[0-3]):[0-5]\d$/.test(fields[2] || '')) {
            errs.push('Use "weekly mon,wed 09:00"');
        }
    } else {
        errs.push('Spec must start with "every", "daily" or "weekly"');
    }
    return errs;
}

// ---- fixtures ----

export function fixtureSummary(sets) {
    const list = sets || [];
    return {
        sets: list.length,
        files: list.reduce((acc, s) => acc + (s.files || 0), 0),
        bytes: list.reduce((acc, s) => acc + (s.bytes || 0), 0),
    };
}
