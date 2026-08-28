function node(tag, className, text) {
    const el = document.createElement(tag);
    if (className) el.className = className;
    if (text !== undefined) el.textContent = text;
    return el;
}

export function mountMessageDialog() {
    const overlay = node('div', 'message-overlay');
    overlay.hidden = true;
    const card = node('section', 'message-card');
    card.setAttribute('role', 'dialog'); card.setAttribute('aria-modal', 'true');
    const title = node('h2', 'message-title');
    const message = node('p', 'message-text');
    const actions = node('div', 'message-actions');
    const cancel = node('button', 'btn', 'Cancel'); cancel.dataset.messageCancel = '';
    const confirm = node('button', 'btn btn-accent', 'Confirm');
    actions.append(cancel, confirm); card.append(title, message, actions); overlay.append(card);
    let resolve, trigger, mode = 'confirm';
    const close = (value) => {
        overlay.hidden = true;
        const done = resolve; resolve = null;
        if (trigger?.focus) trigger.focus();
        if (done) done(value);
    };
    cancel.addEventListener('click', () => close(mode === 'confirm' ? false : undefined));
    confirm.addEventListener('click', () => close(mode === 'confirm' ? true : undefined));
    overlay.addEventListener('mousedown', (event) => { if (event.target === overlay) close(mode === 'confirm' ? false : undefined); });
    document.addEventListener('keydown', (event) => { if (!overlay.hidden && event.key === 'Escape') close(mode === 'confirm' ? false : undefined); });
    const open = (options, kind) => new Promise((done) => {
        mode = kind; resolve = done; trigger = options.trigger || document.activeElement;
        title.textContent = options.title; message.textContent = options.message;
        cancel.hidden = kind !== 'confirm'; confirm.textContent = kind === 'confirm' ? options.confirmLabel : 'Close';
        confirm.classList.toggle('btn-danger', !!options.destructive); confirm.classList.toggle('btn-accent', !options.destructive);
        overlay.hidden = false;
        (options.destructive && kind === 'confirm' ? cancel : confirm).focus();
    });
    return { confirm: (options) => open(options, 'confirm'), alert: (options) => open(options, 'alert'), getElement: () => overlay };
}
