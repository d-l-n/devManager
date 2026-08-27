import { api, events } from './api.js';
import { mount as mountPlaywright } from './panels/playwright.js';
import { mount as mountScripts } from './panels/scripts.js';
import { mount as mountGit } from './panels/git.js';
import { mount as mountEvidence } from './panels/evidence.js';
import { mount as mountMonitor } from './panels/monitor.js';
import { mount as mountBacklog } from './panels/backlog.js';
import { applyTheme, THEME_CYCLE } from './theme.js';
import { showToast } from './widgets/toast.js';
import { mountSettings } from './dialogs/settings.js';
import { mountBacklogItemDialog } from './dialogs/backlog-item.js';

const $ = (id) => document.getElementById(id);

const state = {
    projects: [],
    selected: -1,
    logs: new Map(),      // index -> [{ts,line,isError}]
    errorsOnly: false,
    view: 'project',
    // Filtros de búsqueda
    searchQuery: '',
    statusFilter: 'all', // all, running, stopped, error
    pinnedFilter: 'all', // all, pinned, unpinned
    sortBy: 'name',      // name, path, status, pinned
    sortOrder: 'asc',     // asc, desc
};

function timestamp() {
    const d = new Date();
    const p = (n) => String(n).padStart(2, '0');
    return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

// Debounce function para búsqueda instantánea
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Búsqueda con debounce
const debouncedSearch = debounce(() => {
    renderList();
}, 300);

// Crear controles de filtro
function createFilterControls() {
    // Buscar el sidebar o contenedor de búsqueda
    const searchContainer = document.querySelector('.search-container') || 
                           document.querySelector('input[type="search"]').parentElement;
    
    if (!searchContainer) return;
    
    // Crear contenedor de filtros
    const filterContainer = document.createElement('div');
    filterContainer.className = 'filter-controls';
    
    // Filtro de estado
    const statusFilter = document.createElement('select');
    statusFilter.className = 'status-filter';
    statusFilter.innerHTML = `
        <option value="all">All Status</option>
        <option value="running">Running</option>
        <option value="stopped">Stopped</option>
        <option value="error">Error</option>
    `;
    statusFilter.value = state.statusFilter;
    statusFilter.addEventListener('change', (e) => {
        state.statusFilter = e.target.value;
        renderList();
    });
    
    // Filtro de pineados
    const pinnedFilter = document.createElement('select');
    pinnedFilter.className = 'pinned-filter';
    pinnedFilter.innerHTML = `
        <option value="all">All Projects</option>
        <option value="pinned">Pinned</option>
        <option value="unpinned">Unpinned</option>
    `;
    pinnedFilter.value = state.pinnedFilter;
    pinnedFilter.addEventListener('change', (e) => {
        state.pinnedFilter = e.target.value;
        renderList();
    });
    
    // Ordenamiento
    const sortContainer = document.createElement('div');
    sortContainer.className = 'sort-container';
    
    const sortBy = document.createElement('select');
    sortBy.className = 'sort-by';
    sortBy.innerHTML = `
        <option value="name">Sort by Name</option>
        <option value="path">Sort by Path</option>
        <option value="pinned">Sort by Pinned</option>
    `;
    sortBy.value = state.sortBy;
    sortBy.addEventListener('change', (e) => {
        state.sortBy = e.target.value;
        renderList();
    });
    
    const sortOrder = document.createElement('button');
    sortOrder.className = 'sort-order';
    sortOrder.innerHTML = state.sortOrder === 'asc' ? '↑' : '↓';
    sortOrder.addEventListener('click', () => {
        state.sortOrder = state.sortOrder === 'asc' ? 'desc' : 'asc';
        sortOrder.innerHTML = state.sortOrder === 'asc' ? '↑' : '↓';
        renderList();
    });
    
    sortContainer.appendChild(sortBy);
    sortContainer.appendChild(sortOrder);
    
    // Ensamblar controles
    filterContainer.appendChild(statusFilter);
    filterContainer.appendChild(pinnedFilter);
    filterContainer.appendChild(sortContainer);
    
    // Insertar después del contenedor de búsqueda
    searchContainer.parentNode.insertBefore(filterContainer, searchContainer.nextSibling);
}

// Atajos de teclado mejorados
function setupKeyboardShortcuts() {
    document.addEventListener('keydown', (e) => {
        // Ctrl+K o Cmd+K para focus en búsqueda
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            const searchInput = document.querySelector('input[type="search"], #search');
            if (searchInput) {
                searchInput.focus();
                searchInput.select();
            }
        }
        
        // Ctrl+F para focus en búsqueda (alternativa)
        if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
            e.preventDefault();
            const searchInput = document.querySelector('input[type="search"], #search');
            if (searchInput) {
                searchInput.focus();
                searchInput.select();
            }
        }
        
        // Escape para limpiar búsqueda
        if (e.key === 'Escape') {
            const searchInput = document.querySelector('input[type="search"], #search');
            if (searchInput && document.activeElement === searchInput) {
                searchInput.value = '';
                state.searchQuery = '';
                renderList();
                searchInput.blur();
            }
        }
        
        // Flechas para navegar la lista
        if (e.key === 'ArrowDown' && !e.target.matches('input, textarea, select')) {
            e.preventDefault();
            navigateProjectList(1);
        } else if (e.key === 'ArrowUp' && !e.target.matches('input, textarea, select')) {
            e.preventDefault();
            navigateProjectList(-1);
        }
        
        // Enter para seleccionar proyecto
        if (e.key === 'Enter' && !e.target.matches('input, textarea, select')) {
            e.preventDefault();
            if (state.selected >= 0) {
                renderDetail();
            }
        }
    });
}

