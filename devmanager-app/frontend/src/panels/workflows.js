// Panel Workflows (Issue #65, MVP): lista workflows, runs, editor inline
// simple (nombre/trigger/schedule/event + steps como filas), webhooks
// salientes y CI/CD por proyecto. Patrón panels: mount(ctx) →
// { onProjectChanged, refresh }.
import { showToast } from '../widgets/toast.js';

const TRIGGERS = ['manual', 'schedule', 'event'];
const EVENTS = ['server_started', 'server_stopped', 'tests_finished', 'webhook_received'];
const KINDS = ['command', 'notify', 'webhook', 'ci_trigger'];
const CI_TARGETS = ['github', 'gitlab', 'jenkins'];

function el(tag, className, text) {
    const n = document.createElement(tag);
    if (className) n.className = className;
    if (text !== undefined) n.textContent = text;
    return n;
}

function input(placeholder, value, mono) {
    const i = document.createElement('input');
    i.type = 'text';
    i.className = 'text-input' + (mono ? ' mono' : '');
    i.placeholder = placeholder || '';
    i.value = value || '';
    return i;
}

function numberInput(value, min, max) {
    const i = document.createElement('input');
    i.type = 'number';
    i.className = 'text-input';
    i.value = value == null ? '' : String(value);
    if (min != null) i.min = min;
    if (max != null) i.max = max;
    return i;
}

function select(options, value) {
    const s = document.createElement('select');
    s.className = 'text-input';
    options.forEach((o) => {
        const opt = document.createElement('option');
        opt.value = o;
        opt.textContent = o;
        if (o === value) opt.selected = true;
        s.appendChild(opt);
    });
    return s;
}

