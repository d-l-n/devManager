// Dialog completo de Proyecto (Add/Edit) — Issue #10.
// Reemplaza el prompt() con prompts hardcoded por un modal con pestañas
// General / Server / Playwright + Browse nativo + autodetección.
// Patrón widgets: mount(ctx) → { openNew, openEdit }.
import { api } from '../api.js';
import { defaultEnvFile, ensureEnvs, isValidEnvName } from '../envs.js';

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
const userDefaults = () => ({ enabled: true, command: '' });

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
    lb.htmlFor = id; // asocia el label al input (a11y)
    const input = document.createElement('input');
    input.type = type;
    input.id = id;
    input.className = 'text-input mono';
    if (value !== undefined) input.value = value;
    const err = el('span', 'pf-error');
    err.id = id + '-error';
    wrap.appendChild(lb);
    wrap.appendChild(input);
    wrap.appendChild(err);
    return { wrap, input, err };
}

// Muestra/limpia errores inline por campo.
function setFieldError(err, message) {
    if (err) err.textContent = message || '';
    if (err) err.style.display = (message ? 'block' : 'none');
}

function sectionTitle(text) {
    return el('div', 'settings-section-title', text);
}

const cloneEnv = (e) => JSON.parse(JSON.stringify(e || { server: {}, playwright: {}, user: {} }));

