// Settings View - replaces the popup dialog with a full screen view
import { api, events, formatVersion } from '../api.js';
import {
    applyTheme, applyStyle, setOledMode, isValidTheme, isValidStyle,
    setAccentOverrides, getAccentOverrides, getEffectiveAccent,
    isValidHexColor, STYLE_DEFAULT_ACCENT,
} from '../theme.js';
import { setToastsEnabled, showToast } from '../widgets/toast.js';
import { icon, hydrateIcons } from '../icons.js';
import { mountExternalNotifications } from '../widgets/external-notifications.js';
import { LANGUAGES, setLanguage, applyTranslations, t } from '../i18n.js';
import { setTouchMode } from '../touch.js';

const $ = (id) => document.getElementById(id);
const normBool = (v) => v === true || v === 'true';

function escapeHtml(s) {
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

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

export function mountSettingsView(ctx = {}) {
    const state = {
        theme: 'dark',
        style: 'standard',
        monitor_polling: true,
        toasts_enabled: true,
        oled_mode: false,
        accent_overrides: {},
        accent_global: false,
        accent_global_color: '',
        external_notifications: true,
        language: 'en',
        touch_optimized: false,
        backup_frequency: 'off',
        backup_retention: 20,
    };

    // Store references to elements
    let themeRadios = [];
    let monitorPollingCheckbox = null;
    let toastsEnabledCheckbox = null;
    let oledModeCheckbox = null;
    let accentColorInput = null;
    let accentResetBtn = null;
    let externalNotificationsCheckbox = null;
    let notifyPanel = null;
    let languageSelect = null;
    let touchCheckbox = null;
    let backupFrequencySelect = null;
    let backupRetentionInput = null;
    let backupListContainer = null;

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

    if (!externalNotificationsCheckbox) {
        externalNotificationsCheckbox = el('input');
        externalNotificationsCheckbox.type = 'checkbox';
        externalNotificationsCheckbox.id = 'external-notifications';
    }

    if (!languageSelect) {
        languageSelect = el('select', 'text-input');
        languageSelect.id = 'language-select';
        LANGUAGES.forEach((l) => {
            const o = el('option', '', l.label);
            o.value = l.code;
            languageSelect.appendChild(o);
        });
    }
    if (!touchCheckbox) {
        touchCheckbox = el('input');
        touchCheckbox.type = 'checkbox';
        touchCheckbox.id = 'touch-optimized';
    }
    if (!backupFrequencySelect) {
        backupFrequencySelect = el('select', 'text-input form-select');
        backupFrequencySelect.id = 'backup-frequency-select';
        const freqs = [
            { value: 'off', label: 'Disabled (Manual only)' },
            { value: 'hourly', label: 'Hourly' },
            { value: '6h', label: 'Every 6 hours' },
            { value: 'daily', label: 'Daily' },
            { value: 'weekly', label: 'Weekly' },
        ];
        freqs.forEach((f) => {
            const o = el('option', '', f.label);
            o.value = f.value;
            backupFrequencySelect.appendChild(o);
        });
    }
    if (!backupRetentionInput) {
        backupRetentionInput = el('input', 'text-input');
        backupRetentionInput.type = 'number';
        backupRetentionInput.min = '1';
        backupRetentionInput.max = '500';
        backupRetentionInput.id = 'backup-retention-input';
        backupRetentionInput.style.maxWidth = '100px';
    }
    
    // Setup event listeners if not already done
    setupEventListeners();
    
    // Hydrate icons in the settings view
    hydrateIcons($('settings-view'));
    
    // Add icons to section titles - check if elements exist first
    const appearanceTitle = $('appearance-title');
    const styleTitle = $('style-title');
    const displayTitle = $('display-title');
    const notificationsTitle = $('notifications-title');
    const monitoringTitle = $('monitoring-title');
    const accentTitle = $('accent-title');
    const backupTitle = $('backup-title');
    const aboutTitle = $('about-title');
        
    if (appearanceTitle) {
        appearanceTitle.replaceChildren(sectionTitleWithIcon('settings', 'Appearance'));
    }
    if (styleTitle) {
        styleTitle.replaceChildren(sectionTitleWithIcon('palette', 'Visual Style'));
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
    if (accentTitle) {
        accentTitle.replaceChildren(sectionTitleWithIcon('palette', 'Accent Color'));
    }
    if (backupTitle) {
        backupTitle.replaceChildren(sectionTitleWithIcon('folder', 'Backup & Restore'));
    }
    if (aboutTitle) {
        aboutTitle.replaceChildren(sectionTitleWithIcon('settings', 'About'));
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

        // Display Section (i18n + touch, Issue #69)
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

        // Idioma (Issue #69)
        const langContainer = el('div', 'settings-options');
        const langLabel = el('label', 'settings-option');
        langLabel.appendChild(languageSelect);
        const langCopy = el('span', 'settings-option-copy');
        langCopy.appendChild(el('span', 'settings-option-label', 'Language'));
        langCopy.appendChild(el('span', 'settings-option-description', 'Interface language with right-to-left support for Arabic')); 
        langLabel.appendChild(langCopy);
        langContainer.appendChild(langLabel);
        displaySection.appendChild(langContainer);

        // Modo táctil (Issue #69)
        const touchContainer = el('div', 'settings-options');
        const touchLabel = optionRow(
            'Touch Mode',
            'Larger 44px touch targets and swipe navigation between tabs for tablets and touch devices',
            touchCheckbox
        );
        touchContainer.appendChild(touchLabel);
        displaySection.appendChild(touchContainer);
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

        // Notificaciones externas (Issue #66): gate + plataformas/reglas/historial.
        const extContainer = el('div', 'settings-options');
        const extLabel = optionRow(
            'External Notifications',
            'Send events to Slack, Discord, Telegram or Microsoft Teams with routing rules and rate limiting',
            externalNotificationsCheckbox
        );
        extContainer.appendChild(extLabel);
        notificationsSection.appendChild(extContainer);

        const notifyAnchor = el('div');
        notificationsSection.appendChild(notifyAnchor);
        notifyPanel = mountExternalNotifications(notifyAnchor);
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

        // Accent Color Section
        const accentSection = $('settings-accent-section');
        if (accentSection) {
            accentSection.innerHTML = '';
            accentSection.appendChild(sectionDescription('Customize the accent color used across buttons, links, and highlights.'));

            // Create accent color inputs if not already done
            if (!accentColorInput) {
                accentColorInput = el('input', 'color-picker-input');
                accentColorInput.type = 'color';
                accentColorInput.id = 'accent-color-picker';
            }
            if (!accentResetBtn) {
                accentResetBtn = el('button', 'btn btn-small');
                accentResetBtn.textContent = 'Reset to default';
            }

            const accentRow = el('div', 'settings-options');
            const currentDefault = '#6366f1';
            accentColorInput.value = state.accent_overrides['standard'] || currentDefault;
            const accentLabel = el('label', 'settings-option');
            accentLabel.appendChild(accentColorInput);
            const accentCopy = el('span', 'settings-option-copy');
            accentCopy.appendChild(el('span', 'settings-option-label', 'Accent color'));
            accentCopy.appendChild(el('span', 'settings-option-description', 'Choose the primary accent color used across the interface'));
            accentLabel.appendChild(accentCopy);
            accentRow.appendChild(accentLabel);
            accentRow.appendChild(accentResetBtn);
            accentSection.appendChild(accentRow);
        }

        // Backup & Restore Section
        const backupSection = $('settings-backup-section');
        if (backupSection) {
            renderBackupSection(backupSection);
        }

        syncUI();
    }

    function renderBackupSection(container) {
        container.innerHTML = '';
        container.appendChild(sectionDescription('Create snapshots of projects and settings, manage automated backups, or restore previous versions.'));

        // Action buttons toolbar
        const btnRow = el('div', 'row-gap');
        btnRow.style.marginBottom = '14px';

        const backupNowBtn = el('button', 'btn btn-primary', 'Create Backup Now');
        backupNowBtn.addEventListener('click', async () => {
            backupNowBtn.disabled = true;
            backupNowBtn.textContent = 'Creating Backup...';
            try {
                const res = await api.createBackup();
                if (res && res.ok) {
                    showToast('Backup Created', res.message || `Created ${res.filename}`, 'success');
                    if (backupListContainer) refreshBackupList(backupListContainer);
                } else {
                    showToast('Backup Failed', (res && res.message) || 'Failed to create backup', 'error');
                }
            } catch (err) {
                showToast('Backup Error', String(err), 'error');
            } finally {
                backupNowBtn.disabled = false;
                backupNowBtn.textContent = 'Create Backup Now';
            }
        });

        const openFolderBtn = el('button', 'btn btn-accent', 'Open Backups Folder');
        openFolderBtn.addEventListener('click', async () => {
            try {
                await api.openBackupsFolder();
            } catch (err) {
                showToast('Error', 'Could not open backups folder: ' + err, 'error');
            }
        });

        const refreshListBtn = el('button', 'btn', 'Refresh List');
        refreshListBtn.addEventListener('click', () => {
            if (backupListContainer) refreshBackupList(backupListContainer);
        });

        btnRow.appendChild(backupNowBtn);
        btnRow.appendChild(openFolderBtn);
        btnRow.appendChild(refreshListBtn);
        container.appendChild(btnRow);

        // Schedule & Retention options
        const schedContainer = el('div', 'settings-options');

        const freqLabel = el('label', 'settings-option');
        freqLabel.appendChild(backupFrequencySelect);
        const freqCopy = el('span', 'settings-option-copy');
        freqCopy.appendChild(el('span', 'settings-option-label', 'Automatic Backup Frequency'));
        freqCopy.appendChild(el('span', 'settings-option-description', 'Schedule automatic snapshots of configuration files'));
        freqLabel.appendChild(freqCopy);
        schedContainer.appendChild(freqLabel);

        const retLabel = el('label', 'settings-option');
        retLabel.appendChild(backupRetentionInput);
        const retCopy = el('span', 'settings-option-copy');
        retCopy.appendChild(el('span', 'settings-option-label', 'Backup Retention Limit'));
        retCopy.appendChild(el('span', 'settings-option-description', 'Maximum number of backup snapshots to keep (1-500)'));
        retLabel.appendChild(retCopy);
        schedContainer.appendChild(retLabel);

        container.appendChild(schedContainer);

        // Subtitle for backups list
        const listHeader = el('div', 'row-between');
        listHeader.style.marginTop = '16px';
        listHeader.style.marginBottom = '8px';
        const listTitle = el('span', 'bold', 'Existing Backups');
        listHeader.appendChild(listTitle);
        container.appendChild(listHeader);

        // List container
        backupListContainer = el('div', 'backups-list');
        container.appendChild(backupListContainer);
        refreshBackupList(backupListContainer);
    }

    async function refreshBackupList(listContainer) {
        listContainer.innerHTML = '';
        const loading = el('div', 'dim center padded', 'Loading backups...');
        listContainer.appendChild(loading);
        try {
            const backups = await api.listBackups();
            listContainer.innerHTML = '';
            if (!backups || backups.length === 0) {
                listContainer.appendChild(el('div', 'dim center padded', 'No backups found. Click "Create Backup Now" to create your first snapshot.'));
                return;
            }
            backups.forEach((b) => {
                const row = el('div', 'backup-item-row');
                row.style.display = 'flex';
                row.style.alignItems = 'center';
                row.style.justifyContent = 'space-between';
                row.style.padding = '10px 14px';
                row.style.margin = '6px 0';
                row.style.background = 'var(--bg-card, var(--bg-panel))';
                row.style.borderRadius = 'var(--radius, 6px)';
                row.style.border = '1px solid var(--border)';

                const info = el('div', 'backup-item-info');
                const nameEl = el('div', 'mono bold', b.filename);
                nameEl.style.fontSize = '13px';
                const createdDate = b.createdAt ? new Date(b.createdAt).toLocaleString() : 'Unknown date';
                const sizeKb = b.sizeBytes ? (b.sizeBytes / 1024).toFixed(1) + ' KB' : '0 KB';
                const metaEl = el('div', 'dim', `${createdDate} · ${sizeKb} · ${b.projectsCount || 0} projects · v${b.appVersion || '2.x'}`);
                metaEl.style.fontSize = '12px';
                metaEl.style.marginTop = '2px';
                info.appendChild(nameEl);
                info.appendChild(metaEl);

                const actions = el('div', 'backup-item-actions');
                actions.style.display = 'flex';
                actions.style.alignItems = 'center';
                actions.style.gap = '8px';

                const statusBadge = el('span', `badge ${b.valid ? 'running' : 'stopped'}`, b.valid ? 'Valid' : 'Corrupted');
                if (b.error) statusBadge.title = b.error;
                actions.appendChild(statusBadge);

                const validateBtn = el('button', 'btn btn-small', 'Validate');
                validateBtn.addEventListener('click', async () => {
                    validateBtn.disabled = true;
                    try {
                        const res = await api.validateBackup(b.filename);
                        showToast('Backup Validation', (res && res.message) || (res && res.ok ? 'Backup is valid' : 'Backup invalid'), (res && res.ok) ? 'success' : 'error');
                    } catch (err) {
                        showToast('Validation Error', String(err), 'error');
                    } finally {
                        validateBtn.disabled = false;
                    }
                });
                actions.appendChild(validateBtn);

                const restoreBtn = el('button', 'btn btn-small btn-warn', 'Restore');
                restoreBtn.addEventListener('click', async () => {
                    let confirmed = false;
                    if (ctx && ctx.messageDialog && ctx.messageDialog.confirm) {
                        confirmed = await ctx.messageDialog.confirm({
                            title: 'Restore Backup',
                            message: `Restoring "${b.filename}" will overwrite current projects and settings. A safety snapshot (.pre-restore) will be saved first. Continue?`,
                            confirmLabel: 'Restore',
                            destructive: true,
                        });
                    } else {
                        confirmed = window.confirm(`Restore "${b.filename}"? Current configuration will be replaced.`);
                    }
                    if (!confirmed) return;

                    restoreBtn.disabled = true;
                    try {
                        const errs = await api.restoreBackup(b.filename);
                        if (errs && errs.length > 0) {
                            showToast('Restore Failed', errs.join('\n'), 'error');
                        } else {
                            showToast('Restore Successful', `Restored configuration from ${b.filename}`, 'success');
                            refreshBackupList(listContainer);
                        }
                    } catch (err) {
                        showToast('Restore Error', String(err), 'error');
                    } finally {
                        restoreBtn.disabled = false;
                    }
                });
                actions.appendChild(restoreBtn);

                row.appendChild(info);
                row.appendChild(actions);
                listContainer.appendChild(row);
            });
        } catch (err) {
            listContainer.innerHTML = '';
            listContainer.appendChild(el('div', 'result-strip err', 'Failed to load backups: ' + err));
        }
    }

    function syncUI() {
        // Theme radios
        themeRadios.forEach(radio => {
            radio.checked = radio.value === state.theme;
        });

        // Checkboxes
        monitorPollingCheckbox.checked = state.monitor_polling;
        toastsEnabledCheckbox.checked = state.toasts_enabled;
        oledModeCheckbox.checked = state.oled_mode;
        if (externalNotificationsCheckbox) externalNotificationsCheckbox.checked = state.external_notifications;
        if (languageSelect) languageSelect.value = state.language;
        if (touchCheckbox) touchCheckbox.checked = state.touch_optimized;
        if (backupFrequencySelect) backupFrequencySelect.value = state.backup_frequency;
        if (backupRetentionInput) backupRetentionInput.value = state.backup_retention;

        // Accent state
        if (accentColorInput) {
            accentColorInput.value = state.accent_overrides['standard'] || '#6366f1';
        }
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

            if (monitorPollingCheckbox && !monitorPollingCheckbox.hasAttribute('data-listener-added')) {
                monitorPollingCheckbox.addEventListener('change', async () => {
                    state.monitor_polling = monitorPollingCheckbox.checked;
                    await api.setSetting('monitor_polling', String(state.monitor_polling));
                });
                monitorPollingCheckbox.setAttribute('data-listener-added', 'true');
            }

            if (toastsEnabledCheckbox && !toastsEnabledCheckbox.hasAttribute('data-listener-added')) {
                toastsEnabledCheckbox.addEventListener('change', async () => {
                    state.toasts_enabled = toastsEnabledCheckbox.checked;
                    await api.setSetting('toasts_enabled', String(state.toasts_enabled));
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

            if (externalNotificationsCheckbox && !externalNotificationsCheckbox.hasAttribute('data-listener-added')) {
                externalNotificationsCheckbox.addEventListener('change', async () => {
                    state.external_notifications = externalNotificationsCheckbox.checked;
                    await api.setSetting('external_notifications', String(state.external_notifications));
                });
                externalNotificationsCheckbox.setAttribute('data-listener-added', 'true');
            }

            if (languageSelect && !languageSelect.hasAttribute('data-listener-added')) {
                languageSelect.addEventListener('change', async () => {
                    state.language = languageSelect.value;
                    setLanguage(state.language);
                    await api.setSetting('language', state.language);
                    applyTranslations(document);
                });
                languageSelect.setAttribute('data-listener-added', 'true');
            }

            if (touchCheckbox && !touchCheckbox.hasAttribute('data-listener-added')) {
                touchCheckbox.addEventListener('change', async () => {
                    state.touch_optimized = touchCheckbox.checked;
                    setTouchMode(state.touch_optimized);
                    await api.setSetting('touch_optimized', String(state.touch_optimized));
                });
                touchCheckbox.setAttribute('data-listener-added', 'true');
            }

            // Accent color picker
            if (accentColorInput && !accentColorInput.hasAttribute('data-listener-added')) {
                accentColorInput.addEventListener('input', async () => {
                    const val = accentColorInput.value;
                    if (isValidHexColor(val)) {
                        state.accent_overrides['standard'] = val;
                        await api.setSetting('accent_override.standard', val);
                        setAccentOverrides(state.accent_overrides);
                    }
                });
                accentColorInput.setAttribute('data-listener-added', 'true');
            }

            // Accent reset button
            if (accentResetBtn && !accentResetBtn.hasAttribute('data-listener-added')) {
                accentResetBtn.addEventListener('click', async () => {
                    delete state.accent_overrides['standard'];
                    await api.setSetting('accent_override.standard', 'default');
                    setAccentOverrides(state.accent_overrides);
                    syncUI();
                });
                accentResetBtn.setAttribute('data-listener-added', 'true');
            }

            // Backup settings listeners
            if (backupFrequencySelect && !backupFrequencySelect.hasAttribute('data-listener-added')) {
                backupFrequencySelect.addEventListener('change', async () => {
                    state.backup_frequency = backupFrequencySelect.value;
                    await api.setSetting('backup_frequency', state.backup_frequency);
                });
                backupFrequencySelect.setAttribute('data-listener-added', 'true');
            }

            if (backupRetentionInput && !backupRetentionInput.hasAttribute('data-listener-added')) {
                backupRetentionInput.addEventListener('change', async () => {
                    const val = parseInt(backupRetentionInput.value, 10);
                    if (!isNaN(val) && val >= 1 && val <= 500) {
                        state.backup_retention = val;
                        await api.setSetting('backup_retention', String(state.backup_retention));
                    }
                });
                backupRetentionInput.setAttribute('data-listener-added', 'true');
            }
        }

    // Eco desde Go: actualizar estado local SIN re-persistir (evitar bucle).
    events().EventsOn('settings:changed', ({ key, value }) => {
        // Track whether a visual rebuild is needed (theme/style/accent changes
        // affect labels, previews and section content that syncUI alone can't update).
        let needsRender = false;
        if (key === 'style') {
            state.style = 'standard';
            applyStyle('standard', { persist: false });
            needsRender = true;
        } else if (key === 'theme') {
            if (!isValidTheme(value)) return;
            state.theme = value;
            applyTheme(value, { persist: false });
            // Update theme button icon to reflect the new theme
            if (window.updateThemeButtonIcon) {
                window.updateThemeButtonIcon();
            }
            needsRender = true;
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
            needsRender = true;
        } else if (key === 'backup_frequency') {
            state.backup_frequency = value;
            if (backupFrequencySelect) backupFrequencySelect.value = value;
        } else if (key === 'backup_retention') {
            state.backup_retention = parseInt(value, 10) || 20;
            if (backupRetentionInput) backupRetentionInput.value = state.backup_retention;
        } else if (key && key.startsWith('accent_override.')) {
            const style = key.slice('accent_override.'.length);
            if (value && value !== '' && value !== 'default') {
                state.accent_overrides[style] = value;
            } else {
                delete state.accent_overrides[style];
            }
            setAccentOverrides(state.accent_overrides);
            needsRender = true;
        } else {
            return;
        }
        // Re-render when visual properties change so labels, previews and accent
        // section content stay in sync; plain syncUI for non-visual toggles.
        if (needsRender && isVisible()) {
            render();
        } else {
            syncUI();
        }
    });

    // Backup restore reload event
    events().EventsOn('settings:reloaded', async () => {
        try {
            const s = await api.getSettings();
            if (s) {
                if (isValidTheme(s.theme)) state.theme = s.theme;
                state.style = 'standard';
                if (typeof s.monitor_polling === 'boolean') state.monitor_polling = s.monitor_polling;
                if (typeof s.toasts_enabled === 'boolean') state.toasts_enabled = s.toasts_enabled;
                if (typeof s.oled_mode === 'boolean') state.oled_mode = s.oled_mode;
                if (s.accent_overrides && typeof s.accent_overrides === 'object') state.accent_overrides = s.accent_overrides;
                if (typeof s.external_notifications === 'boolean') state.external_notifications = s.external_notifications;
                if (typeof s.language === 'string') state.language = s.language;
                if (typeof s.touch_optimized === 'boolean') state.touch_optimized = s.touch_optimized;
                if (typeof s.backup_frequency === 'string') state.backup_frequency = s.backup_frequency;
                if (typeof s.backup_retention === 'number') state.backup_retention = s.backup_retention;
            }
        } catch { /* ignore */ }
        applyTheme(state.theme, { persist: false });
        applyStyle(state.style, { persist: false });
        setToastsEnabled(state.toasts_enabled);
        setOledMode(state.oled_mode, { persist: false });
        setAccentOverrides(state.accent_overrides);
        if (isVisible()) {
            render();
        } else {
            syncUI();
        }
    });

async function init() {
        try {
            const s = await api.getSettings();
            if (s) {
                if (isValidTheme(s.theme)) state.theme = s.theme;
                state.style = 'standard';
                if (typeof s.monitor_polling === 'boolean') state.monitor_polling = s.monitor_polling;
                if (typeof s.toasts_enabled === 'boolean') state.toasts_enabled = s.toasts_enabled;
                if (typeof s.oled_mode === 'boolean') state.oled_mode = s.oled_mode;
                if (s.accent_overrides && typeof s.accent_overrides === 'object') state.accent_overrides = s.accent_overrides;
                if (typeof s.external_notifications === 'boolean') state.external_notifications = s.external_notifications;
                if (typeof s.language === 'string') state.language = s.language;
                if (typeof s.touch_optimized === 'boolean') state.touch_optimized = s.touch_optimized;
                if (typeof s.backup_frequency === 'string') state.backup_frequency = s.backup_frequency;
                if (typeof s.backup_retention === 'number') state.backup_retention = s.backup_retention;
            }
        } catch (error) {
            console.warn('Failed to load settings, using defaults', error);
            showToast('Settings', 'Could not load settings, using defaults', 'warning');
        }
        
        applyTheme(state.theme, { persist: false });
        applyStyle(state.style, { persist: false });
        setToastsEnabled(state.toasts_enabled);
        setOledMode(state.oled_mode, { persist: false });
        setAccentOverrides(state.accent_overrides);
        
        // Version display and update check
        try {
            const ver = await api.getVersion();
            const versionEl = $('app-version');
            if (versionEl) versionEl.textContent = formatVersion(ver);
        } catch { /* ignore */ }
        const updateBtn = $('btn-check-update');
        if (updateBtn && !updateBtn.hasAttribute('data-listener-added')) {
            updateBtn.addEventListener('click', async () => {
                updateBtn.disabled = true;
                updateBtn.textContent = 'Checking...';
                try {
                    const info = await api.checkForUpdate();
                    const resultEl = $('update-result');
                    if (resultEl) {
                        resultEl.hidden = false;
                        if (info.error) {
                            resultEl.className = 'result-strip err';
                            resultEl.textContent = 'Error: ' + info.error;
                        } else if (info.isUpToDate) {
                            resultEl.className = 'result-strip ok';
                            resultEl.textContent = 'You are running the latest version (' + info.currentVersion + ')';
                        } else {
                            resultEl.className = 'result-strip info';
                            resultEl.innerHTML = 'Update available: <strong>' + info.latestVersion + '</strong> (current: ' + info.currentVersion + ')' +
                                (info.downloadURL ? '<br><a href="#" id="update-download-link" style="color:var(--accent);text-decoration:underline;cursor:pointer;">Download</a>' : '') +
                                (info.releaseNotes ? '<br><details style="margin-top:6px"><summary style="cursor:pointer;color:var(--fg-dim)">Release notes</summary><div style="margin-top:4px;white-space:pre-wrap;font-size:12px;max-height:200px;overflow-y:auto;">' + escapeHtml(info.releaseNotes) + '</div></details>' : '');
                            if (info.downloadURL) {
                                setTimeout(() => {
                                    const link = $('update-download-link');
                                    if (link) link.addEventListener('click', (e) => { e.preventDefault(); api.openURL(info.downloadURL); });
                                }, 0);
                            }
                        }
                    }
                } catch (err) {
                    const resultEl = $('update-result');
                    if (resultEl) {
                        resultEl.hidden = false;
                        resultEl.className = 'result-strip err';
                        resultEl.textContent = 'Failed to check for updates: ' + err;
                    }
                }
                updateBtn.disabled = false;
                updateBtn.textContent = 'Check for Updates';
            });
            updateBtn.setAttribute('data-listener-added', 'true');
        }
        
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