export function mount(ctx) {
    const { $, api, events } = ctx;

    let workflows = [];
    let editingId = null; // id en edición inline (o '__new__')

    function idx() {
        return ctx.selectedIndex();
    }

    function showResult(text, cls) {
        const n = $('wf-result');
        if (!n) return;
        n.textContent = text;
        n.className = `result-strip ${cls || ''}`;
        n.hidden = false;
        setTimeout(() => { n.hidden = true; }, 3500);
    }

    // ---- workflows list ----
    function renderList() {
        const box = $('wf-list');
        box.innerHTML = '';
        $('wf-count').textContent = `${workflows.length} workflow(s)`;
        $('wf-empty').hidden = workflows.length !== 0;
        workflows.forEach((w) => {
            const row = el('div', 'script-row wf-row');
            const name = el('span', 's-name', w.name || '(unnamed)');
            name.title = w.id || '';
            const badge = el('span', 's-badge', w.enabled ? w.trigger : 'disabled');
            const runBtn = el('button', 'btn btn-primary', 'Run');
            runBtn.disabled = !w.enabled;
            runBtn.addEventListener('click', async () => {
                const errs = await api.runWorkflow(idx(), w.id).catch((e) => [String((e && e.message) || e)]);
                if (errs && errs.length) showResult(errs.join('\n'), 'err');
                else showResult(`Workflow "${w.name}" started`, 'ok');
            });
            const editBtn = el('button', 'btn btn-accent', editingId === w.id ? 'Close' : 'Edit');
            editBtn.addEventListener('click', () => {
                editingId = editingId === w.id ? null : w.id;
                renderList();
                renderEditor();
            });
            const delBtn = el('button', 'btn btn-danger', 'Delete');
            delBtn.addEventListener('click', async () => {
                const ok = await ctx.messageDialog.confirm({
                    title: 'Delete workflow', message: `Delete "${w.name}" and its run history?`,
                    confirmLabel: 'Delete', destructive: true,
                });
                if (!ok) return;
                const errs = await api.deleteWorkflow(idx(), w.id).catch((e) => [String((e && e.message) || e)]);
                if (errs && errs.length) showResult(errs.join('\n'), 'err');
                else { showResult('Workflow deleted', 'ok'); refresh(); }
            });
            row.append(name, badge, runBtn, editBtn, delBtn);
            box.appendChild(row);
        });
    }

    // ---- editor ----
    function blankWorkflow() {
        return {
            id: '', name: 'New workflow', enabled: true, trigger: 'manual',
            schedule: '', event: 'server_started',
            steps: [{ id: '', name: 'step 1', kind: 'command', command: '', retry: 0, timeout_sec: 120 }],
        };
    }

    function renderEditor() {
        const box = $('wf-editor');
        box.innerHTML = '';
        const w = editingId === '__new__'
            ? blankWorkflow()
            : workflows.find((x) => x.id === editingId);
        if (!w) {
            box.hidden = true;
            return;
        }
        box.hidden = false;
        const form = el('div', 'group');
        form.appendChild(el('div', 'group-title', editingId === '__new__' ? 'New Workflow' : `Edit: ${w.name}`));

        const nameIn = input('Workflow name', w.name);
        const enabledCb = document.createElement('input');
        enabledCb.type = 'checkbox';
        enabledCb.checked = !!w.enabled;
        const enabledLbl = el('label', '', ' Enabled');
        enabledLbl.prepend(enabledCb);
        const triggerSel = select(TRIGGERS, w.trigger);
        const scheduleIn = input('@every 5m, @hourly, @daily o cron "M H DOM MON DOW"', w.schedule, true);
        const eventSel = select(EVENTS, w.event);

        const row1 = el('div', 'row-gap');
        row1.append(nameIn, enabledLbl, el('span', 'dim', 'trigger:'), triggerSel);
        form.appendChild(row1);
        const rowSched = el('div', 'row-gap');
        rowSched.append(el('span', 'dim', 'schedule:'), scheduleIn);
        const rowEvent = el('div', 'row-gap');
        rowEvent.append(el('span', 'dim', 'event:'), eventSel);
        form.appendChild(rowSched);
        form.appendChild(rowEvent);

        const syncTriggerRows = () => {
            rowSched.style.display = triggerSel.value === 'schedule' ? '' : 'none';
            rowEvent.style.display = triggerSel.value === 'event' ? '' : 'none';
        };
        triggerSel.addEventListener('change', syncTriggerRows);
        syncTriggerRows();

        // steps
        const stepsBox = el('div', 'wf-steps');
        const draftSteps = (w.steps || []).map((s) => ({ ...s }));
        const renderSteps = () => {
            stepsBox.innerHTML = '';
            draftSteps.forEach((s, si) => {
                const r = el('div', 'wf-step-row');
                const nm = input('step name', s.name);
                nm.addEventListener('input', () => { s.name = nm.value; });
                const kind = select(KINDS, s.kind);
                kind.addEventListener('change', () => { s.kind = kind.value; renderSteps(); });
                const cmd = input(
                    s.kind === 'command' ? 'command (runs in project dir)' :
                    s.kind === 'webhook' ? 'body (optional JSON)' :
                    s.kind === 'ci_trigger' ? 'message / body' : 'message',
                    s.kind === 'notify' && !s.body ? s.command : (s.body || s.command), true);
                cmd.style.minWidth = '220px';
                cmd.addEventListener('input', () => {
                    if (s.kind === 'notify') s.body = cmd.value;
                    else if (s.kind === 'webhook') s.body = cmd.value;
                    else s.command = cmd.value;
                });
                r.append(nm, kind, cmd);
                if (s.kind === 'webhook') {
                    const urlIn = input('https://... (POST target)', s.url, true);
                    urlIn.style.minWidth = '200px';
                    urlIn.addEventListener('input', () => { s.url = urlIn.value; });
                    r.appendChild(urlIn);
                }
                if (s.kind === 'ci_trigger') {
                    const tgt = select(CI_TARGETS, s.ci_target || 'github');
                    tgt.addEventListener('change', () => { s.ci_target = tgt.value; });
                    r.appendChild(tgt);
                }
                const retry = numberInput(s.retry || 0, 0, 3);
                retry.title = 'retry (0-3)';
                retry.style.width = '64px';
                retry.addEventListener('input', () => { s.retry = parseInt(retry.value || '0', 10); });
                const tmo = numberInput(s.timeout_sec || 120, 0, 3600);
                tmo.title = 'timeout seconds';
                tmo.style.width = '80px';
                tmo.addEventListener('input', () => { s.timeout_sec = parseInt(tmo.value || '0', 10); });
                r.append(el('span', 'dim', 'retry:'), retry, el('span', 'dim', 'timeout:'), tmo);
                const rm = el('button', 'btn btn-danger', '✕');
                rm.title = 'Remove step';
                rm.addEventListener('click', () => { draftSteps.splice(si, 1); renderSteps(); });
                r.appendChild(rm);
                stepsBox.appendChild(r);
            });
        };
        renderSteps();
        form.appendChild(el('div', 'group-title', 'Steps'));
        form.appendChild(stepsBox);
        const addStep = el('button', 'btn btn-accent', '+ Add step');
        addStep.addEventListener('click', () => {
            draftSteps.push({ id: '', name: `step ${draftSteps.length + 1}`, kind: 'command', command: '', retry: 0, timeout_sec: 120 });
            renderSteps();
        });
        form.appendChild(addStep);

        const saveRow = el('div', 'btn-row');
        const saveBtn = el('button', 'btn btn-success', 'Save workflow');
        saveBtn.addEventListener('click', async () => {
            const payload = {
                id: editingId === '__new__' ? '' : w.id,
                name: nameIn.value.trim(),
                enabled: enabledCb.checked,
                trigger: triggerSel.value,
                schedule: scheduleIn.value.trim(),
                event: eventSel.value,
                steps: draftSteps,
            };
            const errs = await api.saveWorkflow(idx(), payload).catch((e) => [String((e && e.message) || e)]);
            if (errs && errs.length) { showResult(errs.join('\n'), 'err'); return; }
            showResult('Workflow saved', 'ok');
            editingId = null;
            refresh();
        });
        const cancelBtn = el('button', 'btn btn-secondary', 'Cancel');
        cancelBtn.addEventListener('click', () => { editingId = null; renderList(); renderEditor(); });
        saveRow.append(saveBtn, cancelBtn);
        form.appendChild(saveRow);
        box.appendChild(form);
    }

    // ---- runs ----
    async function renderRuns() {
        const box = $('wf-runs');
        box.innerHTML = '';
        let runs = [];
        try {
            runs = await api.listWorkflowRuns() || [];
        } catch { runs = []; }
        // Solo runs de este proyecto: el backend guarda project name; filtra
        // por workflows conocidos del proyecto actual.
        const ids = new Set(workflows.map((w) => w.id));
        runs = runs.filter((r) => ids.has(r.workflowId)).slice(0, 20);
        $('wf-runs-count').textContent = `${runs.length} run(s)`;
        $('wf-runs-empty').hidden = runs.length !== 0;
        runs.forEach((r) => {
            const row = el('div', 'wf-run');
            const head = el('div', 'row-between');
            const wname = (workflows.find((w) => w.id === r.workflowId) || {}).name || r.workflowId;
            head.appendChild(el('span', 'strong', `${wname} · ${r.status}`));
            head.appendChild(el('span', 'dim', r.startedAt || ''));
            row.appendChild(head);
            if (r.logs && r.logs.length) {
                const pre = el('pre', 'mono dim wrap', r.logs.slice(-15).join('\n'));
                row.appendChild(pre);
            }
            box.appendChild(row);
        });
    }

    // ---- webhooks salientes ----
    async function renderWebhooks() {
        const box = $('wf-hooks');
        box.innerHTML = '';
        let hooks = [];
        try {
            hooks = await api.listWebhooks(idx()) || [];
        } catch { hooks = []; }
        $('wf-hooks-empty').hidden = hooks.length !== 0;
        hooks.forEach((h) => {
            const row = el('div', 'script-row');
            row.append(
                el('span', 's-name', h.name || '(unnamed)'),
                el('span', 's-cmd', `${h.enabled ? '' : '[off] '}→ ${(h.events || []).join(',')} @ ${h.url || ''}`),
            );
            const del = el('button', 'btn btn-danger', 'Delete');
            del.addEventListener('click', async () => {
                const errs = await api.deleteWebhook(idx(), h.id).catch((e) => [String((e && e.message) || e)]);
                if (errs && errs.length) showResult(errs.join('\n'), 'err');
                else { showResult('Webhook deleted', 'ok'); renderWebhooks(); }
            });
            row.appendChild(del);
            box.appendChild(row);
        });
    }

    // ---- CI/CD ----
    function ciField(obj, key, placeholder) {
        const i = input(placeholder, obj[key] || '', true);
        i.dataset.cikey = key;
        return i;
    }

    async function renderCI() {
        let cfg = { github: {}, gitlab: {}, jenkins: {} };
        try {
            cfg = await api.getCIConfig(idx()) || cfg;
        } catch { /* sin backend */ }
        cfg.github = cfg.github || {};
        cfg.gitlab = cfg.gitlab || {};
        cfg.jenkins = cfg.jenkins || {};
        const box = $('wf-ci');
        box.innerHTML = '';
        const groups = [
            ['GitHub', 'github', [
                ['owner', 'owner'], ['repo', 'repo'], ['workflow_file', 'workflow file (optional)'],
                ['ref', 'ref (default main)'], ['token_ref', 'token ref env:NAME'], ['api_base', 'api base (tests only)'],
            ]],
            ['GitLab', 'gitlab', [
                ['base_url', 'https://gitlab.example.com'], ['project_id', 'project id'],
                ['ref', 'ref (default main)'], ['token_ref', 'token ref env:NAME'],
            ]],
            ['Jenkins', 'jenkins', [
                ['base_url', 'https://jenkins.example.com'], ['job', 'job name'],
                ['token_ref', 'token ref env:NAME'],
            ]],
        ];
        const refs = {};
        groups.forEach(([title, key, fields]) => {
            const g = el('div', 'group');
            g.appendChild(el('div', 'group-title', title));
            refs[key] = {};
            fields.forEach(([f, ph]) => {
                const row = el('div', 'row-gap');
                row.append(el('span', 'dim', `${f}:`), ciField(cfg[key] || {}, f, ph));
                refs[key][f] = row.querySelector('input');
                g.appendChild(row);
            });
            const row = el('div', 'btn-row');
            const trig = el('button', 'btn btn-primary', `Trigger ${key}`);
            trig.addEventListener('click', async () => {
                let res = null;
                try {
                    res = await api.triggerCIBuild(idx(), key);
                } catch (e) {
                    showResult(`CI ${key}: ${String((e && e.message) || e)}`, 'err');
                    return;
                }
                showResult(`CI ${key}: ${(res && res.message) || 'done'}`, res && res.ok ? 'ok' : 'err');
                renderCIStatus();
            });
            const st = el('span', 'dim', '');
            st.id = `wf-ci-status-${key}`;
            row.append(trig, st);
            g.appendChild(row);
            box.appendChild(g);
        });
        const save = el('button', 'btn btn-success', 'Save CI config');
        save.addEventListener('click', async () => {
            const payload = {};
            Object.keys(refs).forEach((k) => {
                payload[k] = {};
                Object.keys(refs[k]).forEach((f) => { payload[k][f] = refs[k][f].value.trim(); });
            });
            const errs = await api.saveCIConfig(idx(), payload).catch((e) => [String((e && e.message) || e)]);
            if (errs && errs.length) showResult(errs.join('\n'), 'err');
            else showResult('CI config saved', 'ok');
        });
        box.appendChild(save);
        renderCIStatus();
    }

    async function renderCIStatus() {
        for (const t of CI_TARGETS) {
            const n = $(`wf-ci-status-${t}`);
            if (!n) continue;
            try {
                const st = await api.getCIStatus(idx(), t);
                n.textContent = st && st.status !== 'unknown' ? `${st.status}: ${st.message || ''}` : 'no builds yet';
            } catch { n.textContent = ''; }
        }
    }

    async function renderListener() {
        const n = $('wf-listener');
        if (!n) return;
        try {
            const addr = await api.getWorkflowListenerAddr();
            n.textContent = addr ? `Incoming webhooks: POST http://${addr}/hook/{workflowId}` : 'Incoming webhook listener: off';
        } catch { n.textContent = ''; }
    }

    // ---- refresh general ----
    async function refresh() {
        const i = idx();
        if (i < 0) {
            workflows = [];
            renderList();
            $('wf-editor').innerHTML = '';
            $('wf-runs').innerHTML = '';
            $('wf-hooks').innerHTML = '';
            return;
        }
        try {
            workflows = await api.listWorkflows(i) || [];
        } catch { workflows = []; }
        renderList();
        renderEditor();
        renderRuns();
        renderWebhooks();
        renderCI();
        renderListener();
    }

    // wiring estático
    $('wf-add').addEventListener('click', () => {
        editingId = '__new__';
        renderList();
        renderEditor();
    });
    $('wf-refresh').addEventListener('click', refresh);
    $('wf-fire').addEventListener('click', async () => {
        const ev = $('wf-event-select').value;
        const errs = await api.triggerEvent(idx(), ev).catch((e) => [String((e && e.message) || e)]);
        if (errs && errs.length) showResult(errs.join('\n'), 'err');
        else showResult(`Event "${ev}" fired`, 'ok');
    });
    $('wf-hook-add').addEventListener('click', async () => {
        const url = $('wf-hook-url').value.trim();
        if (!url) { showResult('Webhook URL required', 'err'); return; }
        const evSel = $('wf-hook-events').value;
        const errs = await api.saveWebhook(idx(), {
            id: '', name: $('wf-hook-name').value.trim() || 'hook',
            url, events: [evSel], enabled: true,
        }).catch((e) => [String((e && e.message) || e)]);
        if (errs && errs.length) showResult(errs.join('\n'), 'err');
        else {
            showResult('Webhook saved', 'ok');
            $('wf-hook-url').value = '';
            renderWebhooks();
        }
    });

    events().EventsOn('workflow:finished', ({ index, name, status }) => {
        if (index === idx()) {
            renderRuns();
            showToast('Workflow', `"${name}" ${status}`, status === 'success' ? 'success' : 'error');
        }
    });
    events().EventsOn('workflow:started', () => {
        if (idx() >= 0) renderRuns();
    });

    return {
        onProjectChanged() {
            editingId = null;
            const r = $('wf-result');
            if (r) r.hidden = true;
            refresh();
        },
        refresh,
    };
}
