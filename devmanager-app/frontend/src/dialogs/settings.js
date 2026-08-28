// Dialog Settings persistente (Task 7). Patrón widgets: mount(ctx) → {open, close, init}.
// Cambios aplican en vivo vía setSetting; el eco settings:changed solo toca estado local.
import { api, events } from '../api.js';
import { applyTheme, applyStyle, isValidTheme, isValidStyle } from '../theme.js';
import { setToastsEnabled } from '../widgets/toast.js';

const normBool = (v) => v === true || v === 'true';

function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

function optionRow(labelText, description, input) {
    const label = el('label', 'settings-option');
    label.appendChild(input);
    const copy = el('span', 'settings-option-copy');
    copy.appendChild(el('span', 'settings-option-label', labelText));
    copy.appendChild(el('span', 'settings-option-description', description));
    label.appendChild(copy);
    return label;
}

export function mountSettings() {
    const state = {
        theme: 'dark',
        style: 'standard',
        monitor_polling: true,
        toasts_enabled: true,
    };
    let isOpen = false;

    // ---- DOM ----
    const overlay = el('div', 'settings-overlay');
    overlay.hidden = true;

    const card = el('div', 'settings-card');

    card.appendChild(el('div', 'settings-title', 'Settings'));

    const secAppearance = el('div', 'settings-section');
    secAppearance.appendChild(el('div', 'settings-section-title', 'Appearance'));
    const themeRadios = ['light', 'dark', 'oled', 'system'].map((value) => {
        const radio = document.createElement('input');
        radio.type = 'radio';
        radio.name = 'settings-theme';
        radio.value = value;
        const label = value === 'system' ? 'Follow system theme' : value[0].toUpperCase() + value.slice(1);
        const descriptions = {
            light: 'Use a bright interface at all times.',
            dark: 'Use the standard low-light interface.',
            oled: 'Use true black backgrounds to save OLED power.',
            system: 'Automatically match your operating system.',
        };
        const row = optionRow(label, descriptions[value], radio);
        secAppearance.appendChild(row);
        return radio;
    });
    card.appendChild(secAppearance);

    const secStyle = el('div', 'settings-section');
    secStyle.appendChild(el('div', 'settings-section-title', 'Interface style'));
    const styleRadios = ['standard', 'brutalist'].map((value) => {
        const radio = document.createElement('input');
        radio.type = 'radio'; radio.name = 'settings-style'; radio.value = value;
        secStyle.appendChild(optionRow(value === 'standard' ? 'Standard' : 'Brutalist', value === 'standard' ? 'Keep the current rounded interface.' : 'Use sharp edges and strong structural contrast.', radio));
        return radio;
    });
    card.appendChild(secStyle);

    const secNotifications = el('div', 'settings-section');
    secNotifications.appendChild(el('div', 'settings-section-title', 'Notifications'));
    const cbToasts = document.createElement('input');
    cbToasts.type = 'checkbox';
    cbToasts.id = 'settings-toasts';
    secNotifications.appendChild(optionRow('Enable toast notifications', 'Show brief updates for completed actions and errors.', cbToasts));
    card.appendChild(secNotifications);

    const secMonitor = el('div', 'settings-section');
    secMonitor.appendChild(el('div', 'settings-section-title', 'Monitor'));
    const cbPolling = document.createElement('input');
    cbPolling.type = 'checkbox';
    cbPolling.id = 'settings-polling';
    secMonitor.appendChild(optionRow('Enable resource polling', 'Refresh CPU and memory usage while the app is open.', cbPolling));
    card.appendChild(secMonitor);

    const footer = el('div', 'settings-footer');
    const btnClose = el('button', 'btn btn-accent', 'Close');
    footer.appendChild(btnClose);
    card.appendChild(footer);

    overlay.appendChild(card);
    document.body.appendChild(overlay);

    // ---- Comportamiento ----
    function syncUI() {
        themeRadios.forEach((r) => { r.checked = r.value === state.theme; });
        styleRadios.forEach((r) => { r.checked = r.value === state.style; });
        cbToasts.checked = state.toasts_enabled;
        cbPolling.checked = state.monitor_polling;
    }

    function open() {
        isOpen = true;
        syncUI();
        overlay.hidden = false;
    }

    function close() {
        isOpen = false;
        overlay.hidden = true;
    }

    themeRadios.forEach((radio) =>
        radio.addEventListener('change', () => {
            if (radio.checked) applyTheme(radio.value); // valida + aplica + persiste
        }));
    styleRadios.forEach((radio) => radio.addEventListener('change', () => { if (radio.checked) applyStyle(radio.value); }));
    cbToasts.addEventListener('change', () =>
        api.setSetting('toasts_enabled', String(cbToasts.checked)));
    cbPolling.addEventListener('change', () =>
        api.setSetting('monitor_polling', String(cbPolling.checked)));
    btnClose.addEventListener('click', close);
    overlay.addEventListener('mousedown', (e) => {
        if (e.target === overlay) close();
    });

    // Atajo propio de este task (los atajos completos llegan en Task 8).
    document.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === ',') {
            e.preventDefault();
            isOpen ? close() : open();
        } else if (e.key === 'Escape' && isOpen) {
            close();
        }
    });

    // Eco desde Go: actualizar estado local SIN re-persistir (evitar bucle).
    events().EventsOn('settings:changed', ({ key, value }) => {
        if (key === 'style') {
            if (!isValidStyle(value)) return;
            state.style = value;
            applyStyle(value, { persist: false });
        } else if (key === 'theme') {
            if (!isValidTheme(value)) return;
            state.theme = value;
            applyTheme(value, { persist: false });
        } else if (key === 'toasts_enabled') {
            state.toasts_enabled = normBool(value);
            setToastsEnabled(state.toasts_enabled);
        } else if (key === 'monitor_polling') {
            state.monitor_polling = normBool(value);
        } else {
            return;
        }
        syncUI();
    });

    async function init() {
        try {
            const s = await api.getSettings();
            if (s) {
                if (isValidTheme(s.theme)) state.theme = s.theme;
                if (isValidStyle(s.style)) state.style = s.style;
                if (typeof s.monitor_polling === 'boolean') state.monitor_polling = s.monitor_polling;
                if (typeof s.toasts_enabled === 'boolean') state.toasts_enabled = s.toasts_enabled;
            }
        } catch { /* defaults */ }
        applyTheme(state.theme, { persist: false });
        applyStyle(state.style, { persist: false });
        setToastsEnabled(state.toasts_enabled);
        syncUI();
    }

    return { open, close, init, isOpen: () => isOpen, getState: () => ({ ...state }) };
}