export function mountProjectDialog(onSaved) {
    const state = { index: -1, isEdit: false, originalPort: 0, envs: {}, activeEnv: 'dev', selectedEnv: 'dev' };
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
    nameField.input.required = true;
    nameField.input.setAttribute('aria-describedby', 'pf-name-error');
    body.appendChild(nameField.wrap);

    const pathRow = el('div', 'pf-path-row');
    const pathField = field('pf-path', 'Path', '', 'text');
    pathField.input.required = true;
    body.appendChild(pathField.wrap);
    const btnBrowse = el('button', 'btn btn-accent pf-inline-btn', 'Browse...');
    const btnDetect = el('button', 'btn btn-accent pf-inline-btn', 'Detect Auto');
    pathRow.appendChild(btnBrowse);
    pathRow.appendChild(btnDetect);
    body.appendChild(pathRow);

    const detectStatus = el('div', 'pf-status');
    body.appendChild(detectStatus);

    // --- Environments (Fase 1 #67) ---
    body.appendChild(sectionTitle('Environments'));
    const envHint = el('div', 'pf-status');
    envHint.textContent = 'Editing env drives the Server/Playwright/User fields below. ★ = active.';
    body.appendChild(envHint);
    const envList = el('div', 'pf-env-list');
    envList.id = 'pf-env-list';
    body.appendChild(envList);
    const envAddRow = el('div', 'pf-path-row');
    const envNew = document.createElement('input');
    envNew.id = 'pf-env-new';
    envNew.className = 'text-input mono';
    envNew.placeholder = 'new-env-name';
    envNew.setAttribute('aria-label', 'New environment name');
    const envCopy = document.createElement('select');
    envCopy.id = 'pf-env-copy';
    envCopy.className = 'text-input mono';
    envCopy.title = 'Copy from';
    envCopy.setAttribute('aria-label', 'Copy new environment from');
    const btnEnvAdd = el('button', 'btn btn-accent pf-inline-btn', 'Add env');
    btnEnvAdd.id = 'pf-env-add';
    envAddRow.appendChild(envNew);
    envAddRow.appendChild(envCopy);
    envAddRow.appendChild(btnEnvAdd);
    body.appendChild(envAddRow);
    const envEditRow = el('div', 'pf-path-row');
    const envRename = document.createElement('input');
    envRename.id = 'pf-env-rename';
    envRename.className = 'text-input mono';
    envRename.placeholder = 'rename selected to...';
    envRename.setAttribute('aria-label', 'Rename selected environment');
    const btnEnvRename = el('button', 'btn pf-inline-btn', 'Rename');
    btnEnvRename.id = 'pf-env-rename-btn';
    const btnEnvActive = el('button', 'btn pf-inline-btn', 'Make active');
    btnEnvActive.id = 'pf-env-make-active-btn';
    const btnEnvDelete = el('button', 'btn pf-inline-btn', 'Delete');
    btnEnvDelete.id = 'pf-env-delete-btn';
    envEditRow.appendChild(envRename);
    envEditRow.appendChild(btnEnvRename);
    envEditRow.appendChild(btnEnvActive);
    envEditRow.appendChild(btnEnvDelete);
    body.appendChild(envEditRow);
    const envError = el('span', 'pf-error');
    envError.id = 'pf-env-error';
    body.appendChild(envError);

    // --- Server ---
    body.appendChild(sectionTitle('Server'));
    const chkServer = document.createElement('input');
    chkServer.type = 'checkbox';
    chkServer.id = 'pf-server-enabled';
    body.appendChild(labelRow('Enable server management', chkServer));
    body.appendChild(field('pf-server-command', 'Command', 'npm run dev').wrap);
    const portField = field('pf-server-port', 'Port', 5173, 'number');
    portField.input.min = 1;
    portField.input.max = 65535;
    body.appendChild(portField.wrap);
    const urlField = field('pf-server-url', 'URL', 'http://localhost:5173', 'url');
    body.appendChild(urlField.wrap);
    const timeoutField = field('pf-server-timeout', 'Startup Timeout (ms)', 15000, 'number');
    body.appendChild(timeoutField.wrap);

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

    // --- User (Create User feature) ---
    body.appendChild(sectionTitle('User'));
    const chkUser = document.createElement('input');
    chkUser.type = 'checkbox';
    chkUser.id = 'pf-user-enabled';
    body.appendChild(labelRow('Enable user creation', chkUser));
    const userCmdField = field('pf-user-command', 'Create User Command', '', 'text');
    userCmdField.input.placeholder = 'npm run create-user';
    userCmdField.input.title =
        'Runs from the project folder with env vars: DM_USER_EMAIL, DM_USER_NAME, DM_USER_PASSWORD, DM_USER_ROLE';
    body.appendChild(userCmdField.wrap);
    const userCmdRow = el('div', 'pf-path-row');
    const btnDetectUser = el('button', 'btn btn-accent pf-inline-btn', 'Detect');
    const userDetectHint = el('span', 'pf-status');
    userCmdRow.appendChild(btnDetectUser);
    userCmdRow.appendChild(userDetectHint);
    body.appendChild(userCmdRow);

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
        $('pf-user-enabled').checked = userDefaults().enabled;
        $('pf-user-command').value = userDefaults().command;
        userDetectHint.textContent = '';
        detectStatus.textContent = '';
    }

    function setStatus(text, ok) {
        detectStatus.textContent = text || '';
        detectStatus.style.color = ok ? 'var(--ok)' : 'var(--warn)';
    }

    function setEnvError(message) {
        setFieldError(envError, message);
    }

    function loadSelectedIntoInputs() {
        const e = state.envs[state.selectedEnv] || { server: {}, playwright: {}, user: {} };
        const s = e.server || {};
        const p = e.playwright || {};
        const u = e.user || {};
        const sd = serverDefaults();
        const pd = pwDefaults();
        $('pf-server-enabled').checked = s.enabled ?? sd.enabled;
        $('pf-server-command').value = s.command ?? sd.command;
        $('pf-server-port').value = s.port ?? sd.port;
        $('pf-server-url').value = s.url ?? sd.url;
        $('pf-server-timeout').value = s.startup_timeout ?? sd.startup_timeout;
        $('pf-pw-enabled').checked = p.enabled ?? pd.enabled;
        $('pf-pw-command').value = p.command ?? pd.command;
        $('pf-pw-ui').value = p.ui_command ?? pd.ui_command;
        $('pf-pw-debug').value = p.debug_command ?? pd.debug_command;
        $('pf-pw-report').value = p.report_command ?? pd.report_command;
        $('pf-user-enabled').checked = u.enabled ?? true;
        $('pf-user-command').value = u.command ?? '';
    }

    function saveCurrentIntoSelected() {
        if (!state.envs[state.selectedEnv]) return;
        // Fase 2/3: preserva vars/env_file/secrets (se editan en env-vars).
        const prev = state.envs[state.selectedEnv] || {};
        state.envs[state.selectedEnv] = {
            vars: { ...(prev.vars || {}) },
            env_file: prev.env_file || defaultEnvFile(state.selectedEnv),
            secrets: [...(prev.secrets || [])],
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
            user: {
                enabled: $('pf-user-enabled').checked,
                command: $('pf-user-command').value.trim(),
            },
        };
    }

    function renderEnvList() {
        const names = Object.keys(state.envs).sort();
        envList.innerHTML = '';
        names.forEach((n) => {
            const b = document.createElement('button');
            b.type = 'button';
            b.className = 'btn pf-inline-btn' + (n === state.selectedEnv ? ' btn-accent' : '');
            b.dataset.envName = n;
            b.textContent = n === state.activeEnv ? `★ ${n}` : n;
            b.title = n === state.activeEnv ? `${n} (active)` : `Edit ${n}`;
            b.setAttribute('aria-pressed', n === state.selectedEnv ? 'true' : 'false');
            b.addEventListener('click', () => {
                if (state.selectedEnv !== n) {
                    saveCurrentIntoSelected();
                    state.selectedEnv = n;
                    renderEnvList();
                    loadSelectedIntoInputs();
                }
            });
            envList.appendChild(b);
        });
        envCopy.innerHTML = '';
        names.forEach((n) => {
            const opt = document.createElement('option');
            opt.value = n;
            opt.textContent = n;
            envCopy.appendChild(opt);
        });
        const count = names.length;
        const selIsActive = state.selectedEnv === state.activeEnv;
        btnEnvDelete.disabled = selIsActive || count <= 1;
        btnEnvDelete.title = selIsActive ? 'Cannot delete the active environment'
            : (count <= 1 ? 'Cannot delete the last environment' : `Delete ${state.selectedEnv}`);
        btnEnvRename.disabled = selIsActive;
        btnEnvRename.title = selIsActive ? 'Cannot rename the active environment' : `Rename ${state.selectedEnv}`;
        btnEnvActive.disabled = selIsActive;
        btnEnvActive.title = selIsActive ? 'Already active' : `Make ${state.selectedEnv} active`;
    }

    // Autodetecta el comando de creación de usuario (user.command) para el
    // path actual. Prefill solo si el campo está vacío; silent=true no pinta
    // el hint cuando no hay match (evita ruido en autodetección automática).
    async function detectUserCommand(silent) {
        const path = $('pf-path').value.trim();
        if (!path) return '';
        try {
            const cmd = await api.detectUserCommand(path);
            if (cmd) {
                if (!$('pf-user-command').value.trim()) $('pf-user-command').value = cmd;
                userDetectHint.textContent = `Detected: ${cmd}`;
                userDetectHint.style.color = 'var(--ok)';
            } else {
                userDetectHint.textContent = silent ? '' : 'No create-user command found';
                userDetectHint.style.color = 'var(--warn)';
            }
            return cmd;
        } catch {
            userDetectHint.textContent = 'Detection failed';
            userDetectHint.style.color = 'var(--warn)';
            return '';
        }
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
            detectUserCommand(true);
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
        state.envs = { dev: { server: serverDefaults(), playwright: pwDefaults(), user: userDefaults(), vars: {}, env_file: defaultEnvFile('dev'), secrets: [] } };
        state.activeEnv = 'dev';
        state.selectedEnv = 'dev';
        envNew.value = '';
        envRename.value = '';
        setEnvError('');
        renderEnvList();
        loadSelectedIntoInputs();
        overlay.hidden = false;
        isOpen = true;
        setFieldError(nameField.err, '');
        setFieldError(pathField.err, '');
        setFieldError(portField.err, '');
        setFieldError(urlField.err, '');
        setFieldError(timeoutField.err, '');
        $('pf-name').focus();
    }

    function openEdit(index, project) {
        state.isEdit = true;
        state.index = index;
        titleEl.textContent = 'Edit Project';
        $('pf-name').value = project.name || '';
        $('pf-path').value = project.path || '';
        const ensured = ensureEnvs(project);
        state.envs = JSON.parse(JSON.stringify(ensured.envs));
        state.activeEnv = ensured.active_env;
        state.selectedEnv = ensured.active_env;
        envNew.value = '';
        envRename.value = '';
        setEnvError('');
        renderEnvList();
        loadSelectedIntoInputs();
        const selServer = (state.envs[state.selectedEnv] && state.envs[state.selectedEnv].server) || {};
        state.originalPort = selServer.port || 0;
        userDetectHint.textContent = '';
        detectStatus.textContent = '';
        // Comando de creación de usuario vacío → intentar autodetección
        if (!((state.envs[state.selectedEnv] && state.envs[state.selectedEnv].user || {}).command)) detectUserCommand(true);
        overlay.hidden = false;
        isOpen = true;
        setFieldError(nameField.err, '');
        setFieldError(pathField.err, '');
        setFieldError(portField.err, '');
        setFieldError(urlField.err, '');
        setFieldError(timeoutField.err, '');
    }

    function close() {
        overlay.hidden = true;
        isOpen = false;
    }

    function collect() {
        saveCurrentIntoSelected();
        const active = cloneEnv(state.envs[state.activeEnv]);
        return {
            name: $('pf-name').value.trim(),
            path: $('pf-path').value.trim(),
            server: { ...active.server },
            playwright: { ...active.playwright },
            user: { ...active.user },
            active_env: state.activeEnv,
            envs: JSON.parse(JSON.stringify(state.envs)),
        };
    }

    function validate() {
        const errors = [];
        setFieldError(nameField.err, '');
        setFieldError(pathField.err, '');
        setFieldError(portField.err, '');
        setFieldError(urlField.err, '');
        setFieldError(timeoutField.err, '');

        if (!$('pf-name').value.trim()) {
            setFieldError(nameField.err, 'Name is required');
            errors.push('Name is required');
        }
        if (!$('pf-path').value.trim()) {
            setFieldError(pathField.err, 'Path is required');
            errors.push('Path is required');
        }
        const port = parseInt($('pf-server-port').value, 10);
        if (isNaN(port) || port < 1 || port > 65535) {
            setFieldError(portField.err, 'Port must be 1-65535');
            errors.push('Port must be 1-65535');
        }
        const pUrl = $('pf-server-url').value.trim();
        if (pUrl && !/^https?:\/\/.+/i.test(pUrl)) {
            setFieldError(urlField.err, 'URL must start with http:// or https://');
            errors.push('URL must be a valid http(s) URL');
        }
        const to = parseInt($('pf-server-timeout').value, 10);
        if (isNaN(to) || to < 0) {
            setFieldError(timeoutField.err, 'Timeout must be a positive number');
            errors.push('Startup timeout must be a non-negative number');
        }
        setEnvError('');
        const names = Object.keys(state.envs || {});
        if (names.length === 0) {
            setEnvError('At least one environment is required');
            errors.push('At least one environment is required');
        }
        names.forEach((n) => {
            if (!isValidEnvName(n)) {
                setEnvError(`Invalid environment name "${n}" (use [a-z0-9_-]{1,32})`);
                errors.push(`Invalid environment name "${n}"`);
            }
        });
        if (!state.activeEnv || !state.envs[state.activeEnv]) {
            setEnvError('Active environment does not exist');
            errors.push('Active environment does not exist');
        }
        return errors;
    }

    async function save() {
        const errors = validate();
        if (errors.length) return false;

        const proj = collect();
        if (!proj.name) return window.messageDialog.alert({ title: 'Project name required', message: 'Enter a name before saving the project.', trigger: btnOk });
        if (!proj.path) return window.messageDialog.alert({ title: 'Project path required', message: 'Enter a path before saving the project.', trigger: btnOk });

        let errs = [];
        if (state.isEdit) {
            errs = await api.updateProject(state.index, proj);
        } else {
            errs = await api.addProject(proj);
        }
        if (errs && errs.length) return window.messageDialog.alert({ title: 'Could not save project', message: errs.join('\n'), trigger: btnOk });
        close();
        if (onSaved) onSaved(state.isEdit ? state.index : -1);
        return { saved: true };
    }

    // ---- Wire envs ----
    btnEnvAdd.addEventListener('click', () => {
        const name = envNew.value.trim();
        setEnvError('');
        if (!isValidEnvName(name)) {
            setEnvError('Invalid name (use [a-z0-9_-]{1,32})');
            return;
        }
        if (state.envs[name]) {
            setEnvError(`Environment "${name}" already exists`);
            return;
        }
        const from = envCopy.value && state.envs[envCopy.value] ? envCopy.value : state.selectedEnv;
        saveCurrentIntoSelected();
        state.envs[name] = cloneEnv(state.envs[from]);
        state.selectedEnv = name;
        envNew.value = '';
        renderEnvList();
        loadSelectedIntoInputs();
    });
    btnEnvRename.addEventListener('click', () => {
        const next = envRename.value.trim();
        const cur = state.selectedEnv;
        setEnvError('');
        if (cur === state.activeEnv) {
            setEnvError('Cannot rename the active environment');
            return;
        }
        if (!isValidEnvName(next)) {
            setEnvError('Invalid name (use [a-z0-9_-]{1,32})');
            return;
        }
        if (state.envs[next]) {
            setEnvError(`Environment "${next}" already exists`);
            return;
        }
        saveCurrentIntoSelected();
        state.envs[next] = state.envs[cur];
        delete state.envs[cur];
        state.selectedEnv = next;
        envRename.value = '';
        renderEnvList();
        loadSelectedIntoInputs();
    });
    btnEnvActive.addEventListener('click', () => {
        saveCurrentIntoSelected();
        state.activeEnv = state.selectedEnv;
        renderEnvList();
    });
    btnEnvDelete.addEventListener('click', () => {
        const cur = state.selectedEnv;
        setEnvError('');
        if (cur === state.activeEnv) {
            setEnvError('Cannot delete the active environment');
            return;
        }
        if (Object.keys(state.envs).length <= 1) {
            setEnvError('Cannot delete the last environment');
            return;
        }
        delete state.envs[cur];
        state.selectedEnv = state.activeEnv;
        renderEnvList();
        loadSelectedIntoInputs();
    });

    // ---- Wire ----
    btnBrowse.addEventListener('click', async () => {
        const path = await api.browseFolder();
        if (path) {
            $('pf-path').value = path;
            if (!state.isEdit) autoDetect(true);
        }
    });
    btnDetect.addEventListener('click', () => autoDetect(true));
    btnDetectUser.addEventListener('click', () => detectUserCommand(false));
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

    return {
        openNew, openEdit, close,
        getElement: () => overlay,
        collect, validate,
        _state: state,
    };
}
