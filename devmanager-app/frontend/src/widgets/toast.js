// Toasts apilables (Task 7): fixed bottom-right, m├íx 3, click-dismiss,
// auto-dismiss 4000ms con fade 220ms. Gate: settings.toasts_enabled.
const MAX_VISIBLE = 3;
const AUTO_DISMISS_MS = 4000;
const FADE_MS = 220;

const LEVEL_COLOR = {
    success: 'var(--ok)',
    info: 'var(--info)',
    warning: 'var(--warn)',
    error: 'var(--err)',
};

let container = null;
let enabled = true;

export function setToastsEnabled(value) {
    enabled = !!value;
}

function ensureContainer() {
    if (container) return container;
    container = document.createElement('div');
    container.id = 'toast-container';
    document.body.appendChild(container);
    return container;
}

function dismiss(el) {
    if (!el || el.dataset.closing) return;
    el.dataset.closing = '1';
    el.classList.add('closing');
    setTimeout(() => el.remove(), FADE_MS);
}

export function showToast(title, message = '', level = 'info') {
    if (!enabled) return null;
    const host = ensureContainer();

    // M├íx 3: el 4┬║ expulsa al m├ís viejo inmediatamente.
    while (host.children.length >= MAX_VISIBLE) {
        host.removeChild(host.firstElementChild);
    }

    const card = document.createElement('div');
    card.className = 'toast';
    card.style.borderLeftColor = LEVEL_COLOR[level] || LEVEL_COLOR.info;

    const t = document.createElement('div');
    t.className = 'toast-title';
    t.textContent = title || '';
    card.appendChild(t);

    if (message) {
        const m = document.createElement('div');
        m.className = 'toast-msg';
        m.textContent = message;
        card.appendChild(m);
    }

    card.addEventListener('click', () => dismiss(card));
    host.appendChild(card);

    setTimeout(() => dismiss(card), AUTO_DISMISS_MS);
    return card;
}
