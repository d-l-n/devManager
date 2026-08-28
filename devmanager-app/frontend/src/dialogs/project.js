// Dialog completo de Proyecto (Add/Edit) — Issue #10.
// Reemplaza el prompt() con prompts hardcoded por un modal con pestañas
// General / Server / Playwright + Browse nativo + autodetección.
// Patrón widgets: mount(ctx) → { openNew, openEdit }.
import { api } from '../api.js';

const serverDefaults = () => ({
    enabled: true, command: 'npm run dev', port: 5173,
    url: 'http://localhost:5173', startup_timeout: 15000,
});
const pwDefaults = () => ({
    enabled: true,
    command: 'npx playwright test',
    ui_command: 'npx playwright test --ui',
    debug_command: 'npx playwright test --debug',
    report_command: 'npx playwright show-report',
});

function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

function labelRow(labelText, input) {
    const label = el('label', 'settings-option');
    label.appendChild(input);
    label.appendChild(el('span', '', labelText));
    return label;
}

function field(id, labelText, value, type = 'text') {
    const wrap = el('div');
    const lb = el('label', 'pf-label', labelText);
    const input = document.createElement(type === 'number' ? 'input' : 'input');
    input.type = type;
    input.id = id;
    input.className = 'text-input mono';
    if (value !== undefined) input.value = value;
    wrap.appendChild(lb);
    wrap.appendChild(input);
    return { wrap, input };
}

function sectionTitle(text) {
    return el('div', 'settings-section-title', text);
}