// Navegación con flechas
function navigateProjectList(direction) {
    const filteredProjects = filterProjects();
    if (filteredProjects.length === 0) return;
    
    const currentIndex = filteredProjects.findIndex(p => 
        state.projects.indexOf(p) === state.selected
    );
    
    let newIndex = currentIndex + direction;
    if (newIndex < 0) newIndex = filteredProjects.length - 1;
    if (newIndex >= filteredProjects.length) newIndex = 0;
    
    const newProject = filteredProjects[newIndex];
    state.selected = state.projects.indexOf(newProject);
    renderList();
}

async function refreshProjects(keepSelection = true) {
    state.projects = await api.getProjects();
    if (!keepSelection || state.selected >= state.projects.length) {
        state.selected = state.projects.length ? 0 : -1;
    }
    renderList();
    renderDetail();
    
    // Hide welcome screen if projects exist
    if (state.projects.length > 0) {
        hideWelcomeScreen();
    }
}

// Función de filtrado avanzado de proyectos
async function filterProjects() {
    let filtered = [...state.projects];
    
    // Filtro por búsqueda (nombre y path)
    if (state.searchQuery) {
        const query = state.searchQuery.toLowerCase();
        filtered = filtered.filter(p => 
            p.name.toLowerCase().includes(query) || 
            p.path.toLowerCase().includes(query)
        );
    }
    
    // Filtro por estado (necesitamos obtener los estados asíncronamente)
    if (state.statusFilter !== 'all') {
        const projectsWithStatus = await Promise.all(
            filtered.map(async (p) => {
                const index = state.projects.indexOf(p);
                const status = await api.getServerStatus(index);
                return { ...p, status: status.state };
            })
        );
        
        filtered = projectsWithStatus.filter(p => p.status === state.statusFilter);
    }
    
    // Filtro por pineados
    if (state.pinnedFilter === 'pinned') {
        filtered = filtered.filter(p => p.pinned);
    } else if (state.pinnedFilter === 'unpinned') {
        filtered = filtered.filter(p => !p.pinned);
    }
    
    // Ordenamiento
    filtered.sort((a, b) => {
        let comparison = 0;
        
        switch (state.sortBy) {
            case 'name':
                comparison = a.name.localeCompare(b.name);
                break;
            case 'path':
                comparison = a.path.localeCompare(b.path);
                break;
            case 'pinned':
                comparison = (b.pinned ? 1 : 0) - (a.pinned ? 1 : 0);
                break;
            default:
                comparison = a.name.localeCompare(b.name);
        }
        
        return state.sortOrder === 'desc' ? -comparison : comparison;
    });
    
    return filtered;
}

// Función para highlight de texto
function highlightText(text, query) {
    if (!query) return text;
    
    const regex = new RegExp(`(${query})`, 'gi');
    return text.replace(regex, '<mark>$1</mark>');
}

