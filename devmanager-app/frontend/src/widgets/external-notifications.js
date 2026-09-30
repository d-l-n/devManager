// Notificaciones externas (Issue #66): sección de settings con plataformas
// (Slack/Discord/Telegram/Teams), reglas de enrutado, historial de entregas y
// botón de evento de prueba. Se monta dentro de settings-notifications-section
// para no tocar el layout del resto de la vista.
import { showToast } from './toast.js';

const esc = (s) => String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

const PLATFORM_LABELS = {
    slack: 'Slack',
    discord: 'Discord',
    telegram: 'Telegram',
    teams: 'Microsoft Teams',
};

const EVENT_LABELS = {
    app: 'App notifications',
    server_started: 'Server started',
    server_stopped: 'Server stopped',
    server_crashed: 'Server error',
    tests_finished: 'Tests finished',
    backup_done: 'Backup done',
    workflow_step: 'Workflow step',
    build_failed: 'Build failed',
};

function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

// notifyPanel mantiene el estado de la subsección y renderiza dentro de container.
export function mountExternalNotifications(container) {
    if (!container) return null;
    const state = { cfg: null, history: [], showForm: false, editing: '', showRuleForm: false, editingRule: '' };

    async function refresh() {
        try {
            const app = window.go?.main?.App;
            if (!app || typeof app.NotifyGetConfig !== 'function') throw new Error('no backend');
            const [cfg, history] = await Promise.all([
                app.NotifyGetConfig(),
                app.NotifyHistory(50),
            ]);
            state.cfg = cfg;
            state.history = history || [];
        } catch (err) {
            console.warn('[notify] config load failed', err);
            container.innerHTML = '';
            container.appendChild(el('div', 'settings-section-description', 'External notifications unavailable (backend not ready).'));
            return;
        }
        render();
    }

    function render() {
        container.innerHTML = '';
        const desc = el('div', 'settings-section-description',
            'Send notifications to Slack, Discord, Telegram or Microsoft Teams. Route events with rules, review delivery history and test the setup.');
        container.appendChild(desc);

        // Plataformas
        const list = el('div', 'settings-options');
        (state.cfg?.platforms || []).forEach((p) => {
            const row = el('div', 'settings-option notify-row');
            const copy = el('span', 'settings-option-copy');
            const label = el('span', 'settings-option-label', `${p.name} — ${PLATFORM_LABELS[p.platform] || p.platform}`);
            const sub = el('span', 'settings-option-description',
                `min: ${p.min_priority || 'info'} · events: ${p.events?.length ? p.events.map((e) => EVENT_LABELS[e] || e).join(', ') : 'all'}${p.enabled ? '' : ' · disabled'}`);
            copy.appendChild(label);
            copy.appendChild(sub);
            row.appendChild(copy);
            const actions = el('span', 'row-gap');
            const editBtn = el('button', 'btn btn-small', 'Edit');
            editBtn.addEventListener('click', () => { state.editing = p.id; state.showForm = true; render(); });
            const delBtn = el('button', 'btn btn-small btn-danger', 'Delete');
            delBtn.addEventListener('click', async () => {
                const errs = await window.go.main.App.NotifyDeletePlatform(p.id);
                if (errs?.length) showToastErrs(errs);
                refresh();
            });
            actions.appendChild(editBtn);
            actions.appendChild(delBtn);
            row.appendChild(actions);
            list.appendChild(row);
        });
        if (!state.cfg?.platforms?.length) {
            list.appendChild(el('div', 'dim', 'No external platforms configured.'));
        }
        container.appendChild(list);

        const addBtn = el('button', 'btn btn-small btn-primary', state.showForm ? 'Close form' : '+ Add platform');
        addBtn.addEventListener('click', () => { state.showForm = !state.showForm; state.editing = ''; render(); });
        container.appendChild(addBtn);

        if (state.showForm) container.appendChild(platformForm());

        // Reglas
        container.appendChild(el('div', 'settings-divider'));
        container.appendChild(el('div', 'settings-option-label', 'Alert rules'));
        const ruleList = el('div', 'settings-options');
        (state.cfg?.rules || []).forEach((r) => {
            const row = el('div', 'settings-option notify-row');
            const copy = el('span', 'settings-option-copy');
            copy.appendChild(el('span', 'settings-option-label', r.name));
            copy.appendChild(el('span', 'settings-option-description',
                `${r.event === 'all' ? 'All events' : (EVENT_LABELS[r.event] || r.event)} · min: ${r.min_priority}${r.enabled ? '' : ' · disabled'}`));
            row.appendChild(copy);
            const actions = el('span', 'row-gap');
            const editBtn = el('button', 'btn btn-small', 'Edit');
            editBtn.addEventListener('click', () => { state.editingRule = r.id; state.showRuleForm = true; render(); });
            const delBtn = el('button', 'btn btn-small btn-danger', 'Delete');
            delBtn.addEventListener('click', async () => {
                const errs = await window.go.main.App.NotifyDeleteRule(r.id);
                if (errs?.length) showToastErrs(errs);
                refresh();
            });
            actions.appendChild(editBtn);
            actions.appendChild(delBtn);
            row.appendChild(actions);
            ruleList.appendChild(row);
        });
        if (!state.cfg?.rules?.length) {
            ruleList.appendChild(el('div', 'dim', 'No rules: platform min_priority governs.'));
        }
        container.appendChild(ruleList);

        const addRuleBtn = el('button', 'btn btn-small btn-primary', state.showRuleForm ? 'Close rule form' : '+ Add rule');
        addRuleBtn.addEventListener('click', () => { state.showRuleForm = !state.showRuleForm; state.editingRule = ''; render(); });
        container.appendChild(addRuleBtn);
        if (state.showRuleForm) container.appendChild(ruleForm());

        // Test + historial
        container.appendChild(el('div', 'settings-divider'));
        const testRow = el('div', 'row-gap');
        const testBtn = el('button', 'btn btn-small btn-accent', 'Send test notification');
        testBtn.addEventListener('click', async () => {
            const errs = await window.go.main.App.NotifyEventFire(0, 'app');
            if (errs?.length) showToastErrs(errs);
            setTimeout(refresh, 800);
        });
        const refreshBtn = el('button', 'btn btn-small', 'Refresh history');
        refreshBtn.addEventListener('click', refresh);
        testRow.appendChild(testBtn);
        testRow.appendChild(refreshBtn);
        container.appendChild(testRow);

        const hist = el('div', 'notify-history');
        state.history.forEach((d) => {
            const row = el('div', 'notify-history-row');
            const statusCls = d.status === 'sent' ? 'ok' : (d.status === 'rate_limited' ? 'warn' : 'err');
            row.innerHTML = `<span class="mini-badge ${statusCls}">${esc(d.status)}</span>` +
                `<span class="dim">${new Date(d.at).toLocaleString()}</span>` +
                `<span>${esc(d.name)} (${esc(d.platform)})</span>` +
                `<span class="dim">${esc(d.event)} · ${esc(d.title)}${d.error ? ' · ' + esc(d.error) : ''}</span>`;
            hist.appendChild(row);
        });
        if (!state.history.length) hist.appendChild(el('div', 'dim', 'No deliveries yet.'));
        container.appendChild(hist);
    }

    function showToastErrs(errs) {
        showToast('Notifications', errs.join('\n'), 'error');
    }

    function platformForm() {
        const editing = (state.cfg?.platforms || []).find((p) => p.id === state.editing);
        const wrap = el('div', 'settings-options notify-form');
        const name = el('input', 'text-input'); name.placeholder = 'Name'; name.value = editing?.name || '';
        const platform = el('select', 'text-input');
        Object.entries(PLATFORM_LABELS).forEach(([v, l]) => {
            const o = el('option', '', l); o.value = v; platform.appendChild(o);
        });
        platform.value = editing?.platform || 'slack';
        const url = el('input', 'text-input'); url.placeholder = 'Webhook URL (https://...)'; url.value = editing?.webhook_url || '';
        const token = el('input', 'text-input'); token.placeholder = 'Telegram bot token'; token.value = editing?.bot_token || '';
        const chat = el('input', 'text-input'); chat.placeholder = 'Telegram chat ID'; chat.value = editing?.chat_id || '';
        const minPrio = el('select', 'text-input');
        ['info', 'warning', 'critical'].forEach((v) => {
            const o = el('option', '', 'min priority: ' + v); o.value = v; minPrio.appendChild(o);
        });
        minPrio.value = editing?.min_priority || 'info';
        const enabled = el('input'); enabled.type = 'checkbox'; enabled.checked = editing ? !!editing.enabled : true;

        const syncTG = () => { token.parentElement.style.display = platform.value === 'telegram' ? '' : 'none'; chat.parentElement.style.display = platform.value === 'telegram' ? '' : 'none'; url.parentElement.style.display = platform.value === 'telegram' ? 'none' : ''; };
        platform.addEventListener('change', syncTG);

        const mkRow = (label, input) => {
            const row = el('div', 'settings-option');
            row.appendChild(input);
            const copy = el('span', 'settings-option-copy');
            copy.appendChild(el('span', 'settings-option-label', label));
            row.appendChild(copy);
            return row;
        };
        wrap.appendChild(mkRow('Name', name));
        wrap.appendChild(mkRow('Platform', platform));
        wrap.appendChild(mkRow('Webhook URL', url));
        wrap.appendChild(mkRow('Bot token', token));
        wrap.appendChild(mkRow('Chat ID', chat));
        wrap.appendChild(mkRow('Minimum priority', minPrio));
        wrap.appendChild(mkRow('Enabled', enabled));

        const save = el('button', 'btn btn-small btn-success', 'Save platform');
        save.addEventListener('click', async () => {
            const errs = await window.go.main.App.NotifySavePlatform({
                id: editing?.id || '',
                name: name.value.trim(),
                platform: platform.value,
                webhook_url: url.value.trim(),
                bot_token: token.value.trim(),
                chat_id: chat.value.trim(),
                min_priority: minPrio.value,
                events: editing?.events || [],
                enabled: enabled.checked,
            });
            if (errs?.length) { showToastErrs(errs); return; }
            state.showForm = false; state.editing = '';
            refresh();
        });
        wrap.appendChild(save);
        setTimeout(syncTG, 0);
        return wrap;
    }

    function ruleForm() {
        const editing = (state.cfg?.rules || []).find((r) => r.id === state.editingRule);
        const wrap = el('div', 'settings-options notify-form');
        const name = el('input', 'text-input'); name.placeholder = 'Rule name'; name.value = editing?.name || '';
        const event = el('select', 'text-input');
        const allOpt = el('option', '', 'All events'); allOpt.value = 'all'; event.appendChild(allOpt);
        Object.entries(EVENT_LABELS).forEach(([v, l]) => {
            if (v === 'app') return;
            const o = el('option', '', l); o.value = v; event.appendChild(o);
        });
        event.value = editing?.event || 'all';
        const minPrio = el('select', 'text-input');
        ['info', 'warning', 'critical'].forEach((v) => {
            const o = el('option', '', 'min priority: ' + v); o.value = v; minPrio.appendChild(o);
        });
        minPrio.value = editing?.min_priority || 'info';
        const enabled = el('input'); enabled.type = 'checkbox'; enabled.checked = editing ? !!editing.enabled : true;

        const mkRow = (label, input) => {
            const row = el('div', 'settings-option');
            row.appendChild(input);
            const copy = el('span', 'settings-option-copy');
            copy.appendChild(el('span', 'settings-option-label', label));
            row.appendChild(copy);
            return row;
        };
        wrap.appendChild(mkRow('Name', name));
        wrap.appendChild(mkRow('Event', event));
        wrap.appendChild(mkRow('Minimum priority', minPrio));
        wrap.appendChild(mkRow('Enabled', enabled));

        const save = el('button', 'btn btn-small btn-success', 'Save rule');
        save.addEventListener('click', async () => {
            const errs = await window.go.main.App.NotifySaveRule({
                id: editing?.id || '',
                name: name.value.trim(),
                event: event.value,
                min_priority: minPrio.value,
                enabled: enabled.checked,
            });
            if (errs?.length) { showToastErrs(errs); return; }
            state.showRuleForm = false; state.editingRule = '';
            refresh();
        });
        wrap.appendChild(save);
        return wrap;
    }

    refresh();
    return { refresh };
}
