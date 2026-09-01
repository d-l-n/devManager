// Settings View - replaces the popup dialog with a full screen view
import { api, events } from '../api.js';
import { applyTheme, applyStyle, setOledMode, isValidTheme, isValidStyle } from '../theme.js';
import { setToastsEnabled } from '../widgets/toast.js';
import { icon, hydrateIcons } from '../icons.js';

const $ = (id) => document.getElementById(id);
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

function divider() {
    const divider = el('div', 'settings-divider');
    return divider;
}

function sectionDescription(text) {
    const desc = el('div', 'settings-section-description', text);
    return desc;
}

function sectionTitleWithIcon(iconName, title) {
    const container = el('div', 'settings-section-title-with-icon');
    const iconEl = icon(iconName, { size: 18 });
    iconEl.setAttribute('class', 'settings-section-icon');
    const titleEl = el('span', 'settings-section-title-text', title);
    container.appendChild(iconEl);
    container.appendChild(titleEl);
    return container;
}

export function mountSettingsView() {
    const state = {
        theme: 'dark',
        style: 'standard',
        monitor_polling: true,
        toasts_enabled: true,
        oled_mode: false,
    };

    // Store references to elements
    let themeRadios = [];
    let styleRadios = [];
    let monitorPollingCheckbox = null;
    let toastsEnabledCheckbox = null;
    let oledModeCheckbox = null;

function render() {
    // Don't render if the view is not visible
    if (!isVisible()) {
        return;
    }
    
    // Create elements if they don't exist
    if (themeRadios.length === 0) {
        themeRadios = ['light', 'dark', 'system'].map((value) => {
            const radio = document.createElement('input');
            radio.type = 'radio';
            radio.name = 'theme';
            radio.value = value;
            radio.id = `theme-${value}`;
            return radio;
        });
    }
    
    if (styleRadios.length === 0) {
        styleRadios = ['standard', 'brutalist'].map((value) => {
            const radio = document.createElement('input');
            radio.type = 'radio';
            radio.name = 'style';
            radio.value = value;
            radio.id = `style-${value}`;
            return radio;
        });
    }
    
    if (!monitorPollingCheckbox) {
        monitorPollingCheckbox = el('input');
        monitorPollingCheckbox.type = 'checkbox';
        monitorPollingCheckbox.id = 'monitor-polling';
    }
    
    if (!toastsEnabledCheckbox) {
        toastsEnabledCheckbox = el('input');
        toastsEnabledCheckbox.type = 'checkbox';
        toastsEnabledCheckbox.id = 'toasts-enabled';
    }
    
    if (!oledModeCheckbox) {
        oledModeCheckbox = el('input');
        oledModeCheckbox.type = 'checkbox';
        oledModeCheckbox.id = 'oled-mode';
    }
    
    // Setup event listeners if not already done
    setupEventListeners();
    
    // Hydrate icons in the settings view
    hydrateIcons($('settings-view'));
    
    // Add icons to section titles - check if elements exist first
    const appearanceTitle = $('appearance-title');
    const displayTitle = $('display-title');
    const notificationsTitle = $('notifications-title');
    const monitoringTitle = $('monitoring-title');
        
    if (appearanceTitle) {
        appearanceTitle.replaceChildren(sectionTitleWithIcon('settings', 'Appearance'));
    }
    if (displayTitle) {
        displayTitle.replaceChildren(sectionTitleWithIcon('monitor', 'Display'));
    }
    if (notificationsTitle) {
        notificationsTitle.replaceChildren(sectionTitleWithIcon('notification', 'Notifications'));
    }
    if (monitoringTitle) {
        monitoringTitle.replaceChildren(sectionTitleWithIcon('code', 'Monitoring'));
    }

    // Theme Section
    const themeSection = $('settings-theme-section');
    if (!themeSection) {
        setTimeout(render, 100);
        return;
    }
    themeSection.innerHTML = '';
        
    // Theme section description
    themeSection.appendChild(sectionDescription('Choose your preferred color scheme for the application interface.'));

        // Theme options
        const themeContainer = el('div', 'settings-options');
        themeRadios.forEach((radio, i) => {
            const value = radio.value;
            let description;
            if (value === 'system') {
                description = 'Automatically switch between light and dark based on your system preferences';
            } else if (value === 'light') {
                description = 'Clean, bright interface ideal for daytime use';
            } else {
                description = 'Easy on the eyes, perfect for low-light environments';
            }
            const label = optionRow(
                value.charAt(0).toUpperCase() + value.slice(1),
                description,
                radio
            );
            themeContainer.appendChild(label);
        });
        themeSection.appendChild(themeContainer);

        // Add divider before style section
        themeSection.appendChild(divider());

        // Style Section
        const styleSection = $('settings-style-section');
        if (!styleSection) {
            console.warn('settings-style-section not found');
        } else {
            styleSection.innerHTML = '';
        
        // Style section description
        styleSection.appendChild(sectionDescription('Customize the visual style and appearance of interface elements.'));

        const styleContainer = el('div', 'settings-options');
        styleRadios.forEach((radio) => {
            const value = radio.value;
            const label = optionRow(
                value.charAt(0).toUpperCase() + value.slice(1),
                value === 'brutalist' ? 'Bold, high-contrast design with strong visual elements' : 'Clean, modern interface with smooth transitions',
                radio
            );
            styleContainer.appendChild(label);
        });
        styleSection.appendChild(styleContainer);
        }

        // Display Section
        const displaySection = $('settings-display-section');
        if (!displaySection) {
            console.warn('settings-display-section not found');
        } else {
            displaySection.innerHTML = '';
        
        // Display section description
        displaySection.appendChild(sectionDescription('Optimize the display for your specific hardware and viewing preferences.'));

        // OLED mode
        const oledContainer = el('div', 'settings-options');
        const oledLabel = optionRow(
            'OLED Mode',
            'Use pure black backgrounds to save power and improve contrast on OLED displays',
            oledModeCheckbox
        );
        oledContainer.appendChild(oledLabel);
        displaySection.appendChild(oledContainer);
        }

        // Notifications Section
        const notificationsSection = $('settings-notifications-section');
        if (!notificationsSection) {
            console.warn('settings-notifications-section not found');
        } else {
            notificationsSection.innerHTML = '';
        
        // Notifications section description
        notificationsSection.appendChild(sectionDescription('Control how and when the application notifies you about important events.'));

        // Toast notifications
        const toastContainer = el('div', 'settings-options');
        const toastLabel = optionRow(
            'Toast Notifications',
            'Show non-intrusive notifications for important events and actions',
            toastsEnabledCheckbox
        );
        toastContainer.appendChild(toastLabel);
        notificationsSection.appendChild(toastContainer);
        }

        // Monitoring Section
        const monitoringSection = $('settings-monitoring-section');
        if (!monitoringSection) {
            console.warn('settings-monitoring-section not found');
        } else {
            monitoringSection.innerHTML = '';
        
        // Monitoring section description
        monitoringSection.appendChild(sectionDescription('Configure automatic monitoring of your development servers and resource usage.'));

        // Monitor polling
        const monitorContainer = el('div', 'settings-options');
        const monitorLabel = optionRow(
            'Monitor Polling',
            'Track server status and resource usage in real-time for all active projects',
            monitorPollingCheckbox
        );
        monitorContainer.appendChild(monitorLabel);
        monitoringSection.appendChild(monitorContainer);
        }

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
        oledModeCheckbox.checked = state.oled_mode;
}

    function setupEventListeners() {
            // Event listeners - only add if not already added
            themeRadios.forEach(radio => {
                if (!radio.hasAttribute('data-listener-added')) {
                    radio.addEventListener('change', async () => {
                        if (radio.checked) {
                            state.theme = radio.value;
                            await api.setSetting('theme', state.theme);
                            applyTheme(state.theme);
                            if (window.updateThemeButtonIcon) {
                                window.updateThemeButtonIcon();
                            }
                        }
                    });
                    radio.setAttribute('data-listener-added', 'true');
                }
            });

            styleRadios.forEach(radio => {
                if (!radio.hasAttribute('data-listener-added')) {
                    radio.addEventListener('change', async () => {
                        if (radio.checked) {
                            state.style = radio.value;
                            await api.setSetting('style', state.style);
                            applyStyle(state.style);
                        }
                    });
                    radio.setAttribute('data-listener-added', 'true');
                }
            });

            if (monitorPollingCheckbox && !monitorPollingCheckbox.hasAttribute('data-listener-added')) {
                monitorPollingCheckbox.addEventListener('change', async () => {
                    state.monitor_polling = monitorPollingCheckbox.checked;
                    await api.setSetting('monitor_polling', state.monitor_polling);
                });
                monitorPollingCheckbox.setAttribute('data-listener-added', 'true');
            }

            if (toastsEnabledCheckbox && !toastsEnabledCheckbox.hasAttribute('data-listener-added')) {
                toastsEnabledCheckbox.addEventListener('change', async () => {
                    state.toasts_enabled = toastsEnabledCheckbox.checked;
                    await api.setSetting('toasts_enabled', state.toasts_enabled);
                    setToastsEnabled(state.toasts_enabled);
                });
                toastsEnabledCheckbox.setAttribute('data-listener-added', 'true');
            }

            if (oledModeCheckbox && !oledModeCheckbox.hasAttribute('data-listener-added')) {
                oledModeCheckbox.addEventListener('change', async () => {
                    state.oled_mode = oledModeCheckbox.checked;
                    setOledMode(state.oled_mode);
                    // Update theme button icon to reflect the new actual theme
                    if (window.updateThemeButtonIcon) {
                        window.updateThemeButtonIcon();
                    }
                });
                oledModeCheckbox.setAttribute('data-listener-added', 'true');
            }
        }

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
            // Update theme button icon to reflect the new theme
            if (window.updateThemeButtonIcon) {
                window.updateThemeButtonIcon();
            }
        } else if (key === 'toasts_enabled') {
            state.toasts_enabled = normBool(value);
            setToastsEnabled(state.toasts_enabled);
} else if (key === 'monitor_polling') {
            state.monitor_polling = normBool(value);
        } else if (key === 'oled_mode') {
            state.oled_mode = normBool(value);
            setOledMode(state.oled_mode, { persist: false });
            // Update theme button icon to reflect the new actual theme
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
                if (typeof s.oled_mode === 'boolean') state.oled_mode = s.oled_mode;
            }
        } catch { /* defaults */ }
        
        applyTheme(state.theme, { persist: false });
        applyStyle(state.style, { persist: false });
        setToastsEnabled(state.toasts_enabled);
        setOledMode(state.oled_mode, { persist: false });
        
        // Don't render here - let switchView handle rendering when the view is actually shown
        // This prevents rendering issues when the view is hidden
    }

    function isVisible() {
        return !$('settings-view').hidden;
    }

    // Re-render when the view becomes visible
    const observer = new MutationObserver((mutations) => {
        mutations.forEach((mutation) => {
            if (mutation.type === 'attributes' && mutation.attributeName === 'hidden') {
                if (!$('settings-view').hidden) {
                    console.log('Settings view became visible, rendering...');
                    setTimeout(render, 50); // Small delay to ensure DOM is ready
                }
            }
        });
    });
    
    observer.observe($('settings-view'), { attributes: true });

    return { init, render, isVisible, getState: () => ({ ...state }) };
}