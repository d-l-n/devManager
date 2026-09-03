import { api, events } from './api.js';
import { mount as mountPlaywright } from './panels/playwright.js';
import { mount as mountScripts } from './panels/scripts.js';
import { mount as mountGit } from './panels/git.js';
import { mount as mountEvidence } from './panels/evidence.js';
import { mount as mountObscura } from './panels/obscura.js';
import { mount as mountBacklog } from './panels/backlog.js';
import { mount as mountMonitor } from './panels/monitor.js';
import { applyTheme, THEME_CYCLE, currentTheme, getOledMode } from './theme.js';
import { showToast } from './widgets/toast.js';
import { mountSettingsView } from './views/settings.js';
import { mountProjectDialog } from './dialogs/project.js';
import { mountAppLogDialog } from './dialogs/applog.js';
import { mountContextMenu } from './widgets/contextmenu.js';
import { mountBacklogItemDialog } from './dialogs/backlog-item.js';
import { mountMessageDialog } from './dialogs/message.js';
import { hydrateIcons, icon, setIcon } from './icons.js';

const $ = (id) => document.getElementById(id);

// Tope de líneas guardadas por proyecto y de nodos del panel de logs (Task 20)
const MAX_LOG_LINES = 2000;

const state = {
    projects: [],
    selected: -1,
    logs: new Map(),      // index -> [{ts,line,isError}]
    errorsOnly: false,
    logWrap: false,
    logAutoScroll: true,
    view: 'project',
    projectFilter: 'all',
    serverStates: new Map(),
    portMismatches: new Map(),
};

