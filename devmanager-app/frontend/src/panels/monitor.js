import { icon } from '../icons.js';

const POLL_MS = 3000;

export function mount(ctx) {
    const { $, api, events } = ctx;

    let visible = false;
    let timer = null;
    let pollEnabled = true;

    function cpuColor(cpu) {
        if (cpu >= 80) return 'var(--err)';
        if (cpu >= 50) return 'var(--warn)';
        return 'var(--ok)';
    }

    function portRowEl(row) {
        const el = document.createElement('div');
        el.className = 'mon-row';

        const name = document.createElement('span');
        name.className = 'mon-name';
        name.textContent = row.name;
        el.appendChild(name);

        const port = document.createElement('span');
        port.className = 'mono';
        port.textContent = `:${row.port}`;
        el.appendChild(port);

        if (row.state === 'ours') {
            const badge = document.createElement('span');
            badge.className = 'mini-badge primary';
            badge.append(icon('pinned'));
            badge.appendChild(document.createTextNode(`Used by '${row.ownerName}'`));
            el.appendChild(badge);
        } else if (row.state === 'foreign') {
            const badge = document.createElement('span');
            badge.className = 'mini-badge warn';
            badge.append(icon('alert'));
            badge.appendChild(document.createTextNode(`${row.ownerName} (PID ${row.ownerPID})`));
            el.appendChild(badge);
        } else {
            const free = document.createElement('span');
            free.className = 'dim';
            free.append(icon('check'));
            free.appendChild(document.createTextNode('Free'));
            el.appendChild(free);
        }

        if (row.state === 'foreign' && row.ownerPID > 0) {
            const spacer = document.createElement('span');
            spacer.className = 'mon-spacer';
            el.appendChild(spacer);

            const kill = document.createElement('button');
            kill.className = 'btn btn-danger';
            kill.textContent = 'Kill';
            kill.addEventListener('click', async () => {
                if (!confirm(`Terminate process tree with PID ${row.ownerPID}? This cannot be undone`)) return;
                await api.killTree(row.ownerPID);
                refresh();
            });
            el.appendChild(kill);
        }
        return el;
    }

    function resRowEl(row) {
        const el = document.createElement('div');
        el.className = 'mon-row';

        const name = document.createElement('span');
        name.className = 'mon-name';
        name.textContent = row.name;
        el.appendChild(name);

        const pid = document.createElement('span');
        pid.className = 'mono dim';
        pid.textContent = `PID ${row.pid}` + (row.children ? ` (+${row.children})` : '');
        el.appendChild(pid);

        const bar = document.createElement('div');
        bar.className = 'cpu-bar';
        const fill = document.createElement('div');
        fill.className = 'cpu-fill';
        fill.style.transform = `scaleX(${Math.min(row.cpu, 100) / 100})`;
        fill.style.transformOrigin = 'left center';
        fill.style.background = cpuColor(row.cpu);
        bar.appendChild(fill);
        el.appendChild(bar);

        const usage = document.createElement('span');
        usage.className = 'mono mon-usage';
        usage.textContent = `${Number(row.cpu).toFixed(1)}%  ${Number(row.rss).toFixed(1)} MB`;
        el.appendChild(usage);
        return el;
    }

    function render(data) {
        const rows = data.portRows || [];
        const ours = rows.filter((r) => r.state === 'ours').length;
        const conflicts = rows.filter((r) => r.state === 'foreign').length;
        $('mon-summary').textContent =
            `${rows.length} ports \u00B7 ${ours} in use \u00B7 ${conflicts} conflict(s)`;

        const ports = $('mon-ports');
        ports.innerHTML = '';
        $('mon-ports-empty').hidden = rows.length > 0;
        rows.forEach((row) => ports.appendChild(portRowEl(row)));

        const res = $('mon-res');
        res.innerHTML = '';
        const empty = $('mon-res-empty');
        if (!pollEnabled) {
            empty.textContent = 'Resource polling disabled in Settings.';
            empty.hidden = false;
        } else if (!data.resRows || data.resRows.length === 0) {
            empty.textContent = 'No servers running.';
            empty.hidden = false;
        } else {
            empty.hidden = true;
            data.resRows.forEach((row) => res.appendChild(resRowEl(row)));
        }
    }

    async function refresh() {
        render(await api.getMonitorData());
    }

    // Paridad _update_timer_state: interval solo si visible + auto + setting.
    function updateTimer() {
        const shouldRun = visible && pollEnabled && $('mon-auto').checked;
        if (shouldRun && timer === null) {
            refresh(); // refresh inmediato al activarse
            timer = setInterval(refresh, POLL_MS);
        } else if (!shouldRun && timer !== null) {
            clearInterval(timer);
            timer = null;
        }
    }

    $('mon-refresh').addEventListener('click', refresh);
    $('mon-auto').addEventListener('change', updateTimer);

    events().EventsOn('settings:changed', ({ key, value }) => {
        if (key !== 'monitor_polling') return;
        pollEnabled = value === 'true';
        updateTimer();
        if (visible) refresh();
    });

    api.getSettings()
        .then((s) => { if (s) pollEnabled = !!s.monitor_polling; })
        .catch(() => {});

    return {
        refresh,
        setVisible(v) {
            visible = v;
            updateTimer();
            if (v && timer === null) refresh();
        },
    };
}
