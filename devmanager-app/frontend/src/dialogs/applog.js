// Dialog App Log global (Issue #14): muestra stdout/stderr capturados por el
// backend (ring de 3000) con live-updates v├¡a el evento applog:line.
import { api, events } from '../api.js';

const MAX_NODES = 3000;
const errOnlyElId = 'applog-errors-only';

function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

export function mountAppLogDialog() {
    let isOpen = false;

    const overlay = el('div', 'settings-overlay');
    overlay.hidden = true;
    const card = el('div', 'settings-card');
    card.style.width = '720px';
    card.style.maxWidth = 'calc(100vw - 32px)';

    card.appendChild(el('div', 'settings-title', 'Application Log'));

    const toolbar = el('div', 'applog-toolbar');
    const btnRefresh = el('button', 'btn btn-accent', 'Refresh');
    const btnClear = el('button', 'btn btn-danger', 'Clear');
    const labelErr = document.createElement('label');
    const chkErr = document.createElement('input');
    chkErr.type = 'checkbox';
    chkErr.id = errOnlyElId;
    labelErr.style.cssText = 'display:flex;align-items:center;gap:6px;user-select:none;';
    labelErr.appendChild(chkErr);
    labelErr.appendChild(el('span', '', 'Errors only'));
    const count = el('span', 'grow');
    count.style.color = 'var(--fg-dim)';
    const btnClose = el('button', 'btn', 'Close');
    toolbar.appendChild(btnRefresh);
    toolbar.appendChild(btnClear);
    toolbar.appendChild(labelErr);
    toolbar.appendChild(count);
    toolbar.appendChild(btnClose);
    card.appendChild(toolbar);

    const output = el('div', 'applog-output');
    card.appendChild(output);

    overlay.appendChild(card);
    document.body.appendChild(overlay);

    // ---- Helpers ----
    function appendEntry(e, force) {
        if (chkErr.checked && !e.isError && !force) return;
        const div = el('div', 'log-line' + (e.isError ? ' err' : ''));
        const ts = el('span', 'ts', `[${e.ts}]`);
        div.appendChild(ts);
        div.appendChild(document.createTextNode(e.text));
        output.appendChild(div);
        while (output.childElementCount > MAX_NODES) output.removeChild(output.firstChild);
        output.scrollTop = output.scrollHeight;
    }

    async function render(force) {
        const data = await api.getAppLog();
        output.innerHTML = '';
        (data || []).forEach((e) => appendEntry(e, force));
        count.textContent = `${(data || []).length} line(s)`;
    }

    function refresh() { render(true); }

    function open() {
        overlay.hidden = false;
        isOpen = true;
        refresh();
    }

    function close() {
        overlay.hidden = true;
        isOpen = false;
    }

    // ---- Wire ----
    btnRefresh.addEventListener('click', refresh);
    btnClear.addEventListener('click', async () => {
        await api.clearAppLog();
        output.innerHTML = '';
        count.textContent = '0 line(s)';
    });
    chkErr.addEventListener('change', () => refresh());
    btnClose.addEventListener('click', close);
    overlay.addEventListener('mousedown', (e) => { if (e.target === overlay) close(); });
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && isOpen) close(); });

    // Live updates desde Go
    events().EventsOn('applog:line', (entry) => {
        if (isOpen && entry && entry.text) appendEntry(entry, false);
    });

    return { open, close, isOpen: () => isOpen };
}
