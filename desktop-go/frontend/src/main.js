import { api, events } from './api.js';
import { mount as mountPlaywright } from './panels/playwright.js';
import { mount as mountScripts } from './panels/scripts.js';
import { mount as mountGit } from './panels/git.js';
import { mount as mountEvidence } from './panels/evidence.js';
import { mount as mountMonitor } from './panels/monitor.js';
import { applyTheme, THEME_CYCLE } from './theme.js';
import { showToast } from './widgets/toast.js';
import { mountSettings } from './dialogs/settings.js';

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
        li.addEventListener('dblclick', () => editProject(i));
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
    ctx.panels.evidencePanel.onProjectChanged(p);
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

async function addProjectFlow() {
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
}

async function editProject(index) {
    if (index < 0 || index >= state.projects.length) return;
    const p = state.projects[index];
    const name = prompt('Project name:', p.name);
    if (name === null) return;
    const path = prompt('Project path:', p.path);
    if (path === null) return;
    if (name === p.name && path === p.path) return;
    const errs = await api.updateProject(index, {
        ...p,
        name,
        path,
        server: p.server,
        playwright: p.playwright,
        pinned: p.pinned,
    });
    if (errs && errs.length) alert(errs.join('\n'));
    await refreshProjects();
}

function hasSelection() {
    return state.selected >= 0 && state.selected < state.projects.length;
}

async function startOrRestartSelected() {
    if (!hasSelection()) return;
    const st = await api.getServerStatus(state.selected);
    if (st.state === 'running') await api.restartServer(state.selected);
    else await api.startServer(state.selected);
}

async function quitApp() {
    if (confirm('Quit Local Dev Manager? All servers will be stopped.')) await api.quit();
}

async function restartAppFlow() {
    if (confirm('Restart Local Dev Manager?\n\nAll running servers and scripts will be stopped.')) {
        await api.restartApp();
    }
}

function wireEvents() {
    $('btn-add').addEventListener('click', addProjectFlow);

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
    $('btn-settings').addEventListener('click', () => settingsDialog.open());
    $('btn-quit').addEventListener('click', quitApp);

    // Eventos push desde Go
    events().EventsOn('projects:changed', async () => refreshProjects());
    events().EventsOn('config:error', (payload) =>
        showToast('Configuration', payload.message, 'warning'));
    events().EventsOn('notify', ({ title, message, level }) =>
        showToast(title, message, level));
    events().EventsOn('server:log', (payload) =>
        appendLog(payload.index, payload.line, payload.isError));
    events().EventsOn('server:state', () => refreshStatus());
    events().EventsOn('server:ready', () => refreshStatus());

    setInterval(refreshStatus, 1000); // uptime ticker
}

function isEditableTarget(e) {
    const t = e.target;
    if (!(t instanceof HTMLElement)) return false;
    if (t.isContentEditable) return true;
    return ['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName);
}

function wireKeyboardShortcuts() {
    window.addEventListener('keydown', (e) => {
        const mod = e.ctrlKey || e.metaKey;
        const key = typeof e.key === 'string' ? e.key.toLowerCase() : '';

        // Settings modal ya registra Ctrl+, en settings.js: no duplicar.
        if (mod && key === ',') return;

        // Único atajo que funciona dentro de inputs: robar el foco para buscar.
        if (mod && !e.shiftKey && !e.altKey && key === 'f') {
            e.preventDefault();
            $('search').focus();
            return;
        }
        if (isEditableTarget(e)) return;

        const sel = state.selected;
        if (!mod && !e.altKey && key === 'f5') {
            e.preventDefault();
            if (e.shiftKey) {
                if (hasSelection()) api.stopServer(sel);
            } else {
                startOrRestartSelected();
            }
            return;
        }
        if (!mod && key === 'delete') {
            if (!hasSelection()) return;
            const p = state.projects[sel];
            if (confirm(`Remove project '${p.name}' from the manager?\n\nLocal files will not be deleted.`)) {
                api.removeProject(sel).then(() => refreshProjects());
            }
            return;
        }
        if (!mod) return; // resto requiere modificador

        if (e.shiftKey) {
            switch (key) {
                case 'c':
                    e.preventDefault();
                    if (hasSelection()) api.openVSCode(sel);
                    break;
                case 'o':
                    e.preventDefault();
                    if (hasSelection()) api.openOpenCode(sel);
                    break;
                case 'r':
                    e.preventDefault();
                    restartAppFlow();
                    break;
            }
            return;
        }

        if (e.altKey) {
            if (key === 't' && hasSelection()) {
                e.preventDefault();
                api.openTerminal(sel);
            }
            return;
        }

        switch (key) {
            case 't':
                e.preventDefault();
                if (hasSelection()) api.runTests(sel);
                break;
            case 'l':
                e.preventDefault();
                if (hasSelection()) {
                    state.logs.set(sel, []);
                    $('log-output').innerHTML = '';
                }
                break;
            case 'n':
                e.preventDefault();
                addProjectFlow();
                break;
            case 'e':
                e.preventDefault();
                editProject(sel);
                break;
            case 'o':
                e.preventDefault();
                if (hasSelection()) api.openInExplorer(sel);
                break;
            case '`':
                e.preventDefault();
                if (hasSelection()) api.openTerminal(sel);
                break;
            case 'q':
                e.preventDefault();
                quitApp();
                break;
        }
    });
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
const evidencePanel = mountEvidence(ctx);
const monitorPanel = mountMonitor(ctx);
ctx.panels = { playwrightPanel, scriptsPanel, gitPanel, evidencePanel, monitorPanel };

const settingsDialog = mountSettings();

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
    wireKeyboardShortcuts();
    await refreshProjects(false);
    await settingsDialog.init(); // carga settings: tema + gate de toasts
}

boot();