export function mountProjectDialog(onSaved) {
    const state = { index: -1, isEdit: false, originalPort: 0 };
    let isOpen = false;

    // ---- DOM ----
    const overlay = el('div', 'settings-overlay');
    overlay.hidden = true;
    const card = el('div', 'settings-card project-dialog-card');

    const titleEl = el('div', 'settings-title', 'Add Project');
    card.appendChild(titleEl);

    const body = el('div', 'project-dialog-body');
    card.appendChild(body);

    // --- General ---
    body.appendChild(sectionTitle('General'));
    const nameField = field('pf-name', 'Name', '', 'text');
    body.appendChild(nameField.wrap);

    const pathRow = el('div', 'pf-path-row');
    const pathField = field('pf-path', 'Path', '', 'text');
    pathRow.appendChild(pathField.wrap);
    const btnBrowse = el('button', 'btn btn-accent pf-inline-btn', 'Browse...');
    const btnDetect = el('button', 'btn btn-accent pf-inline-btn', 'Detect Auto');
    pathRow.appendChild(btnBrowse);
    pathRow.appendChild(btnDetect);
    body.appendChild(pathRow);

    const detectStatus = el('div', 'pf-status');
    body.appendChild(detectStatus);

    // --- Server ---
    body.appendChild(sectionTitle('Server'));
    const chkServer = document.createElement('input');
    chkServer.type = 'checkbox';
    chkServer.id = 'pf-server-enabled';
    body.appendChild(labelRow('Enable server management', chkServer));
    body.appendChild(field('pf-server-command', 'Command', 'npm run dev').wrap);
    body.appendChild(field('pf-server-port', 'Port', 5173, 'number').wrap);
    body.appendChild(field('pf-server-url', 'URL', 'http://localhost:5173').wrap);
    body.appendChild(field('pf-server-timeout', 'Startup Timeout (ms)', 15000, 'number').wrap);

    // --- Playwright ---
    body.appendChild(sectionTitle('Playwright'));
    const chkPw = document.createElement('input');
    chkPw.type = 'checkbox';
    chkPw.id = 'pf-pw-enabled';
    body.appendChild(labelRow('Enable Playwright integration', chkPw));
    body.appendChild(field('pf-pw-command', 'Test Command', 'npx playwright test').wrap);
    body.appendChild(field('pf-pw-ui', 'UI Command', 'npx playwright test --ui').wrap);
    body.appendChild(field('pf-pw-debug', 'Debug Command', 'npx playwright test --debug').wrap);
    body.appendChild(field('pf-pw-report', 'Report Command', 'npx playwright show-report').wrap);

    // --- Footer ---
    const footer = el('div', 'settings-footer');
    const btnOk = el('button', 'btn btn-accent', 'OK');
    const btnCancel = el('button', 'btn', 'Cancel');
    btnCancel.style.marginLeft = '8px';
    footer.appendChild(btnCancel);
    footer.appendChild(btnOk);
    card.appendChild(footer);

    overlay.appendChild(card);
    document.body.appendChild(overlay);

    // ---- Helpers ----
    const $ = (id) => document.getElementById(id);

    function resetDefaults() {
        $('pf-server-enabled').checked = serverDefaults().enabled;
        $('pf-server-command').value = serverDefaults().command;
        $('pf-server-port').value = serverDefaults().port;
        $('pf-server-url').value = serverDefaults().url;
        $('pf-server-timeout').value = serverDefaults().startup_timeout;
        $('pf-pw-enabled').checked = pwDefaults().enabled;
        $('pf-pw-command').value = pwDefaults().command;
        $('pf-pw-ui').value = pwDefaults().ui_command;
        $('pf-pw-debug').value = pwDefaults().debug_command;
        $('pf-pw-report').value = pwDefaults().report_command;
        detectStatus.textContent = '';
    }

    function setStatus(text, ok) {
        detectStatus.textContent = text || '';
        detectStatus.style.color = ok ? 'var(--ok)' : 'var(--warn)';
    }

    async function autoDetect(silentName) {
        const path = $('pf-path').value.trim();
        if (!path) {
            setStatus('Select a valid project folder first', false);
            return;
        }
        try {
            const d = await api.detectProjectConfig(path);
            if (silentName && !$('pf-name').value.trim()) {
                $('pf-name').value = d.name || '';
            }
            $('pf-server-command').value = d.server_command || '';
            $('pf-server-port').value = d.port;
            $('pf-server-url').value = d.url;
            if (d.playwright_enabled) $('pf-pw-enabled').checked = true;
            setStatus(`Detected: port ${d.port}${d.playwright_enabled ? ', Playwright found' : ''}`, true);
        } catch {
            setStatus('Detection failed', false);
        }
    }

    function openNew() {
        state.isEdit = false;
        state.index = -1;
        state.originalPort = 0;
        titleEl.textContent = 'Add Project';
        $('pf-name').value = '';
        $('pf-path').value = '';
        resetDefaults();
        overlay.hidden = false;
        isOpen = true;
        $('pf-name').focus();
    }

    function openEdit(index, project) {
        state.isEdit = true;
        state.index = index;
        titleEl.textContent = 'Edit Project';
        $('pf-name').value = project.name || '';
        $('pf-path').value = project.path || '';
        const s = project.server || serverDefaults();
        const p = project.playwright || pwDefaults();
        state.originalPort = s.port || 0;
        $('pf-server-enabled').checked = !!s.enabled;
        $('pf-server-command').value = s.command || '';
        $('pf-server-port').value = s.port ?? 5173;
        $('pf-server-url').value = s.url || '';
        $('pf-server-timeout').value = s.startup_timeout ?? 15000;
        $('pf-pw-enabled').checked = !!p.enabled;
        $('pf-pw-command').value = p.command || '';
        $('pf-pw-ui').value = p.ui_command || '';
        $('pf-pw-debug').value = p.debug_command || '';
        $('pf-pw-report').value = p.report_command || '';
        detectStatus.textContent = '';
        overlay.hidden = false;
        isOpen = true;
    }

    function close() {
        overlay.hidden = true;
        isOpen = false;
    }

    function collect() {
        return {
            name: $('pf-name').value.trim(),
            path: $('pf-path').value.trim(),
            server: {
                enabled: $('pf-server-enabled').checked,
                command: $('pf-server-command').value.trim(),
                port: parseInt($('pf-server-port').value, 10) || 0,
                url: $('pf-server-url').value.trim(),
                startup_timeout: parseInt($('pf-server-timeout').value, 10) || 15000,
            },
            playwright: {
                enabled: $('pf-pw-enabled').checked,
                command: $('pf-pw-command').value.trim(),
                ui_command: $('pf-pw-ui').value.trim(),
                debug_command: $('pf-pw-debug').value.trim(),
                report_command: $('pf-pw-report').value.trim(),
            },
        };
    }

    async function save() {
        const proj = collect();
        if (!proj.name) return window.messageDialog.alert({ title: 'Project name required', message: 'Enter a name before saving the project.', trigger: btnOk });
        if (!proj.path) return window.messageDialog.alert({ title: 'Project path required', message: 'Enter a path before saving the project.', trigger: btnOk });

        let errors = [];
        if (state.isEdit) {
            errors = await api.updateProject(state.index, proj);
        } else {
            errors = await api.addProject(proj);
        }
        if (errors && errors.length) return window.messageDialog.alert({ title: 'Could not save project', message: errors.join('\n'), trigger: btnOk });
        close();
        if (onSaved) onSaved();
        return { saved: true };
    }

    // ---- Wire ----
    btnBrowse.addEventListener('click', async () => {
        const path = await api.browseFolder();
        if (path) {
            $('pf-path').value = path;
            if (!state.isEdit) autoDetect(true);
        }
    });
    btnDetect.addEventListener('click', () => autoDetect(true));
    btnOk.addEventListener('click', save);
    btnCancel.addEventListener('click', close);
    overlay.addEventListener('mousedown', (e) => { if (e.target === overlay) close(); });
    card.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && isOpen) close();
    });
    // Enter en inputs no numericos guarda (sin repetir en number inputs)
    card.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && e.target && e.target.tagName === 'INPUT') e.preventDefault();
    });

    return { openNew, openEdit, close };
}
