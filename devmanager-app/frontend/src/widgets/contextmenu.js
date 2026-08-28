// Menu contextual de proyecto (Issue #12): singleton posicionado en el cursor.
// mount() ÔåÆ { show(items, x, y), hide() }.
function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

let menuEl = null;

export function mountContextMenu() {
    if (menuEl) return menu(menuEl);

    const root = el('div', 'context-menu');
    root.hidden = true;
    document.body.appendChild(root);
    menuEl = root;

    document.addEventListener('mousedown', (e) => {
        if (root.hidden || root.contains(e.target)) return;
        root.hidden = true;
    });
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') root.hidden = true;
    });

    function show(items, x, y) {
        root.innerHTML = '';
        items.forEach((it) => {
            if (it.separator) {
                root.appendChild(el('div', 'context-menu-sep'));
                return;
            }
            const btn = el('button', 'context-menu-item' + (it.danger ? ' danger' : ''), it.label);
            if (it.icon) btn.textContent = it.icon + '  ' + it.label;
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                root.hidden = true;
                if (it.onClick) it.onClick();
            });
            root.appendChild(btn);
        });
        root.hidden = false;
        // Clamp dentro de la ventana
        const r = root.getBoundingClientRect();
        const vw = window.innerWidth, vh = window.innerHeight;
        const px = Math.min(x, Math.max(0, vw - r.width - 8));
        const py = Math.min(y, Math.max(0, vh - r.height - 8));
        root.style.left = `${px}px`;
        root.style.top = `${py}px`;
    }

    return {
        show,
        hide: () => { root.hidden = true; },
    };
}
