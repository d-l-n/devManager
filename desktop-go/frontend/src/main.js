import { api, events } from './api.js';
import { mount as mountPlaywright } from './panels/playwright.js';
import { mount as mountScripts } from './panels/scripts.js';
import { mount as mountGit } from './panels/git.js';
import { mount as mountMonitor, THEME_CYCLE } from './panels/monitor.js';

const $ = (id) => document.getElementById(id);

const state = {
    projects: [],
    selected: -1,
    logs: new Map(),      // index -> [{ts,line,isError}]
    errorsOnly: false,
    view: 'project',
};

function timestamp() {
    const d = new Date();
    const p = (n) => String(n).padStart(2, '0');
    return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

async function refreshProjects(keepSelection = true) {
    state.projects = await api.getProjects();
    if (!keepSelection || state.selected >= state.projects.length) {
        state.selected = state.projects.length ? 0 : -1;
    }
    renderList();
    renderDetail();
}

function renderList() {
    const ul = $('project-list');
    ul.innerHTML = '';
    const q = $('search').value.toLowerCase();
    state.projects.forEach((p, i) => {
        if (q && !p.name.toLowerCase().includes(q)) return;
        const li = document.createElement('li');
        li.className = i === state.selected ? 'selected' : '';
        const dot = document.createElement('span');
        dot.className = 'proj-dot';
        dot.dataset.index = i;
        li.appendChild(dot);
        const name = document.createElement('span');
        name.textContent = p.name;
        li.appendChild(name);
        li.addEventListener('click', () => {
            state.selected = i;
            renderList();
            renderDetail();
        });
        ul.appendChild(li);
    });
    updateDots();
}

function updateDots() {
    document.querySelectorAll('.proj-dot').forEach(async (dot) => {
        const i = parseInt(dot.dataset.index, 10);
        const status = await api.getServerStatus(i);
        dot.className = `proj-dot ${status.state}`;
    });
}

function renderDetail() {
    const has = state.selected >= 0;
    const monitorMode = state.view === 'monitor';
    $('empty-state').hidden = has || monitorMode;
    $('project-detail').hidden = !has || monitorMode;
    if (!has || monitorMode) return;
    const p = state.projects[state.selected];
    $('project-name').textContent = p.name;
    $('url-label').textContent = p.server.url;
    reloadLogs();
    refreshStatus();
    ctx.panels.playwrightPanel.onProjectChanged(p);
    ctx.panels.scriptsPanel.onProjectChanged(p);
    ctx.panels.gitPanel.onProjectChanged(p);
}

let uptimeTimer = null;
async function refreshStatus() {
    if (state.selected < 0) return;
    const status = await api.getServerStatus(state.selected);
    const badge = $('state-badge');
    badge.className = `badge ${status.state}`;
    badge.textContent = status.state;
    ['btn-start', 'btn-restart'].forEach((id) => {
        $(id).disabled = status.state === 'running' || status.state === 'starting';
    });
    $('btn-stop').disabled = status.state === 'stopped';

    const up = $('uptime-label');
    if (status.uptimeSeconds > 0) {
        const s = Math.floor(status.uptimeSeconds);
        up.textContent = `up ${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m ${s % 60}s`;
    } else {
        up.textContent = '';
    }
    updateDots();
}

function appendLog(index, line, isError) {
    if (!state.logs.has(index)) state.logs.set(index, []);
    state.logs.get(index).push({ ts: timestamp(), line, isError });

    if (index !== state.selected) return;
    if (state.errorsOnly && !isError) return;
    appendLogEntry({ ts: timestamp(), line, isError });
}

function appendLogEntry(entry) {
    const out = $('log-output');
    const div = document.createElement('div');
    div.className = `log-line${entry.isError ? ' err' : ''}`;
    const ts = document.createElement('span');
    ts.className = 'ts';
    ts.textContent = `[${entry.ts}]`;
    div.appendChild(ts);
    div.appendChild(document.createTextNode(entry.line));
    out.appendChild(div);
    out.scrollTop = out.scrollHeight;
}

function reloadLogs() {
    const out = $('log-output');
    out.innerHTML = '';
    (state.logs.get(state.selected) || [])
        .filter((e) => !state.errorsOnly || e.isError)
        .forEach(appendLogEntry);
}

function switchView(view) {
    if (state.view === view) return;
    state.view = view;
    $('view-project').classList.toggle('active', view === 'project');
    $('view-monitor').classList.toggle('active', view === 'monitor');
    $('monitor-view').hidden = view !== 'monitor';
    ctx.panels.monitorPanel.setVisible(view === 'monitor');
    renderDetail();
}

function applyTheme(theme) {
    document.documentElement.dataset.theme = theme;
    api.setSetting('theme', theme);
}

function wireEvents() {
    $('btn-add').addEventListener('click', async () => {
        const name = prompt('Project name:');
        if (!name) return;
        const path = prompt('Project path:');
        if (!path) return;
        const errs = await api.addProject({
            name, path,
            server: { enabled: true, command: 'npm run dev', port: 5173, url: 'http://localhost:5173', startup_timeout: 15000 },
            playwright: { enabled: false, command: '', ui_command: '', debug_command: '', report_command: '' },
            pinned: false,
        });
        if (errs && errs.length) alert(errs.join('\n'));
        await refreshProjects();
    });

    $('search').addEventListener('input', renderList);
    $('errors-only').addEventListener('change', (e) => {
        state.errorsOnly = e.target.checked;
        reloadLogs();
    });
    $('btn-clear-log').addEventListener('click', () => {
        state.logs.set(state.selected, []);
        $('log-output').innerHTML = '';
    });

    $('btn-start').addEventListener('click', () => api.startServer(state.selected));
    $('btn-stop').addEventListener('click', () => api.stopServer(state.selected));
    $('btn-restart').addEventListener('click', () => api.restartServer(state.selected));

    $('view-project').addEventListener('click', () => switchView('project'));
    $('view-monitor').addEventListener('click', () => switchView('monitor'));

    $('btn-theme').addEventListener('click', () => {
        const cur = document.documentElement.dataset.theme || 'dark';
        applyTheme(THEME_CYCLE[(THEME_CYCLE.indexOf(cur) + 1) % THEME_CYCLE.length]);
    });
    $('btn-settings').addEventListener('click', () => console.log('settings task 7'));
    $('btn-quit').addEventListener('click', async () => {
        if (confirm('Quit Local Dev Manager? All servers will be stopped.')) await api.quit();
    });

    // Eventos push desde Go
    events().EventsOn('projects:changed', async () => refreshProjects());
    events().EventsOn('config:error', (payload) => alert(payload.message));
    events().EventsOn('notify', ({ level, title, message }) =>
        console.log('[notify]', level, title, message));
    events().EventsOn('server:log', (payload) =>
        appendLog(payload.index, payload.line, payload.isError));
    events().EventsOn('server:state', () => refreshStatus());
    events().EventsOn('server:ready', () => refreshStatus());

    setInterval(refreshStatus, 1000); // uptime ticker
}

const ctx = {
    $,
    api,
    events,
    selectedIndex: () => state.selected,
    appendLog,
};

const playwrightPanel = mountPlaywright(ctx);
const scriptsPanel = mountScripts(ctx);
const gitPanel = mountGit(ctx);
const monitorPanel = mountMonitor(ctx);
ctx.panels = { playwrightPanel, scriptsPanel, gitPanel, monitorPanel };

function switchTab(name) {
    document.querySelectorAll('.tab').forEach((b) =>
        b.classList.toggle('active', b.dataset.tab === name));
    document.querySelectorAll('.panel').forEach((p) =>
        p.classList.toggle('active', p.id === `panel-${name}`));
}
document.querySelectorAll('.tab').forEach((btn) =>
    btn.addEventListener('click', () => switchTab(btn.dataset.tab)));

async function boot() {
    wireEvents();
    await refreshProjects(false);
    try {
        const s = await api.getSettings();
        if (s && s.theme) document.documentElement.dataset.theme = s.theme;
    } catch { /* defaults */ }
}

boot();
