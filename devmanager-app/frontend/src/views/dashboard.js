// Vista Dashboard global (Issue #64): una card por proyecto con estado,
// puerto, uptime actual y CPU/RAM en vivo + resumen consolidado (running/
// stopped/puertos) + timelines de uptime 24h por proyecto (SVG sin deps).
// Frontend-only: reúne GetProjects + GetServerStatus + GetMonitorData +
// GetDashboardHistory. Patrón monitor.js: poll solo mientras es visible.
import { showToast } from '../widgets/toast.js';

const POLL_MS = 5000;
const HISTORY_WINDOW_MS = 24 * 60 * 60 * 1000;
const SPARK_W = 240;
const SPARK_H = 10;
const SPARK_BUCKETS = 120; // 12 min por bucket

const SVG_NS = 'http://www.w3.org/2000/svg';

export function mount(ctx) {
    const { $, api, events } = ctx;

    let visible = false;
    let timer = null;

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

        const port = document.createElement('span');
        port.className = 'dash-card-port mono';
        port.textContent = project.server && project.server.port ? `:${project.server.port}` : '';
        info.appendChild(port);

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

    async function doRefresh() {
        const projects = await api.getProjects();
        const cards = $('dash-cards');
        const empty = $('dash-empty');
        cards.innerHTML = '';

        if (!projects || projects.length === 0) {
            empty.hidden = false;
            $('dash-summary').textContent = '';
            $('dash-start-all').disabled = true;
            $('dash-stop-all').disabled = true;
            $('dash-uptimes').innerHTML = '';
            $('dash-uptimes-empty').hidden = true;
            return;
        }

        empty.hidden = true;
        $('dash-start-all').disabled = false;
        $('dash-stop-all').disabled = false;

        const statuses = await Promise.all(projects.map(async (_, i) => {
            try { return await api.getServerStatus(i); } catch { return { state: 'stopped' }; }
        }));

        // CPU/RAM en vivo + resumen de puertos (una sola llamada al monitor).
        const usageByName = {};
        let portsInUse = 0;
        let conflicts = 0;
        try {
            const mon = await api.getMonitorData();
            ((mon && mon.resRows) || []).forEach((r) => { usageByName[r.name] = r; });
            const rows = (mon && mon.portRows) || [];
            portsInUse = rows.filter((r) => r.state === 'ours' || r.state === 'foreign').length;
            conflicts = rows.filter((r) => r.state === 'foreign').length;
        } catch { /* resumen sin puertos si el monitor falla */ }

        statuses.forEach((status, i) => cards.appendChild(cardEl(projects[i], i, status, usageByName[projects[i].name])));

        const running = statuses.filter((s) => s.state === 'running').length;
        const stopped = statuses.length - running;
        const parts = [`${running} running`, `${stopped} stopped`, `${portsInUse} ports in use`];
        if (conflicts > 0) parts.push(`${conflicts} conflict(s)`);
        $('dash-summary').textContent = parts.join(' · ');

        // Uptime (24h): historial por proyecto → timelines.
        const uptimes = $('dash-uptimes');
        uptimes.innerHTML = '';
        let history = [];
        try { history = (await api.getDashboardHistory()) || []; } catch { /* sin historial */ }
        let rendered = 0;
        history.forEach((h) => {
            const row = uptimeRowEl(h);
            if (row) { uptimes.appendChild(row); rendered++; }
        });
        $('dash-uptimes-empty').hidden = rendered > 0;
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

    return {
        refresh,
        setVisible(v) {
            visible = v;
            updateTimer();
            if (v) refresh();
        },
    };
}
