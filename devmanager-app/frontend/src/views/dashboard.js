// Vista Dashboard global (Issue #64): una card por proyecto con estado,
// puerto, uptime actual y CPU/RAM en vivo + resumen consolidado (running/
// stopped/puertos/tests) + timelines de uptime 24h + sección Performance
// (CPU/RAM 24h de los samples) + Alertas consolidadas + búsqueda y filtros
// + toggles de secciones persistidos en Settings (dashboard_section.*).
// Frontend-only: reúne GetProjects + GetServerStatus + GetMonitorData +
// GetDashboardHistory + GetPlaywrightStatus. Patrón monitor.js: poll solo
// mientras es visible.
import { showToast } from '../widgets/toast.js';
import { activeEnvOf, effectiveServer } from '../envs.js';

const POLL_MS = 5000;
const HISTORY_WINDOW_MS = 24 * 60 * 60 * 1000;
const SPARK_W = 240;
const SPARK_H = 10;
const SPARK_BUCKETS = 120; // 12 min por bucket

// Secciones del dashboard con toggle de visibilidad (Settings).
const SECTIONS = ['projects', 'alerts', 'uptime', 'perf'];

const SVG_NS = 'http://www.w3.org/2000/svg';

export function mount(ctx) {
    const { $, api, events } = ctx;

    let visible = false;
    let timer = null;

    // ---- Estado de filtros/búsqueda ----
    const filterState = { query: '', filter: 'all' };

    // ---- Estado de tests por índice de proyecto (GetPlaywrightStatus) ----
    let pwByIndex = {};

    // Snapshot del último render para re-filtrar sin re-render.
    let currentProjects = [];
    let currentStatuses = [];

    // ---- Visibilidad de secciones (paridad tolerante: ausente → visible) ----
    const sections = Object.fromEntries(SECTIONS.map((s) => [s, true]));

    function fmtUptime(seconds) {
        if (!seconds || seconds <= 0) return '';
        const s = Math.floor(seconds);
        if (s >= 3600) return `up ${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
        if (s >= 60) return `up ${Math.floor(s / 60)}m ${s % 60}s`;
        return `up ${s}s`;
    }

    function cpuClass(cpu) {
        if (cpu >= 80) return 'dash-chip err';
        if (cpu >= 50) return 'dash-chip warn';
        return 'dash-chip';
    }

    // testChipClass mapea estado de Playwright a clase del chip.
    function testChipClass(pwState) {
        if (pwState === 'passed') return 'dash-chip test-ok';
        if (pwState === 'failed') return 'dash-chip err';
        if (pwState === 'error') return 'dash-chip warn';
        return 'dash-chip';
    }

    // testChipLabel: texto compacto para el chip de tests.
    function testChipLabel(pwState) {
        switch (pwState) {
            case 'passed': return 'tests passed';
            case 'failed': return 'tests failed';
            case 'error': return 'test error';
            case 'running': return 'tests running';
            case 'starting': return 'tests starting';
            default: return '';
        }
    }

    function cardEl(project, index, status, usage) {
        const card = document.createElement('div');
        card.className = 'dash-card';

        const info = document.createElement('div');
        info.className = 'dash-card-info';

        const name = document.createElement('span');
        name.className = 'dash-card-name';
        name.textContent = project.name;
        info.appendChild(name);

        const state = (status && status.state) || 'stopped';
        const badge = document.createElement('span');
        badge.className = `badge ${state}`;
        badge.textContent = state;
        info.appendChild(badge);

        const eff = effectiveServer(project);
        const port = document.createElement('span');
        port.className = 'dash-card-port mono';
        port.textContent = eff && eff.port ? `:${eff.port}` : '';
        info.appendChild(port);

        // Fase 1 #67: sufijo ·env en la card.
        const envName = activeEnvOf(project);
        if (envName) {
            const envChip = document.createElement('span');
            envChip.className = 'dash-chip';
            envChip.textContent = `·${envName}`;
            info.appendChild(envChip);
        }

        // Uptime actual (paridad refreshStatus del project view, formato compacto).
        const up = fmtUptime(status && status.uptimeSeconds);
        if (up) {
            const upEl = document.createElement('span');
            upEl.className = 'dash-card-uptime dim';
            upEl.textContent = up;
            info.appendChild(upEl);
        }

        // CPU/RAM en vivo del árbol del server (GetMonitorData.resRows match por nombre).
        if (state === 'running' && usage) {
            const cpu = document.createElement('span');
            cpu.className = cpuClass(usage.cpu);
            cpu.textContent = `CPU ${Math.round(usage.cpu)}%`;
            info.appendChild(cpu);
            const ram = document.createElement('span');
            ram.className = 'dash-chip';
            ram.textContent = `RAM ${Math.round(usage.rss)} MB`;
            info.appendChild(ram);
        }

        // Resultado de tests (GetPlaywrightStatus): passed/failed/running/...
        const pwState = pwByIndex[index];
        const chipLabel = testChipLabel(pwState);
        if (chipLabel) {
            const chip = document.createElement('span');
            chip.className = testChipClass(pwState);
            chip.textContent = chipLabel;
            info.appendChild(chip);
        }

        // Click en la card → seleccionar el proyecto y volver a Project view.
        card.addEventListener('click', () => ctx.selectProject(index));
        card.appendChild(info);

        const actions = document.createElement('div');
        actions.className = 'dash-card-actions';
        const btnStart = document.createElement('button');
        btnStart.className = 'btn btn-small';
        btnStart.textContent = 'Start';
        btnStart.disabled = state === 'running' || state === 'starting';
        btnStart.addEventListener('click', (e) => { e.stopPropagation(); api.startServer(index); });
        const btnStop = document.createElement('button');
        btnStop.className = 'btn btn-small';
        btnStop.textContent = 'Stop';
        btnStop.disabled = state === 'stopped';
        btnStop.addEventListener('click', (e) => { e.stopPropagation(); api.stopServer(index); });
        actions.appendChild(btnStart);
        actions.appendChild(btnStop);
        card.appendChild(actions);
        return card;
    }

    // matchCard decide si una card pasa el filtro actual (búsqueda + chip).
    function matchCard(project, status, pwState) {
        const q = filterState.query.trim().toLowerCase();
        if (q && !project.name.toLowerCase().includes(q)) return false;
        const state = (status && status.state) || 'stopped';
        switch (filterState.filter) {
            case 'running': return state === 'running' || state === 'starting';
            case 'stopped': return state === 'stopped';
            case 'failed': return pwState === 'failed' || pwState === 'error';
            default: return true;
        }
    }

    // applyFilter oculta las cards que no pasan el filtro sin re-render.
    // Recalcula matchCard con el snapshot del último render (data-match
    // quedaría desactualizado respecto a query/filter actuales).
    function applyFilter() {
        const cards = $('dash-cards');
        let shown = 0;
        Array.from(cards.children).forEach((card) => {
            const i = Number(card.dataset.index);
            const ok = matchCard(currentProjects[i], currentStatuses[i], pwByIndex[i]);
            card.style.display = ok ? '' : 'none';
            if (ok) shown++;
        });
        $('dash-filter-empty').hidden = shown > 0;
    }

    // uptimeSvg renderiza la timeline 24h: bucket de 12 min, verde running,
    // gris stopped; sin datos = track. Devuelve {svg, covered, up}.
    function uptimeSvg(samples) {
        const now = Date.now();
        const start = now - HISTORY_WINDOW_MS;
        const bucketMs = HISTORY_WINDOW_MS / SPARK_BUCKETS;
        const vals = new Array(SPARK_BUCKETS).fill(-1); // -1 = sin datos
        (samples || []).forEach((s) => {
            const ts = (s.ts || 0) * 1000;
            if (ts < start || ts > now) return;
            const b = Math.min(SPARK_BUCKETS - 1, Math.floor((ts - start) / bucketMs));
            vals[b] = s.running ? 1 : 0;
        });

        const svg = document.createElementNS(SVG_NS, 'svg');
        svg.setAttribute('viewBox', `0 0 ${SPARK_W} ${SPARK_H}`);
        svg.setAttribute('preserveAspectRatio', 'none');
        svg.setAttribute('class', 'dash-spark');
        const track = document.createElementNS(SVG_NS, 'rect');
        track.setAttribute('x', '0');
        track.setAttribute('y', '0');
        track.setAttribute('width', String(SPARK_W));
        track.setAttribute('height', String(SPARK_H));
        track.setAttribute('rx', '2');
        track.setAttribute('class', 'dash-spark-track');
        svg.appendChild(track);
        vals.forEach((v, i) => {
            if (v < 0) return;
            const rect = document.createElementNS(SVG_NS, 'rect');
            rect.setAttribute('x', ((i * SPARK_W) / SPARK_BUCKETS).toFixed(2));
            rect.setAttribute('y', '0');
            rect.setAttribute('width', (SPARK_W / SPARK_BUCKETS - 0.4).toFixed(2));
            rect.setAttribute('height', String(SPARK_H));
            rect.setAttribute('class', v === 1 ? 'dash-spark-up' : 'dash-spark-down');
            svg.appendChild(rect);
        });

        const covered = vals.filter((v) => v >= 0).length;
        const up = vals.filter((v) => v === 1).length;
        return { svg, covered, up };
    }

    function uptimeRowEl(history) {
        const { svg, covered, up } = uptimeSvg(history.samples);
        if (covered === 0) return null;

        const row = document.createElement('div');
        row.className = 'dash-uptime-row';

        const name = document.createElement('span');
        name.className = 'dash-uptime-name';
        name.textContent = history.name;
        row.appendChild(name);

        row.appendChild(svg);

        const pct = Math.round((up / covered) * 100);
        const label = document.createElement('span');
        label.className = 'dash-uptime-pct dim';
        label.textContent = `${pct}% up`;
        row.appendChild(label);
        return row;
    }

    // ---- Performance (24h): CPU/RAM desde los samples del sampler ----

    // cpuSpark renderiza barras de CPU 0-100% por bucket de 12 min (max del
    // bucket); bucket sin datos = track. Devuelve svg.
    function cpuSpark(samples) {
        const now = Date.now();
        const start = now - HISTORY_WINDOW_MS;
        const bucketMs = HISTORY_WINDOW_MS / SPARK_BUCKETS;
        const vals = new Array(SPARK_BUCKETS).fill(-1);
        (samples || []).forEach((s) => {
            const ts = (s.ts || 0) * 1000;
            if (ts < start || ts > now) return;
            if (!(s.cpu > 0)) return;
            const b = Math.min(SPARK_BUCKETS - 1, Math.floor((ts - start) / bucketMs));
            if (s.cpu > vals[b]) vals[b] = s.cpu;
        });

        const svg = document.createElementNS(SVG_NS, 'svg');
        svg.setAttribute('viewBox', `0 0 ${SPARK_W} ${SPARK_H}`);
        svg.setAttribute('preserveAspectRatio', 'none');
        svg.setAttribute('class', 'dash-spark');
        const track = document.createElementNS(SVG_NS, 'rect');
        track.setAttribute('x', '0');
        track.setAttribute('y', '0');
        track.setAttribute('width', String(SPARK_W));
        track.setAttribute('height', String(SPARK_H));
        track.setAttribute('rx', '2');
        track.setAttribute('class', 'dash-spark-track');
        svg.appendChild(track);
        vals.forEach((v, i) => {
            if (v < 0) return;
            const h = Math.max(1, Math.round((Math.min(100, v) / 100) * SPARK_H));
            const rect = document.createElementNS(SVG_NS, 'rect');
            rect.setAttribute('x', ((i * SPARK_W) / SPARK_BUCKETS).toFixed(2));
            rect.setAttribute('y', String(SPARK_H - h));
            rect.setAttribute('width', (SPARK_W / SPARK_BUCKETS - 0.4).toFixed(2));
            rect.setAttribute('height', String(h));
            rect.setAttribute('class', v >= 80 ? 'dash-spark-cpu-hot' : 'dash-spark-cpu');
            svg.appendChild(rect);
        });
        return svg;
    }

    // perfRowEl: stats CPU/RAM 24h de un proyecto. Sin samples con CPU → null.
    function perfRowEl(history) {
        const withCpu = (history.samples || []).filter((s) => s.cpu > 0);
        if (withCpu.length === 0) return null;

        const cpuVals = withCpu.map((s) => s.cpu);
        const avgCpu = cpuVals.reduce((a, b) => a + b, 0) / cpuVals.length;
        const maxCpu = Math.max(...cpuVals);
        const rssVals = (history.samples || []).filter((s) => s.rss > 0).map((s) => s.rss);
        const avgRss = rssVals.length ? rssVals.reduce((a, b) => a + b, 0) / rssVals.length : 0;

        const row = document.createElement('div');
        row.className = 'dash-perf-row';

        const name = document.createElement('span');
        name.className = 'dash-perf-name';
        name.textContent = history.name;
        row.appendChild(name);

        row.appendChild(cpuSpark(history.samples));

        const label = document.createElement('span');
        label.className = 'dash-perf-stats dim';
        const rssPart = avgRss > 0 ? ` · RAM ~${Math.round(avgRss)} MB` : '';
        label.textContent = `avg ${Math.round(avgCpu)}% · peak ${Math.round(maxCpu)}%${rssPart}`;
        row.appendChild(label);
        return row;
    }

    // ---- Alertas consolidadas ----

    // buildAlerts reúne problemas activos: conflictos de puerto (foreign),
    // tests fallidos y errores de test.
    function buildAlerts(projects, portRows) {
        const alerts = [];
        (portRows || []).forEach((r) => {
            if (r.state !== 'foreign') return;
            const owner = [r.ownerName, r.ownerPID ? `PID ${r.ownerPID}` : ''].filter(Boolean).join(' ');
            alerts.push({
                level: 'err',
                text: `Port conflict :${r.port} — foreign process${owner ? ` (${owner})` : ''}`,
            });
        });
        projects.forEach((p, i) => {
            const pw = pwByIndex[i];
            if (pw === 'failed') alerts.push({ level: 'err', text: `Tests failed: ${p.name}` });
            else if (pw === 'error') alerts.push({ level: 'warn', text: `Test error: ${p.name}` });
        });
        return alerts;
    }

    function alertsEl(alerts) {
        const wrap = document.createElement('div');
        wrap.className = 'dash-alerts';
        alerts.forEach((a) => {
            const row = document.createElement('div');
            row.className = `dash-alert dash-alert-${a.level}`;
            const chip = document.createElement('span');
            chip.className = `dash-chip ${a.level}`;
            chip.textContent = a.level === 'err' ? 'alert' : 'warn';
            row.appendChild(chip);
            const text = document.createElement('span');
            text.className = 'dash-alert-text';
            text.textContent = a.text;
            row.appendChild(text);
            wrap.appendChild(row);
        });
        return wrap;
    }

    // ---- Secciones (toggles persistidos en Settings) ----

    function applySections() {
        document.querySelectorAll('#dashboard-view [data-dash-section]').forEach((group) => {
            const name = group.getAttribute('data-dash-section');
            group.hidden = !sections[name];
        });
        document.querySelectorAll('[data-dash-section-toggle]').forEach((cb) => {
            cb.checked = !!sections[cb.getAttribute('data-dash-section-toggle')];
        });
    }

    function onSectionToggle(name, value) {
        sections[name] = value;
        applySections();
        // Persistencia tolerante: si SetSetting falla, el toggle local queda.
        Promise.resolve(api.setSetting(`dashboard_section.${name}`, value ? 'true' : 'false'))
            .catch(() => {});
    }

    async function doRefresh() {
        const projects = await api.getProjects();
        const cards = $('dash-cards');
        cards.innerHTML = '';

        if (!projects || projects.length === 0) {
            $('dash-empty').hidden = false;
            $('dash-filter-empty').hidden = true;
            $('dash-summary').textContent = '';
            $('dash-start-all').disabled = true;
            $('dash-stop-all').disabled = true;
            $('dash-tests-all').disabled = true;
            $('dash-uptimes').innerHTML = '';
            $('dash-uptimes-empty').hidden = true;
            $('dash-perf').innerHTML = '';
            $('dash-perf-empty').hidden = true;
            $('dash-alerts').innerHTML = '';
            $('dash-alerts-empty').hidden = true;
            return;
        }

        $('dash-empty').hidden = true;
        $('dash-start-all').disabled = false;
        $('dash-stop-all').disabled = false;
        $('dash-tests-all').disabled = false;

        const statuses = await Promise.all(projects.map(async (_, i) => {
            try { return await api.getServerStatus(i); } catch { return { state: 'stopped' }; }
        }));

        // Estado de tests por proyecto (idle/off → sin chip).
        pwByIndex = {};
        const pwResults = await Promise.all(projects.map(async (_, i) => {
            try { return await api.getPlaywrightStatus(i); } catch { return null; }
        }));
        pwResults.forEach((r, i) => {
            if (r && r.state && r.state !== 'idle' && r.state !== 'off') pwByIndex[i] = r.state;
        });

        // CPU/RAM en vivo + resumen de puertos (una sola llamada al monitor).
        const usageByName = {};
        let portRows = [];
        try {
            const mon = await api.getMonitorData();
            ((mon && mon.resRows) || []).forEach((r) => { usageByName[r.name] = r; });
            portRows = (mon && mon.portRows) || [];
        } catch { /* resumen sin puertos si el monitor falla */ }
        const conflicts = portRows.filter((r) => r.state === 'foreign');
        const portsInUse = portRows.filter((r) => r.state === 'ours' || r.state === 'foreign').length;

        statuses.forEach((status, i) => {
            const card = cardEl(projects[i], i, status, usageByName[projects[i].name]);
            card.dataset.index = String(i);
            cards.appendChild(card);
        });
        currentProjects = projects;
        currentStatuses = statuses;
        applyFilter();

        // Resumen: servidores + puertos + tests agregados (pass/fail).
        const running = statuses.filter((s) => s.state === 'running').length;
        const stopped = statuses.length - running;
        const pwVals = Object.values(pwByIndex);
        const testsPassed = pwVals.filter((s) => s === 'passed').length;
        const testsFailed = pwVals.filter((s) => s === 'failed' || s === 'error').length;
        const parts = [`${running} running`, `${stopped} stopped`, `${portsInUse} ports in use`];
        if (conflicts.length > 0) parts.push(`${conflicts.length} conflict(s)`);
        if (testsPassed + testsFailed > 0) parts.push(`tests: ${testsPassed} passed · ${testsFailed} failed`);
        $('dash-summary').textContent = parts.join(' · ');

        // Alertas consolidadas.
        const alerts = buildAlerts(projects, portRows);
        const alertsWrap = $('dash-alerts');
        alertsWrap.innerHTML = '';
        if (alerts.length > 0) {
            alertsWrap.appendChild(alertsEl(alerts));
            $('dash-alerts-empty').hidden = true;
        } else {
            $('dash-alerts-empty').hidden = false;
        }

        // Uptime (24h) + Performance (24h) comparten el mismo historial.
        let history = [];
        try { history = (await api.getDashboardHistory()) || []; } catch { /* sin historial */ }

        const uptimes = $('dash-uptimes');
        uptimes.innerHTML = '';
        let rendered = 0;
        history.forEach((h) => {
            const row = uptimeRowEl(h);
            if (row) { uptimes.appendChild(row); rendered++; }
        });
        $('dash-uptimes-empty').hidden = rendered > 0;

        const perf = $('dash-perf');
        perf.innerHTML = '';
        let perfRendered = 0;
        history.forEach((h) => {
            const row = perfRowEl(h);
            if (row) { perf.appendChild(row); perfRendered++; }
        });
        $('dash-perf-empty').hidden = perfRendered > 0;
    }

    // Serialización de refresh: eventos server:state pueden dispararlo mientras
    // otro refresh está awaiting; sin guard se duplican cards (innerHTML ''
    // + await + append intercalados). pending garantiza estado final fresco.
    let refreshing = false;
    let pending = false;
    async function refresh() {
        if (refreshing) { pending = true; return; }
        refreshing = true;
        try {
            await doRefresh();
        } finally {
            refreshing = false;
            if (pending) { pending = false; refresh(); }
        }
    }

    function updateTimer() {
        if (visible && timer === null) {
            timer = setInterval(refresh, POLL_MS);
        } else if (!visible && timer !== null) {
            clearInterval(timer);
            timer = null;
        }
    }

    // ---- Búsqueda y filtros ----

    $('dash-search').addEventListener('input', () => {
        filterState.query = $('dash-search').value;
        applyFilter();
    });

    $('dash-filters').addEventListener('click', (e) => {
        const chip = e.target.closest('[data-dash-filter]');
        if (!chip) return;
        filterState.filter = chip.getAttribute('data-dash-filter');
        document.querySelectorAll('#dash-filters .filter-chip').forEach((c) => {
            const active = c === chip;
            c.classList.toggle('active', active);
            c.setAttribute('aria-pressed', active ? 'true' : 'false');
        });
        applyFilter();
    });

    // ---- Toggles de secciones ----

    $('dash-layout').addEventListener('click', () => {
        const menu = $('dash-layout-menu');
        const open = menu.hidden;
        menu.hidden = !open;
        $('dash-layout').setAttribute('aria-expanded', open ? 'true' : 'false');
    });

    // Click fuera cierra el menú (patrón contextmenu).
    document.addEventListener('click', (e) => {
        const menu = $('dash-layout-menu');
        if (menu.hidden) return;
        if (e.target.closest('.dash-layout-wrap')) return;
        menu.hidden = true;
        $('dash-layout').setAttribute('aria-expanded', 'false');
    });

    $('dash-layout-menu').addEventListener('change', (e) => {
        const cb = e.target.closest('[data-dash-section-toggle]');
        if (!cb) return;
        onSectionToggle(cb.getAttribute('data-dash-section-toggle'), cb.checked);
    });

    // Sync externo de settings (otra vista cambió la sección).
    events().EventsOn('settings:changed', (payload) => {
        const key = (payload && payload.key) || '';
        if (!key.startsWith('dashboard_section.')) return;
        const name = key.slice('dashboard_section.'.length);
        sections[name] = payload.value === 'true';
        applySections();
    });

    // Carga inicial de visibilidad persistida (tolerante: sin settings → todo visible).
    Promise.resolve(api.getSettings())
        .then((s) => {
            const saved = (s && s.dashboard_sections) || {};
            SECTIONS.forEach((name) => {
                if (name in saved) sections[name] = !!saved[name];
            });
            applySections();
        })
        .catch(() => {});

    // Refresco inmediato ante cambios del backend (server:state llega al
    // start/stop individual; projects:changed al añadir/quitar proyectos).
    events().EventsOn('server:state', () => { if (visible) refresh(); });
    events().EventsOn('projects:changed', () => { if (visible) refresh(); });

    $('dash-refresh').addEventListener('click', refresh);

    $('dash-start-all').addEventListener('click', async () => {
        const projects = await api.getProjects();
        projects.forEach(async (_, i) => {
            try {
                const st = await api.getServerStatus(i);
                if (st.state === 'stopped') await api.startServer(i);
            } catch { /* siguiente poll corrige */ }
        });
        showToast('Dashboard', 'Starting all stopped servers', 'info');
    });

    $('dash-stop-all').addEventListener('click', async () => {
        const ok = await ctx.messageDialog.confirm({
            title: 'Stop all servers',
            message: 'Stop every running server? In-flight requests will be interrupted.',
            confirmLabel: 'Stop all',
            destructive: true,
        });
        if (!ok) return;
        const projects = await api.getProjects();
        projects.forEach(async (_, i) => {
            try {
                const st = await api.getServerStatus(i);
                if (st.state === 'running') await api.stopServer(i);
            } catch { /* siguiente poll corrige */ }
        });
    });

    // Run all tests: dispara Playwright en cada proyecto con servidor running
    // (el backend cancela si el server no responde; aquí se evita lanzarlo).
    $('dash-tests-all').addEventListener('click', async () => {
        const projects = await api.getProjects();
        let launched = 0;
        for (let i = 0; i < projects.length; i++) {
            try {
                const st = await api.getServerStatus(i);
                if (st.state === 'running') { api.runTests(i); launched++; }
            } catch { /* siguiente poll corrige */ }
        }
        showToast(
            'Dashboard',
            launched > 0 ? `Running tests on ${launched} running server(s)` : 'No running servers to test',
            launched > 0 ? 'info' : 'warn',
        );
    });

    return {
        refresh,
        setVisible(v) {
            visible = v;
            updateTimer();
            if (v) refresh();
        },
    };
}
