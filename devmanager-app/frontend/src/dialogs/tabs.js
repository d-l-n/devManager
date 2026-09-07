import { setIcon } from '../icons.js';

// Dialog de personalización de tabs del detail view (por proyecto):
// visibilidad (hidden) + orden. Guarda en project.tabs via updateProject.
// Logs no se puede ocultar (fallback del tab activo).
// Patrón widgets: mount(onSaved) → { open, close, getElement }.

export const TABS = [
    { id: 'logs', label: 'Logs', customizable: false },
    { id: 'scripts', label: 'Scripts', customizable: true },
    { id: 'git', label: 'Git', customizable: true },
    { id: 'deps', label: 'Deps', customizable: true },
    { id: 'playwright', label: 'Playwright', customizable: true },
    { id: 'evidence', label: 'Evidence', customizable: true },
    { id: 'obscura', label: 'Obscura', customizable: true },
    { id: 'backlog', label: 'Backlog', customizable: true },
];

export const CUSTOMIZABLE = TABS.filter((t) => t.customizable);

function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

// Merge: orden guardado primero (ids conocidos, sin duplicados),
// resto en orden default al final.
export function mergeOrder(savedOrder) {
    const seen = new Set();
    const out = [];
    for (const id of savedOrder || []) {
        if (CUSTOMIZABLE.some((t) => t.id === id) && !seen.has(id)) {
            out.push(id);
            seen.add(id);
        }
    }
    for (const t of CUSTOMIZABLE) {
        if (!seen.has(t.id)) out.push(t.id);
    }
    return out;
}

export function mountTabsDialog(onSaved, saver) {
    let isOpen = false;
    let projectIndex = -1;
    let project = null;
    let order = [];
    let hiddenSet = new Set();
    let saverFn = saver;

    // ---- DOM ----
    const overlay = el('div', 'dialog-overlay');
    overlay.hidden = true;
    const card = el('div', 'dialog-card tabs-dialog-card');

    const header = el('div', 'dialog-header');
    header.appendChild(el('h2', 'dialog-title', 'Customize Tabs'));
    const closeBtn = el('button', 'dialog-close');
    closeBtn.title = 'Close dialog';
    closeBtn.setAttribute('aria-label', 'Close dialog');
    setIcon(closeBtn, 'stop');
    closeBtn.addEventListener('click', close);
    header.appendChild(closeBtn);
    card.appendChild(header);

    const hint = el('div', 'form-hint dim');
    hint.textContent = 'Hidden tabs stay hidden. Logs is always visible.';
    card.appendChild(hint);

    const list = el('div', 'tabs-custom-list');
    list.id = 'tabs-custom-list';
    card.appendChild(list);

    const buttonRow = el('div', 'dialog-buttons');
    const cancelBtn = el('button', 'btn btn-secondary', 'Cancel');
    cancelBtn.type = 'button';
    cancelBtn.addEventListener('click', close);
    const saveBtn = el('button', 'btn btn-primary', 'Save');
    saveBtn.type = 'button';
    buttonRow.appendChild(cancelBtn);
    buttonRow.appendChild(saveBtn);
    card.appendChild(buttonRow);

    overlay.appendChild(card);
    document.body.appendChild(overlay);

    // ---- List rendering ----
    function renderRows() {
        list.replaceChildren();
        order.forEach((id, idx) => {
            const meta = CUSTOMIZABLE.find((t) => t.id === id);
            if (!meta) return;
            const row = el('div', 'form-row');
            row.dataset.tabId = id;

            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.id = `tc-${id}`;
            checkbox.checked = !hiddenSet.has(id);
            checkbox.setAttribute('aria-label', `Show ${meta.label}`);
            checkbox.addEventListener('change', () => {
                if (checkbox.checked) hiddenSet.delete(id);
                else hiddenSet.add(id);
            });

            const label = el('label', 'form-label tabs-custom-label', meta.label);
            label.htmlFor = checkbox.id;
            label.prepend(checkbox);

            const up = el('button', 'btn tabs-custom-move', '↑');
            up.type = 'button';
            up.title = 'Move up';
            up.disabled = idx === 0;
            up.addEventListener('click', () => move(idx, -1));
            const down = el('button', 'btn tabs-custom-move', '↓');
            down.type = 'button';
            down.title = 'Move down';
            down.disabled = idx === order.length - 1;
            down.addEventListener('click', () => move(idx, 1));

            const btns = el('div', 'row-gap');
            btns.appendChild(up);
            btns.appendChild(down);

            row.appendChild(label);
            row.appendChild(btns);
            list.appendChild(row);
        });
    }

    // Swap en el array de orden y re-render (las flechas se re-habilitan solas).
    function move(idx, delta) {
        const j = idx + delta;
        if (j < 0 || j >= order.length) return;
        [order[idx], order[j]] = [order[j], order[idx]];
        renderRows();
    }

    // ---- Save ----
    async function save() {
        const hidden = order.filter((id) => hiddenSet.has(id));
        const tabs = { hidden, order: [...order] };
        const updated = { ...project, tabs };
        try {
            const errs = await saverFn(projectIndex, updated);
            if (errs && errs.length) {
                window.messageDialog.alert({ title: 'Could not save tabs', message: errs.join('\n'), trigger: saveBtn });
                return;
            }
        } catch (error) {
            window.messageDialog.alert({ title: 'Could not save tabs', message: error.message, trigger: saveBtn });
            return;
        }
        close();
        if (onSaved) onSaved(projectIndex);
    }

    // ---- Public API ----
    function open(index, proj) {
        projectIndex = index;
        project = proj;
        order = mergeOrder((proj && proj.tabs && proj.tabs.order) || []);
        hiddenSet = new Set((proj && proj.tabs && proj.tabs.hidden) || []);
        renderRows();
        overlay.hidden = false;
        isOpen = true;
    }

    function close() {
        overlay.hidden = true;
        isOpen = false;
        projectIndex = -1;
        project = null;
    }

    document.addEventListener('keydown', (e) => {
        if (!isOpen) return;
        if (e.key === 'Escape') close();
    });
    overlay.addEventListener('click', (e) => {
        if (e.target === overlay) close();
    });

    saveBtn.addEventListener('click', save);

    return { open, close, getElement: () => overlay, setSaver: (fn) => { saverFn = fn; } };
}