async function renderList() {
    const ul = $('project-list');
    ul.innerHTML = '';
    
    const filteredProjects = await filterProjects();
    const query = state.searchQuery;
    
    if (filteredProjects.length === 0) {
        const emptyLi = document.createElement('li');
        emptyLi.className = 'empty-state';
        emptyLi.innerHTML = `
            <div class="empty-icon">🔍</div>
            <div class="empty-title">No projects found</div>
            <div class="empty-subtitle">Try adjusting your search or filters</div>
        `;
        ul.appendChild(emptyLi);
        return;
    }
    
    filteredProjects.forEach((p, originalIndex) => {
        const realIndex = state.projects.indexOf(p);
        const li = document.createElement('li');
        li.className = realIndex === state.selected ? 'selected' : '';
        
        // Status dot
        const dot = document.createElement('span');
        dot.className = 'proj-dot';
        dot.dataset.index = realIndex;
        li.appendChild(dot);
        
        // Project info container
        const info = document.createElement('div');
        info.className = 'project-info';
        
        // Project name with highlight
        const name = document.createElement('div');
        name.className = 'project-name';
        name.innerHTML = highlightText(p.name, query);
        info.appendChild(name);
        
        // Project path with highlight
        const path = document.createElement('div');
        path.className = 'project-path';
        path.innerHTML = highlightText(p.path, query);
        info.appendChild(path);
        
        // Pinned indicator
        if (p.pinned) {
            const pin = document.createElement('span');
            pin.className = 'pin-indicator';
            pin.innerHTML = '📌';
            info.appendChild(pin);
        }
        
        li.appendChild(info);
        
        // Event listeners
        li.addEventListener('click', () => {
            state.selected = realIndex;
            renderList();
            renderDetail();
        });
        
        // li.addEventListener('dblclick', () => editProject(realIndex)); // Deshabilitado hasta que updateProject esté disponible
        
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
    ctx.panels.backlogPanel.onProjectChanged(p);
}

let uptimeTimer = null;
async function refreshStatus() {
    if (state.selected < 0) return;
    const status = await api.getServerStatus(state.selected);
    const badge = $('state-badge');
    badge.className = `badge ${status.state}`;
    badge.textContent = status.state;
    ['start-server', 'restart-server'].forEach((id) => {
        $(id).disabled = status.state === 'running' || status.state === 'starting';
    });
    $('stop-server').disabled = status.state === 'stopped';

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
    showAddProjectModal();
}

// async function editProject(index) {
//     if (index < 0 || index >= state.projects.length) return;
//     const p = state.projects[index];
//     const name = prompt('Project name:', p.name);
//     if (name === null) return;
//     const path = prompt('Project path:', p.path);
//     if (path === null) return;
//     if (name === p.name && path === p.path) return;
//     const errs = await api.updateProject(index, {
//         ...p,
//         name,
//         path,
//         server: p.server,
//         playwright: p.playwright,
//         pinned: p.pinned,
//     });
//     if (errs && errs.length) alert(errs.join('\n'));
//     await refreshProjects();
// }

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
    $('add-project').addEventListener('click', addProjectFlow);

    $('search').addEventListener('input', (e) => {
    state.searchQuery = e.target.value;
    debouncedSearch();
});
    $('errors-only').addEventListener('change', (e) => {
        state.errorsOnly = e.target.checked;
        reloadLogs();
    });
    $('clear-logs').addEventListener('click', () => {
        state.logs.set(state.selected, []);
        $('log-output').innerHTML = '';
    });

    $('start-server').addEventListener('click', () => api.startServer(state.selected));
    $('stop-server').addEventListener('click', () => api.stopServer(state.selected));
    $('restart-server').addEventListener('click', () => api.restartServer(state.selected));

    // $('view-project').addEventListener('click', () => switchView('project')); // No existe en el HTML
    // $('view-monitor').addEventListener('click', () => switchView('monitor')); // No existe en el HTML

    $('theme-toggle').addEventListener('click', () => {
        const cur = document.documentElement.dataset.theme || 'dark';
        applyTheme(THEME_CYCLE[(THEME_CYCLE.indexOf(cur) + 1) % THEME_CYCLE.length]);
    });
    $('settings-btn').addEventListener('click', () => settingsDialog.open());
    // $('btn-quit').addEventListener('click', quitApp); // No existe en el HTML

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
                // editProject(sel); // Deshabilitado hasta que updateProject esté disponible
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
const backlogPanel = mountBacklog(ctx);
ctx.panels = { playwrightPanel, scriptsPanel, gitPanel, evidencePanel, monitorPanel, backlogPanel };

const settingsDialog = mountSettings();
const backlogItemDialog = mountBacklogItemDialog();

// Add dialogs to DOM
document.body.appendChild(settingsDialog.getElement());
document.body.appendChild(backlogItemDialog.getElement());

// Make backlog dialog available globally
window.backlogItemDialog = backlogItemDialog;

function switchTab(name) {
    document.querySelectorAll('.tab').forEach((b) =>
        b.classList.toggle('active', b.dataset.tab === name));
    document.querySelectorAll('.panel').forEach((p) =>
        p.classList.toggle('active', p.id === `panel-${name}`));
}
document.querySelectorAll('.tab').forEach((btn) =>
    btn.addEventListener('click', () => switchTab(btn.dataset.tab)));

// Welcome Screen Functions
function showWelcomeScreen() {
    const welcomeScreen = $('welcome-screen');
    if (welcomeScreen) {
        welcomeScreen.hidden = false;
        // Add event listeners for welcome screen buttons
        wireWelcomeScreenEvents();
    }
}

function hideWelcomeScreen() {
    const welcomeScreen = $('welcome-screen');
    if (welcomeScreen) {
        welcomeScreen.hidden = true;
    }
}

function wireWelcomeScreenEvents() {
    const welcomeAddBtn = $('welcome-add-project');
    const welcomeImportBtn = $('welcome-import');
    const welcomeSkipBtn = $('welcome-skip');
    
    if (welcomeAddBtn && !welcomeAddBtn.hasAttribute('data-wired')) {
        welcomeAddBtn.addEventListener('click', () => {
            hideWelcomeScreen();
            showAddProjectModal();
        });
        welcomeAddBtn.setAttribute('data-wired', 'true');
    }
    
    if (welcomeImportBtn && !welcomeImportBtn.hasAttribute('data-wired')) {
        welcomeImportBtn.addEventListener('click', async () => {
            try {
                const folderPath = await api.browseForFolder();
                if (folderPath) {
                    const importedProjects = await api.importProjects(folderPath);
                    if (importedProjects && importedProjects.length > 0) {
                        await refreshProjects();
                        showToast(`Imported ${importedProjects.length} project(s) successfully!`, 'success');
                    } else {
                        showToast('No projects found in the selected folder', 'info');
                    }
                }
            } catch (error) {
                showToast('Failed to import projects: ' + error.message, 'error');
            }
        });
        welcomeImportBtn.setAttribute('data-wired', 'true');
    }
    
    if (welcomeSkipBtn && !welcomeSkipBtn.hasAttribute('data-wired')) {
        welcomeSkipBtn.addEventListener('click', () => {
            hideWelcomeScreen();
        });
        welcomeSkipBtn.setAttribute('data-wired', 'true');
    }
}

// Modal Functions
function showAddProjectModal() {
    const modal = $('add-project-modal');
    if (modal) {
        modal.hidden = false;
        // Clear previous values
        $('project-name').value = '';
        $('project-path').value = '';
        // Wire modal events if not already wired
        wireModalEvents();
    }
}

function hideAddProjectModal() {
    const modal = $('add-project-modal');
    if (modal) {
        modal.hidden = true;
    }
}

function wireModalEvents() {
    const cancelBtn = $('cancel-add-project');
    const confirmBtn = $('confirm-add-project');
    const closeBtn = $('modal-close');
    const browseBtn = $('browse-path');
    
    if (cancelBtn && !cancelBtn.hasAttribute('data-wired')) {
        cancelBtn.addEventListener('click', hideAddProjectModal);
        cancelBtn.setAttribute('data-wired', 'true');
    }
    
    if (closeBtn && !closeBtn.hasAttribute('data-wired')) {
        closeBtn.addEventListener('click', hideAddProjectModal);
        closeBtn.setAttribute('data-wired', 'true');
    }
    
    if (confirmBtn && !confirmBtn.hasAttribute('data-wired')) {
        confirmBtn.addEventListener('click', handleAddProject);
        confirmBtn.setAttribute('data-wired', 'true');
    }
    
    if (browseBtn && !browseBtn.hasAttribute('data-wired')) {
        browseBtn.addEventListener('click', async () => {
            try {
                const path = await api.browseForFolder();
                if (path) {
                    $('project-path').value = path;
                    // Auto-fill project name from folder name
                    const folderName = path.split(/[\\/]/).pop();
                    if (folderName && !$('project-name').value) {
                        $('project-name').value = folderName;
                    }
                }
            } catch (error) {
                showToast('Failed to browse for folder', 'error');
            }
        });
        browseBtn.setAttribute('data-wired', 'true');
    }
}

async function handleAddProject() {
    const nameInput = $('project-name');
    const pathInput = $('project-path');
    
    const name = nameInput.value.trim();
    const path = pathInput.value.trim();
    
    if (!name) {
        showToast('Project name cannot be empty', 'error');
        return;
    }
    
    if (!path) {
        showToast('Project path cannot be empty', 'error');
        return;
    }
    
    try {
        const errs = await api.addProject(name, path);
        if (errs && errs.length) {
            showToast('Error adding project: ' + errs.join(', '), 'error');
        } else {
            await refreshProjects();
            hideAddProjectModal();
            showToast('Project added successfully!', 'success');
        }
    } catch (error) {
        showToast('Failed to add project: ' + error.message, 'error');
    }
}

async function boot() {
    wireEvents();
    wireKeyboardShortcuts();
    setupKeyboardShortcuts(); // Nuevos atajos mejorados
    await refreshProjects(false);
    await settingsDialog.init(); // carga settings: tema + gate de toasts
    
    // Show welcome screen if no projects exist
    if (state.projects.length === 0) {
        showWelcomeScreen();
    }
    
    // Crear controles de filtro después de que el DOM esté listo
    setTimeout(() => {
        createFilterControls();
    }, 100);
}

boot();
