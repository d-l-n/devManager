// Dialog de variables de entorno por env (Fase 2 #67, secrets+diff Fase 3).
// Tabla key/value editable + add/delete, validación de keys + duplicadas,
// Load desde archivo / Save a archivo / Save a proyecto.
// Secretos: checkbox por fila, valor enmascarado salvo reveal explícito.
// Diff: selects A/B + vista side-by-side read-only.
// Patrón project.js: mount(ctx, onSaved?) → { open, close }.
// ctx: { api, messageDialog? }. Toasts vía window.showToast si existe.
import { MASKED, isSecretKey, isValidVarKey, isServerRunning } from '../envs.js';

function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

function toast(title, message, level) {
    if (window.showToast) window.showToast(title, message, level);
}

export function mount(ctx, onSaved) {
    const api = (ctx && ctx.api) || {};
    const msg = (ctx && ctx.messageDialog) || window.messageDialog;
    const state = { index: -1, env: '', running: false, secrets: [], revealed: false, envNames: [] };
    let isOpen = false;

    // ---- DOM ----
    const overlay = el('div', 'settings-overlay');
    overlay.hidden = true;
    const card = el('div', 'settings-card project-dialog-card');

    const titleEl = el('div', 'settings-title', 'Environment Variables');
    card.appendChild(titleEl);

    const runBanner = el('div', 'pf-status');
    runBanner.id = 'ev-running-banner';
    card.appendChild(runBanner);

    const body = el('div', 'project-dialog-body');
    card.appendChild(body);

    const rows = el('div', 'ev-rows');
    rows.id = 'ev-rows';
    body.appendChild(rows);

    const addRow = el('div', 'pf-path-row');
    const newKey = document.createElement('input');
    newKey.id = 'ev-new-key';
    newKey.className = 'text-input mono';
    newKey.placeholder = 'NEW_KEY';
    newKey.setAttribute('aria-label', 'New variable name');
    newKey.autocomplete = 'off';
    const newVal = document.createElement('input');
    newVal.id = 'ev-new-value';
    newVal.className = 'text-input mono';
    newVal.placeholder = 'value';
    newVal.setAttribute('aria-label', 'New variable value');
    newVal.autocomplete = 'off';
    const btnAdd = el('button', 'btn btn-accent pf-inline-btn', 'Add');
    btnAdd.id = 'ev-add';
    addRow.appendChild(newKey);
    addRow.appendChild(newVal);
    addRow.appendChild(btnAdd);
    body.appendChild(addRow);

    const errEl = el('span', 'pf-error');
    errEl.id = 'ev-error';
    body.appendChild(errEl);

    // ---- Diff (Fase 3): selects A/B + vista side-by-side read-only ----
    const diffTitle = el('div', 'settings-section-title', 'Compare environments');
    body.appendChild(diffTitle);
    const diffRow = el('div', 'pf-path-row');
    const diffA = document.createElement('select');
    diffA.id = 'ev-diff-a';
    diffA.className = 'text-input mono';
    diffA.setAttribute('aria-label', 'Compare from environment');
    const diffB = document.createElement('select');
    diffB.id = 'ev-diff-b';
    diffB.className = 'text-input mono';
    diffB.setAttribute('aria-label', 'Compare to environment');
    const btnCompare = el('button', 'btn pf-inline-btn', 'Compare');
    btnCompare.id = 'ev-compare';
    diffRow.appendChild(diffA);
    diffRow.appendChild(diffB);
    diffRow.appendChild(btnCompare);
    body.appendChild(diffRow);
    const diffOut = el('div', 'ev-diff-out');
    diffOut.id = 'ev-diff-out';
    body.appendChild(diffOut);

    const footer = el('div', 'settings-footer');
    const btnLoad = el('button', 'btn', 'Load from file');
    btnLoad.id = 'ev-load';
    btnLoad.title = 'Replace table with the project dotenv file content';
    const btnSaveFile = el('button', 'btn', 'Save to file');
    btnSaveFile.id = 'ev-save-file';
    btnSaveFile.title = 'Write table to the project dotenv file';
    const btnReveal = el('button', 'btn', 'Reveal');
    btnReveal.id = 'ev-reveal';
    btnReveal.title = 'Show real secret values (explicit only)';
    const btnSave = el('button', 'btn btn-accent', 'Save to project');
    btnSave.id = 'ev-save';
    const btnCancel = el('button', 'btn', 'Close');
    btnCancel.id = 'ev-cancel';
    btnCancel.style.marginLeft = '8px';
    footer.appendChild(btnLoad);
    footer.appendChild(btnSaveFile);
    footer.appendChild(btnReveal);
    footer.appendChild(btnSave);
    footer.appendChild(btnCancel);
    card.appendChild(footer);

    overlay.appendChild(card);
    document.body.appendChild(overlay);

    // ---- Helpers ----
    function setError(message) {
        errEl.textContent = message || '';
        errEl.style.display = message ? 'block' : 'none';
    }

    function collect() {
        const out = {};
        rows.querySelectorAll('.ev-row').forEach((r) => {
            const k = r.querySelector('.ev-key').value.trim();
            const v = r.querySelector('.ev-value').value;
            if (k) out[k] = v;
        });
        return out;
    }

    function validate() {
        const errors = [];
        setError('');
        const seen = new Set();
        rows.querySelectorAll('.ev-row').forEach((r) => {
            const keyInput = r.querySelector('.ev-key');
            const k = keyInput.value.trim();
            keyInput.removeAttribute('aria-invalid');
            if (!k) {
                keyInput.setAttribute('aria-invalid', 'true');
                errors.push('Empty variable name');
                return;
            }
            if (!isValidVarKey(k)) {
                keyInput.setAttribute('aria-invalid', 'true');
                errors.push(`Invalid variable name "${k}" (use ^[A-Z_][A-Z0-9_]*$)`);
                return;
            }
            if (seen.has(k)) {
                keyInput.setAttribute('aria-invalid', 'true');
                errors.push(`Duplicate variable "${k}"`);
                return;
            }
            seen.add(k);
        });
        const nk = newKey.value.trim();
        if (nk && !isValidVarKey(nk)) errors.push(`Invalid variable name "${nk}"`);
        if (errors.length) setError(errors[0]);
        return errors;
    }

    function addRowEl(key, value, secret) {
        const r = el('div', 'ev-row pf-path-row');
        const k = document.createElement('input');
        k.className = 'text-input mono ev-key';
        k.value = key || '';
        k.placeholder = 'KEY';
        k.autocomplete = 'off';
        k.setAttribute('aria-label', 'Variable name');
        const v = document.createElement('input');
        v.className = 'text-input mono ev-value';
        v.value = value !== undefined ? value : '';
        v.placeholder = 'value';
        v.autocomplete = 'off';
        v.setAttribute('aria-label', `Value for ${key || 'variable'}`);
        const isSec = !!secret;
        if (isSec) v.type = 'password';
        const secLabel = el('label', 'settings-option ev-secret-label');
        const sec = document.createElement('input');
        sec.type = 'checkbox';
        sec.className = 'ev-secret';
        sec.checked = isSec;
        sec.title = 'Secret: mask value';
        sec.setAttribute('aria-label', `Secret for ${key || 'variable'}`);
        secLabel.appendChild(sec);
        secLabel.appendChild(el('span', '', 'secret'));
        sec.addEventListener('change', async () => {
            const kk = k.value.trim();
            if (!kk) {
                sec.checked = !sec.checked;
                setError('Enter a variable name before marking secret');
                return;
            }
            setError('');
            try {
                const errs = sec.checked
                    ? (api.setSecretKey ? await api.setSecretKey(state.index, state.env, kk) : [])
                    : (api.unsetSecretKey ? await api.unsetSecretKey(state.index, state.env, kk) : []);
                if (errs && errs.length) {
                    sec.checked = !sec.checked;
                    setError(errs.join('\n'));
                    return;
                }
            } catch (e) {
                sec.checked = !sec.checked;
                setError((e && e.message) || String(e));
                return;
            }
            if (sec.checked) {
                if (!state.secrets.includes(kk)) state.secrets.push(kk);
                v.type = 'password';
                if (v.value !== MASKED && !state.revealed) v.value = MASKED;
            } else {
                state.secrets = state.secrets.filter((s) => s !== kk);
                v.type = 'text';
            }
        });
        const del = el('button', 'btn pf-inline-btn ev-del', 'Delete');
        del.title = key ? `Delete ${key}` : 'Delete row';
        del.addEventListener('click', () => r.remove());
        r.appendChild(k);
        r.appendChild(v);
        r.appendChild(secLabel);
        r.appendChild(del);
        rows.appendChild(r);
        return r;
    }

    function renderVars(vars, secrets) {
        rows.innerHTML = '';
        const secs = secrets !== undefined ? secrets : state.secrets;
        Object.keys(vars || {}).sort().forEach((k) => addRowEl(k, vars[k], isSecretKey(secs, k)));
        setError('');
    }

    function renderDiffSelects() {
        const names = state.envNames.length ? state.envNames : [state.env];
        [diffA, diffB].forEach((sel) => {
            sel.innerHTML = '';
            names.forEach((n) => {
                const opt = document.createElement('option');
                opt.value = n;
                opt.textContent = n;
                sel.appendChild(opt);
            });
        });
        diffA.value = state.env;
        const other = names.find((n) => n !== state.env);
        if (other) diffB.value = other;
        diffOut.innerHTML = '';
    }

    function renderDiff(rowsData, envA, envB) {
        diffOut.innerHTML = '';
        if (!rowsData || !rowsData.length) {
            diffOut.appendChild(el('div', 'pf-status', 'No differences'));
            return;
        }
        const table = document.createElement('table');
        table.className = 'ev-diff-table';
        const head = document.createElement('tr');
        ['', 'Key', envA, envB].forEach((h) => {
            const th = document.createElement('th');
            th.textContent = h;
            head.appendChild(th);
        });
        table.appendChild(head);
        rowsData.forEach((row) => {
            const tr = document.createElement('tr');
            tr.className = `ev-diff-row ev-diff-${row.status || 'same'}`;
            tr.dataset.diffKey = row.key;
            tr.dataset.diffStatus = row.status;
            const badge = el('td', `badge ev-diff-status`, row.status);
            const tdK = el('td', 'mono', row.key);
            const tdA = el('td', 'mono', row.a !== undefined ? row.a : '');
            const tdB = el('td', 'mono', row.b !== undefined ? row.b : '');
            tr.appendChild(badge);
            tr.appendChild(tdK);
            tr.appendChild(tdA);
            tr.appendChild(tdB);
            table.appendChild(tr);
        });
        diffOut.appendChild(table);
    }

    function refreshBanner() {
        runBanner.textContent = state.running
            ? 'Server running: "Save to project" is disabled until it stops.'
            : '';
        runBanner.style.color = 'var(--warn)';
        btnSave.disabled = state.running;
        btnSave.title = state.running
            ? 'Stop the server to save variables to the project'
            : 'Save variables to the project';
    }

    async function open(index, envName, initialVars, extra) {
        state.index = index;
        state.env = envName || '';
        titleEl.textContent = `Environment Variables — ${state.env}`;
        state.running = false;
        state.revealed = false;
        btnReveal.textContent = 'Reveal';
        const ex = extra || {};
        try {
            if (api.getServerStatus) {
                const st = await api.getServerStatus(index);
                state.running = !!(st && isServerRunning(st.state));
            }
        } catch { /* sin backend: edición habilitada */ }
        refreshBanner();
        // Secrets: explícitos o fetch (solo keys, nunca valores).
        if (Array.isArray(ex.secrets)) {
            state.secrets = [...ex.secrets];
        } else {
            try {
                state.secrets = api.getSecrets ? (await api.getSecrets(index, state.env)) || [] : [];
            } catch {
                state.secrets = [];
            }
        }
        state.envNames = Array.isArray(ex.envNames) && ex.envNames.length ? [...ex.envNames] : [state.env];
        let vars = initialVars;
        if (vars === undefined) {
            try {
                vars = api.getEnvVars ? await api.getEnvVars(index, state.env, false) : {};
            } catch {
                vars = {};
            }
        }
        renderVars(vars || {}, state.secrets);
        renderDiffSelects();
        newKey.value = '';
        newVal.value = '';
        overlay.hidden = false;
        isOpen = true;
        if (newKey.focus) newKey.focus();
    }

    function close() {
        overlay.hidden = true;
        isOpen = false;
    }

    async function saveToProject() {
        const errors = validate();
        if (errors.length) return false;
        if (state.running) {
            toast('Environment Variables', 'Stop the server before saving to the project', 'warning');
            return false;
        }
        const vars = collect();
        let errs = null;
        try {
            errs = await api.setEnvVars(state.index, state.env, vars);
        } catch (e) {
            toast('Environment Variables', (e && e.message) || String(e), 'error');
            return false;
        }
        if (errs && errs.length) {
            if (msg && msg.alert) {
                await msg.alert({ title: 'Could not save variables', message: errs.join('\n'), trigger: btnSave });
            } else {
                setError(errs.join('\n'));
            }
            return false;
        }
        toast('Environment Variables', `Saved ${Object.keys(vars).length} variable(s) to ${state.env}`, 'success');
        close();
        if (onSaved) onSaved(state.index, state.env);
        return { saved: true };
    }

    // ---- Wire ----
    btnAdd.addEventListener('click', () => {
        const k = newKey.value.trim();
        setError('');
        if (!k) {
            setError('Enter a variable name');
            return;
        }
        if (!isValidVarKey(k)) {
            setError(`Invalid variable name "${k}" (use ^[A-Z_][A-Z0-9_]*$)`);
            return;
        }
        const existing = [...rows.querySelectorAll('.ev-key')].map((i) => i.value.trim());
        if (existing.includes(k)) {
            setError(`Duplicate variable "${k}"`);
            return;
        }
        addRowEl(k, newVal.value);
        newKey.value = '';
        newVal.value = '';
        newKey.focus();
    });
    btnLoad.addEventListener('click', async () => {
        if (!api.loadDotEnvFile) {
            setError('Load from file not available');
            return;
        }
        let vars = {};
        try {
            vars = await api.loadDotEnvFile(state.index, state.env);
        } catch (e) {
            setError((e && e.message) || 'Load failed');
            return;
        }
        renderVars(vars || {});
        toast('Environment Variables', `Loaded ${Object.keys(vars || {}).length} variable(s) from file`, 'info');
    });
    btnSaveFile.addEventListener('click', async () => {
        const errors = validate();
        if (errors.length) return false;
        if (!api.saveDotEnvFile) {
            setError('Save to file not available');
            return;
        }
        const vars = collect();
        let errs = null;
        try {
            errs = await api.saveDotEnvFile(state.index, state.env, vars);
        } catch (e) {
            setError((e && e.message) || 'Save failed');
            return false;
        }
        if (errs && errs.length) {
            setError(errs.join('\n'));
            return false;
        }
        toast('Environment Variables', `Wrote ${Object.keys(vars).length} variable(s) to file`, 'success');
        return { saved: true };
    });
    btnReveal.addEventListener('click', async () => {
        if (!api.getEnvVars) {
            setError('Reveal not available');
            return;
        }
        state.revealed = !state.revealed;
        btnReveal.textContent = state.revealed ? 'Hide' : 'Reveal';
        try {
            // Reveal explícito: único path que trae valores reales.
            const vars = await api.getEnvVars(state.index, state.env, state.revealed);
            renderVars(vars || {}, state.secrets);
        } catch (e) {
            state.revealed = !state.revealed;
            btnReveal.textContent = state.revealed ? 'Hide' : 'Reveal';
            setError((e && e.message) || 'Reveal failed');
        }
    });
    btnCompare.addEventListener('click', async () => {
        if (!api.getEnvDiff) {
            setError('Compare not available');
            return;
        }
        const a = diffA.value;
        const b = diffB.value;
        setError('');
        try {
            // El backend enmascara secretos: la vista nunca ve reales.
            const rowsData = await api.getEnvDiff(state.index, a, b);
            renderDiff(rowsData || [], a, b);
        } catch (e) {
            setError((e && e.message) || 'Compare failed');
        }
    });
    btnSave.addEventListener('click', saveToProject);
    btnCancel.addEventListener('click', close);
    overlay.addEventListener('mousedown', (e) => { if (e.target === overlay) close(); });
    card.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && isOpen) close();
    });

    return {
        open, close,
        getElement: () => overlay,
        collect, validate,
        _state: state,
    };
}
