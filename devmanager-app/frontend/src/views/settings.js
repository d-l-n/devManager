// Settings View - replaces the popup dialog with a full screen view
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

export function mountSettingsView() {
    const state = {
        theme: 'dark',
        style: 'standard',
        monitor_polling: true,
        toasts_enabled: true,
        oled_enabled: false,
    };

    // Theme radio buttons (excluding OLED)
    const themeRadios = ['light', 'dark', 'system'].map((value) => {
        const radio = document.createElement('input');
        radio.type = 'radio';
        radio.name = 'theme';
        radio.value = value;
        radio.id = `theme-${value}`;
        return radio;
    });

    // Style radio buttons
    const styleRadios = ['standard', 'brutalist'].map((value) => {
        const radio = document.createElement('input');
        radio.type = 'radio';
        radio.name = 'style';
        radio.value = value;
        radio.id = `style-${value}`;
        return radio;
    });

    // Checkboxes
    const monitorPollingCheckbox = el('input');
    monitorPollingCheckbox.type = 'checkbox';
    monitorPollingCheckbox.id = 'monitor-polling';

    const toastsEnabledCheckbox = el('input');
    toastsEnabledCheckbox.type = 'checkbox';
    toastsEnabledCheckbox.id = 'toasts-enabled';

    const oledEnabledCheckbox = el('input');
    oledEnabledCheckbox.type = 'checkbox';
    oledEnabledCheckbox.id = 'oled-enabled';

    function render() {
        const themeSection = $('settings-theme-section');
        themeSection.innerHTML = '';

        // Theme options
        const themeContainer = el('div', 'settings-options');
        themeRadios.forEach((radio, i) => {
            const value = radio.value;
            const label = optionRow(
                value.charAt(0).toUpperCase() + value.slice(1),
                value === 'system' ? 'Follow system theme preference' : `Use ${value} theme`,
                radio
            );
            themeContainer.appendChild(label);
        });
        themeSection.appendChild(themeContainer);

        // Style options
        const styleContainer = el('div', 'settings-options');
        styleRadios.forEach((radio) => {
            const value = radio.value;
            const label = optionRow(
                value.charAt(0).toUpperCase() + value.slice(1),
                value === 'brutalist' ? 'Bold, high-contrast design' : 'Clean, modern interface',
                radio
            );
            styleContainer.appendChild(label);
        });
        themeSection.appendChild(styleContainer);

        // OLED option
        const oledContainer = el('div', 'settings-options');
        const oledLabel = optionRow(
            'OLED Mode',
            'Pure black background for OLED displays (overrides theme selection)',
            oledEnabledCheckbox
        );
        oledContainer.appendChild(oledLabel);
        themeSection.appendChild(oledContainer);

        const behaviorSection = $('settings-behavior-section');
        behaviorSection.innerHTML = '';

        // Monitor polling
        const monitorContainer = el('div', 'settings-options');
        const monitorLabel = optionRow(
            'Monitor Polling',
            'Automatically monitor running servers and resource usage',
            monitorPollingCheckbox
        );
        monitorContainer.appendChild(monitorLabel);
        behaviorSection.appendChild(monitorContainer);

        // Toast notifications
        const toastContainer = el('div', 'settings-options');
        const toastLabel = optionRow(
            'Toast Notifications',
            'Show toast notifications for important events',
            toastsEnabledCheckbox
        );
        toastContainer.appendChild(toastLabel);
        behaviorSection.appendChild(toastContainer);

        syncUI();
    }

    function syncUI() {
        // Theme radios
        themeRadios.forEach(radio => {
            radio.checked = radio.value === state.theme;
        });

        // Style radios
        styleRadios.forEach(radio => {
            radio.checked = radio.value === state.style;
        });

        // Checkboxes
        monitorPollingCheckbox.checked = state.monitor_polling;
        toastsEnabledCheckbox.checked = state.toasts_enabled;
        oledEnabledCheckbox.checked = state.oled_enabled;

        // Disable theme radios when OLED is enabled
        themeRadios.forEach(radio => {
            radio.disabled = state.oled_enabled;
        });
    }

    // Event listeners
    themeRadios.forEach(radio => {
        radio.addEventListener('change', async () => {
            if (radio.checked) {
                state.theme = radio.value;
                await api.setSetting('theme', state.theme);
                if (!state.oled_enabled) {
                    applyTheme(state.theme);
                    if (window.updateThemeButtonIcon) {
                        window.updateThemeButtonIcon();
                    }
                }
            }
        });
    });

    styleRadios.forEach(radio => {
        radio.addEventListener('change', async () => {
            if (radio.checked) {
                state.style = radio.value;
                await api.setSetting('style', state.style);
                applyStyle(state.style);
            }
        });
    });

    monitorPollingCheckbox.addEventListener('change', async () => {
        state.monitor_polling = monitorPollingCheckbox.checked;
        await api.setSetting('monitor_polling', state.monitor_polling);
    });

    toastsEnabledCheckbox.addEventListener('change', async () => {
        state.toasts_enabled = toastsEnabledCheckbox.checked;
        await api.setSetting('toasts_enabled', state.toasts_enabled);
        setToastsEnabled(state.toasts_enabled);
    });

    oledEnabledCheckbox.addEventListener('change', async () => {
        state.oled_enabled = oledEnabledCheckbox.checked;
        await api.setSetting('oled_enabled', state.oled_enabled);
        
        // Apply theme based on OLED state
        if (state.oled_enabled) {
            applyTheme('oled');
        } else {
            applyTheme(state.theme);
        }
        
        // Update theme button icon
        if (window.updateThemeButtonIcon) {
            window.updateThemeButtonIcon();
        }
        
        syncUI(); // Update disabled state of theme radios
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
            // Only apply theme if OLED is not enabled
            if (!state.oled_enabled) {
                applyTheme(value, { persist: false });
                // Update theme button icon to reflect the new theme
                if (window.updateThemeButtonIcon) {
                    window.updateThemeButtonIcon();
                }
            }
        } else if (key === 'toasts_enabled') {
            state.toasts_enabled = normBool(value);
            setToastsEnabled(state.toasts_enabled);
        } else if (key === 'monitor_polling') {
            state.monitor_polling = normBool(value);
        } else if (key === 'oled_enabled') {
            state.oled_enabled = normBool(value);
            // Apply theme based on OLED state
            if (state.oled_enabled) {
                applyTheme('oled');
            } else {
                // When OLED is disabled, apply the current theme setting
                applyTheme(state.theme);
            }
            // Update theme button icon
            if (window.updateThemeButtonIcon) {
                window.updateThemeButtonIcon();
            }
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
                if (typeof s.oled_enabled === 'boolean') state.oled_enabled = s.oled_enabled;
            }
        } catch { /* defaults */ }
        
        // Apply theme based on OLED state
        if (state.oled_enabled) {
            applyTheme('oled', { persist: false });
        } else {
            applyTheme(state.theme, { persist: false });
        }
        
        applyStyle(state.style, { persist: false });
        setToastsEnabled(state.toasts_enabled);
        
        render();
    }

    function isVisible() {
        return !$('settings-view').hidden;
    }

    return { init, render, isVisible, getState: () => ({ ...state }) };
}