function timestamp() {
    const d = new Date();
    const p = (n) => String(n).padStart(2, '0');
    return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

// Update theme button icon based on current theme
function updateThemeButtonIcon() {
    const theme = currentTheme();
    const oledMode = getOledMode();
    
    // Show sun for light, moon for dark or OLED
    const icon = theme === 'light' ? 'sun' : 'moon';
    
    setIcon($('btn-theme'), icon);
    
    // Remove any inline styles to let CSS handle them consistently
    const btn = $('btn-theme');
    btn.style.background = '';
    btn.style.color = '';
}

// Make function globally available for settings module
window.updateThemeButtonIcon = updateThemeButtonIcon;

async function refreshProjects(keepSelection = true) {
    state.projects = await api.getProjects();
    if (!keepSelection || state.selected >= state.projects.length) {
        state.selected = state.projects.length ? 0 : -1;
    }
    await refreshServerStates();
    renderList();
    renderDetail();
}

async function refreshServerStates() {
    const statuses = await Promise.all(state.projects.map(async (_, index) => {
        try { return [index, await api.getServerStatus(index)]; } catch { return [index, null]; }
    }));
    statuses.forEach(([index, status]) => {
        if (status && status.state) state.serverStates.set(index, status.state);
    });
}

function renderList() {
    const ul = $('project-list');
    ul.innerHTML = '';
    const q = $('search').value.toLowerCase();
    let visibleCount = 0;
    // Fijados primero (orden estable: pinned antes que unpinned)
    const order = state.projects
        .map((_, i) => i)
        .sort((a, b) => (state.projects[b].pinned ? 1 : 0) - (state.projects[a].pinned ? 1 : 0));
    order.forEach((i) => {
        const p = state.projects[i];
        if (q && !p.name.toLowerCase().includes(q)) return;
        const serverState = state.serverStates.get(i) || 'stopped';
        if (state.projectFilter !== 'all' && serverState !== state.projectFilter) return;
        
        visibleCount++;
        const li = document.createElement('li');
        const cls = [];
        const isSelected = i === state.selected;
        if (isSelected) cls.push('selected');
        if (p.pinned) cls.push('pinned');
        li.className = cls.join(' ');
        li.setAttribute('role', 'option');
        li.setAttribute('aria-selected', isSelected);
        const dot = document.createElement('span');
        dot.className = 'proj-dot';
        dot.dataset.index = i;
        li.appendChild(dot);
        const name = document.createElement('span');
        name.textContent = p.name;
        name.className = 'proj-name';
        li.appendChild(name);
        const st = document.createElement('span');
        st.className = 'proj-state';
        li.appendChild(st);
        // Botón de pin: alterna fijado sin propagar la selección.
        const pin = document.createElement('button');
        pin.className = 'pin-btn';
        pin.setAttribute('aria-label', p.pinned ? 'Unpin project' : 'Pin project');
        setIcon(pin, p.pinned ? 'pinned' : 'pin');
        pin.title = 'Pin / unpin';
        pin.addEventListener('click', async (e) => {
            e.stopPropagation();
            await api.togglePin(i);
            refreshProjects();
        });
        pin.addEventListener('dblclick', (e) => e.stopPropagation());
        li.appendChild(pin);
        li.addEventListener('click', () => {
            state.selected = i;
            renderList();
            renderDetail();
        });
        li.addEventListener('dblclick', () => editProject(i));
        li.addEventListener('contextmenu', (e) => {
            e.preventDefault();
            state.selected = i;
            showProjectContextMenu(i, p, e.clientX, e.clientY);
        });
        ul.appendChild(li);
    });
    
    // Show/hide empty state message
    const emptyMsg = $('projects-empty');
    if (visibleCount === 0) {
        emptyMsg.hidden = false;
        // Customize message based on filter
        if (state.projectFilter === 'running') {
            emptyMsg.textContent = 'No running projects';
        } else if (state.projectFilter === 'stopped') {
            emptyMsg.textContent = 'No stopped projects';
        } else if (q) {
            emptyMsg.textContent = `No projects match "${q}"`;
        } else if (state.projects.length === 0) {
            emptyMsg.textContent = 'No projects yet. Add one to get started!';
        } else {
            emptyMsg.textContent = 'No projects found';
        }
    } else {
        emptyMsg.hidden = true;
    }
    
    // Contador de proyectos en el sidebar
    const count = $('project-count');
    count.textContent = `${state.projects.length} project(s)`;
    count.title = `${state.projects.length} project(s)`;
    updateDots();
}

// Menu contextual del proyecto (Issue #12): derecho sobre un item de la lista.
function showProjectContextMenu(index, p, x, y) {
    const run = (fn) => { fn().then(() => refreshProjects()); };
    const items = [
        { label: 'Restart Server', icon: 'restart', onClick: () => run(() => api.restartServer(index)) },
        { label: 'Stop Server', icon: 'stop', onClick: () => run(() => api.stopServer(index)) },
        { label: 'Open in Browser', icon: 'browser', onClick: () => api.openURL((p.server && p.server.url) || '') },
        { separator: true },
        { label: 'Open in Explorer', icon: 'folder', onClick: () => api.openInExplorer(index) },
        { label: 'Open Terminal', icon: 'command', onClick: () => api.openTerminal(index) },
        { label: 'Open in VS Code', icon: 'code', onClick: () => api.openVSCode(index) },
        { label: 'Open in OpenCode', icon: 'code', onClick: () => api.openOpenCode(index) },
        { separator: true },
        { label: 'Run Tests', icon: 'tests', onClick: () => run(() => api.runTests(index)) },
        { label: p.pinned ? 'Unpin' : 'Pin', icon: p.pinned ? 'pinned' : 'pin', onClick: () => run(() => api.togglePin(index)) },
        { label: 'Edit Project', icon: 'edit', onClick: () => editProject(index) },
        { separator: true },
        { label: 'Remove Project', icon: 'trash', danger: true, onClick: () => removeProjectFlow(index) },
    ];
    contextMenu.show(items, x, y);
}

async function updateDots() {
    const dots = document.querySelectorAll('.proj-dot');
    if (dots.length === 0) return;

    // Batch all status requests into a single Promise.all
    const indices = Array.from(dots).map((dot) => parseInt(dot.dataset.index, 10));
    const statuses = await Promise.all(
        indices.map((i) => api.getServerStatus(i).catch(() => ({ state: 'stopped' })))
    );

    let listDirty = false;
    dots.forEach((dot, idx) => {
        const i = indices[idx];
        const status = statuses[idx];
        const previous = state.serverStates.get(i);
        state.serverStates.set(i, status.state);
        dot.className = `proj-dot ${status.state}`;
        const label = dot.closest('li') && dot.closest('li').querySelector('.proj-state');
        if (label) {
            const running = status.state === 'running';
            label.textContent = running ? 'running' : '';
            label.classList.toggle('running', running);
        }
        if (previous !== undefined && previous !== status.state && state.projectFilter !== 'all') {
            listDirty = true;
        }
    });
    if (listDirty) renderList();
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
    ctx.panels.obscuraPanel.onProjectChanged(p);
    ctx.panels.backlogPanel.onProjectChanged(p);
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
    $('btn-open-url').disabled = status.state !== 'running';

    const up = $('uptime-label');
    if (status.uptimeSeconds > 0) {
        const s = Math.floor(status.uptimeSeconds);
        up.textContent = `up ${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m ${s % 60}s`;
    } else {
        up.textContent = '';
    }

    // Badge Server: puerto activo/configurado y advertencia de desajuste.
    const bs = $('badge-server');
    if (bs) {
        bs.className = `badge ${status.state}`;
        const port = status.activePort || state.projects[state.selected].server.port;
        const mismatch = state.portMismatches.get(state.selected);
        bs.textContent = `Server: :${port}${mismatch ? ' (port mismatch)' : ''}`;
    }
    const saveDetectedPort = $('btn-save-detected-port');
    if (saveDetectedPort) saveDetectedPort.hidden = !state.portMismatches.has(state.selected);

    // Badge Playwright (Task 15): sin Playwright, estado "off".
    try {
        const ps = await api.getPlaywrightStatus(state.selected);
        const pw = $('badge-pw');
        if (pw) {
            const st = ps && ps.state ? ps.state : 'off';
            pw.className = `badge ${st}`;
            pw.textContent = `Playwright: ${st}`;
        }
    } catch (e) { console.warn('[playwright status]', e?.message || e); }

    // Badge Git: rama actual y estado del árbol; '—' cuando no es repo.
    try {
        const gs = await api.getGitStatus(state.selected);
        const bg = $('badge-git');
        if (bg && gs) {
            if (gs.isRepo) {
                bg.className = `badge ${gs.isDirty ? 'dirty' : 'clean'}`;
                bg.textContent = `Git: ${gs.branch || 'unknown'}${gs.isDirty ? ' • dirty' : ''}`;
            } else {
                bg.className = 'badge';
                bg.textContent = 'Git: —';
            }
        }
    } catch (e) { console.warn('[git status]', e?.message || e); }

    updateDots();
}

// Texto plano de los logs del proyecto seleccionado (Copy/Save, Task 20)
function logText() {
    const lines = state.logs.get(state.selected) || [];
    return lines.map((l) => `[${l.ts}] ${l.line}`).join('\n');
}

function appendLog(index, line, isError) {
    if (!state.logs.has(index)) state.logs.set(index, []);
    const arr = state.logs.get(index);
    arr.push({ ts: timestamp(), line, isError });
    // Cap de líneas en memoria por proyecto (Task 20).
    if (arr.length > MAX_LOG_LINES) state.logs.set(index, arr.slice(-MAX_LOG_LINES));

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
    // Cap del DOM (~2000 nodos; descarta los más viejos).
    while (out.childElementCount > MAX_LOG_LINES) out.removeChild(out.firstChild);
    // Sin follow: no forzar scroll al fondo
    if (state.logAutoScroll) out.scrollTop = out.scrollHeight;
}

function reloadLogs() {
    const out = $('log-output');
    out.innerHTML = '';
    (state.logs.get(state.selected) || [])
        .filter((e) => !state.errorsOnly || e.isError)
        .forEach(appendLogEntry);
    if (state.logAutoScroll) out.scrollTop = out.scrollHeight;
}

function switchView(view) {
    if (state.view === view) return;
    state.view = view;
    
    // Handle tab states - only show active for project/monitor, none for settings
    $('view-project').classList.toggle('active', view === 'project');
    $('view-monitor').classList.toggle('active', view === 'monitor');
    
    // Handle view visibility
    $('monitor-view').hidden = view !== 'monitor';
    $('settings-view').hidden = view !== 'settings';
    ctx.panels.monitorPanel.setVisible(view === 'monitor');
    
    // Render settings view when switching to it
    if (view === 'settings' && window.settingsView && window.settingsView.render) {
        // Ensure settings view is properly initialized and rendered
        try {
            window.settingsView.render();
        } catch (error) {
            console.error('Error rendering settings view:', error);
        }
    }
    
    // Only render project detail for project view
    if (view === 'project') {
        renderDetail();
    } else {
        // Hide project detail when not in project view
        $('project-detail').hidden = true;
        $('empty-state').hidden = view !== 'project';
    }
}

async function addProjectFlow() {
    projectDialog.openNew();
}

async function editProject(index) {
    if (index < 0 || index >= state.projects.length) return;
    projectDialog.openEdit(index, state.projects[index]);
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
    if (await messageDialog.confirm({ title: 'Quit app', message: 'All servers will be stopped.', confirmLabel: 'Quit', destructive: true })) await api.quit();
}

async function restartAppFlow() {
    if (await messageDialog.confirm({ title: 'Restart app', message: 'All running servers and scripts will be stopped.', confirmLabel: 'Restart', destructive: true })) {
        await api.restartApp();
    }
}

async function removeProjectFlow(index, trigger) {
    const p = state.projects[index];
    if (!p) return;
    const ok = await messageDialog.confirm({ title: 'Remove project', message: `Remove “${p.name}” from the manager? Local files will not be deleted.`, confirmLabel: 'Remove', destructive: true, trigger });
    if (ok) { await api.removeProject(index); await refreshProjects(); }
}

function wireEvents() {
    $('btn-add').addEventListener('click', addProjectFlow);
    $('btn-reload').addEventListener('click', () => api.reloadProjects());

    $('search').addEventListener('input', renderList);
    document.querySelectorAll('.filter-chip').forEach((button) => {
        button.addEventListener('click', () => {
            state.projectFilter = button.dataset.filter;
            document.querySelectorAll('.filter-chip').forEach((chip) => {
                const isActive = chip === button;
                chip.classList.toggle('active', isActive);
                chip.setAttribute('aria-pressed', isActive);
            });
            renderList();
        });
    });
    $('errors-only').addEventListener('change', (e) => {
        state.errorsOnly = e.target.checked;
        reloadLogs();
    });
    $('btn-clear-log').addEventListener('click', () => {
        state.logs.set(state.selected, []);
        $('log-output').innerHTML = '';
    });

    // Herramientas del panel de logs (Task 20)
    $('log-wrap').addEventListener('click', () => {
        state.logWrap = !state.logWrap;
        $('log-output').classList.toggle('log-wrap', state.logWrap);
        $('log-wrap').classList.toggle('active', state.logWrap);
    });
    $('log-follow').addEventListener('click', () => {
        state.logAutoScroll = !state.logAutoScroll;
        $('log-follow').classList.toggle('active', state.logAutoScroll);
        if (state.logAutoScroll) {
            const out = $('log-output');
            out.scrollTop = out.scrollHeight;
        }
    });
    $('log-copy').addEventListener('click', async () => {
        const text = logText();
        if (!text) return;
        try {
            await navigator.clipboard.writeText(text);
        } catch {
            // Fallback: textarea oculto + execCommand si Clipboard API no está
            const ta = document.createElement('textarea');
            ta.value = text;
            ta.style.position = 'fixed';
            ta.style.opacity = '0';
            document.body.appendChild(ta);
            ta.select();
            document.execCommand('copy');
            ta.remove();
        }
    });
    $('log-save').addEventListener('click', () => {
        const text = logText();
        if (!text) return;
        const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = 'project-log.txt';
        a.click();
        URL.revokeObjectURL(a.href);
    });

    $('btn-start').addEventListener('click', () => api.startServer(state.selected));
    $('btn-stop').addEventListener('click', () => api.stopServer(state.selected));
    $('btn-restart').addEventListener('click', () => api.restartServer(state.selected));

    $('view-project').addEventListener('click', () => switchView('project'));
    $('view-monitor').addEventListener('click', () => switchView('monitor'));

    // Auto-asignar puertos únicos (Task 16)
    $('btn-auto-ports').addEventListener('click', async () => {
        const n = await api.autoAssignPorts();
        showToast('Auto-Assign Ports',
            n > 0 ? `Assigned unique ports to ${n} project(s)` : 'All projects already have unique ports',
            n > 0 ? 'success' : 'info');
        if (n > 0) refreshProjects();
    });

$('btn-theme').addEventListener('click', () => {
        // Simple toggle between light and dark (OLED mode is handled separately)
        const cur = currentTheme();
        // If current theme is OLED, treat it as dark for toggling purposes
        const effectiveTheme = cur === 'oled' ? 'dark' : cur;
        const newTheme = effectiveTheme === 'light' ? 'dark' : 'light';
        applyTheme(newTheme);
        updateThemeButtonIcon();
    });
    $('btn-settings').addEventListener('click', () => switchView('settings'));
    $('btn-applog').addEventListener('click', () => appLogDialog.open());
    $('btn-quit').addEventListener('click', quitApp);

    // Abre en el navegador la URL del proyecto seleccionado (Task 18)
    $('btn-open-url').addEventListener('click', () => {
        const p = state.projects[state.selected];
        if (p && p.server && p.server.url) api.openURL(p.server.url);
    });
    $('btn-save-detected-port').addEventListener('click', async () => {
        const mismatch = state.portMismatches.get(state.selected);
        if (!mismatch) return;
        const errors = await api.saveDetectedPort(state.selected, mismatch.detected);
        if (errors && errors.length) {
            showToast('Port Mismatch', errors.join('\n'), 'error');
            return;
        }
        state.portMismatches.delete(state.selected);
        await refreshProjects();
        showToast('Server Port', `Saved detected port ${mismatch.detected}`, 'success');
    });

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

    // Puerto detectado distinto al configurado: actualizar la URL en estado y panel (Task 19).
    events().EventsOn('server:port_detected', ({ index, port, url }) => {
        const p = state.projects[index];
        if (p && p.server) {
            if (url) p.server.url = url;
            if (port) p.server.activePort = port;
        }
        if (index === state.selected && $('url-label')) {
            if (url) $('url-label').textContent = url;
            refreshStatus();
        }
    });
    events().EventsOn('server:port_mismatch', ({ index, configured, detected, url }) => {
        state.portMismatches.set(index, { configured, detected, url });
        const parts = [];
        if (configured != null) parts.push(`Configured ${configured}`);
        if (detected != null) parts.push(`detected ${detected}`);
        if (url) parts.push(`redirected to ${url}`);
        showToast('Port Mismatch', parts.join(' '), 'warning');
        if (index === state.selected) refreshStatus();
    });

    setInterval(refreshStatus, 2000); // uptime ticker (2s interval)
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

        // Settings shortcut - Ctrl+,
        if (mod && key === ',') {
            switchView('settings');
            return;
        }

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
            removeProjectFlow(sel);
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
                case 't':
                    e.preventDefault();
                    if (hasSelection()) api.openTerminal(sel);
                    break;
            }
            return;
        }

        if (e.altKey) {
            if (key === 'm') {
                e.preventDefault();
                switchView('monitor');
                return;
            }
            if (key === 'l') {
                e.preventDefault();
                appLogDialog.open();
                return;
            }
            if (key === 't' && hasSelection()) {
                e.preventDefault();
                api.openTerminal(sel);
            }
            return;
        }

        switch (key) {
            case 'r':
                e.preventDefault();
                api.reloadProjects();
                break;
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
            case 'k':
                e.preventDefault();
                appLogDialog.open();
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

const messageDialog = mountMessageDialog();
document.body.appendChild(messageDialog.getElement());
ctx.messageDialog = messageDialog;
window.messageDialog = messageDialog;

const playwrightPanel = mountPlaywright(ctx);
const scriptsPanel = mountScripts(ctx);
const gitPanel = mountGit(ctx);
const evidencePanel = mountEvidence(ctx);
const obscuraPanel = mountObscura(ctx);
const monitorPanel = mountMonitor(ctx);
const backlogPanel = mountBacklog(ctx);
ctx.panels = { playwrightPanel, scriptsPanel, gitPanel, evidencePanel, obscuraPanel, monitorPanel, backlogPanel };

const settingsView = mountSettingsView();
window.settingsView = settingsView;
const projectDialog = mountProjectDialog(async (savedIndex) => {
    await refreshProjects(false);
    // Auto-select the newly added/edited project
    if (typeof savedIndex === 'number' && savedIndex >= 0) {
        state.selected = savedIndex;
        renderList();
        renderDetail();
    } else {
        // For adds without known index, select the last project
        if (state.projects.length > 0) {
            state.selected = state.projects.length - 1;
            renderList();
            renderDetail();
        }
    }
});
const appLogDialog = mountAppLogDialog();
const contextMenu = mountContextMenu();
const backlogItemDialog = mountBacklogItemDialog();
document.body.appendChild(backlogItemDialog.getElement());
window.backlogItemDialog = backlogItemDialog;
hydrateIcons();
updateThemeButtonIcon();

// Make updateThemeButtonIcon globally accessible for settings dialog
window.updateThemeButtonIcon = updateThemeButtonIcon;

function switchTab(name) {
    document.querySelectorAll('.tab').forEach((b) => {
        const isActive = b.dataset.tab === name;
        b.classList.toggle('active', isActive);
        b.setAttribute('aria-selected', isActive);
    });
    document.querySelectorAll('.panel').forEach((p) =>
        p.classList.toggle('active', p.id === `panel-${name}`));
}
document.querySelectorAll('.tab').forEach((btn) =>
    btn.addEventListener('click', () => switchTab(btn.dataset.tab)));

async function boot() {
    wireEvents();
    wireKeyboardShortcuts();
    await refreshProjects(false);
    await settingsView.init(); // carga settings: tema + gate de toasts
    // Background update check (Issue #58)
    checkForUpdateOnBoot();
}

async function checkForUpdateOnBoot() {
    try {
        const info = await api.checkForUpdate();
        if (info && !info.isUpToDate && !info.error) {
            showToast('Update Available', `v${info.latestVersion} is available (current: ${info.currentVersion})`, 'info');
        }
    } catch { /* silent — don't bother user on network errors */ }
}

boot();
