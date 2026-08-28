export function mount(ctx) {
    const { $, api, events } = ctx;
    let rows = []; // [{name, command, el}]

    function setActive(name) {
        const label = $('script-active-label');
        const running = !!name;
        label.textContent = running ? `Running script: ${name}` : 'No script running';
        label.className = running ? 'strong warn-text' : 'dim';
        $('script-stop').disabled = !running;
        rows.forEach((r) => {
            const isIt = running && r.name === name;
            r.el.classList.toggle('running', isIt);
            r.badge.textContent = isIt ? 'Running...' : 'Idle';
            r.runBtn.disabled = isIt;
        });
    }

    function render(scripts) {
        const list = $('scripts-list');
        list.innerHTML = '';
        rows = [];
        const empty = $('scripts-empty');
        if (!scripts || !scripts.length) {
            empty.hidden = false;
            empty.textContent = 'No scripts found in package.json';
            return;
        }
        empty.hidden = true;
        for (const s of scripts) {
            const row = document.createElement('div');
            row.className = 'script-row';
            const name = document.createElement('span');
            name.className = 's-name';
            name.textContent = s.name;
            const cmd = document.createElement('span');
            cmd.className = 's-cmd';
            cmd.textContent = s.command;
            cmd.title = s.command;
            const badge = document.createElement('span');
            badge.className = 's-badge';
            badge.textContent = 'Idle';
            const runBtn = document.createElement('button');
            runBtn.className = 'btn btn-primary';
            runBtn.textContent = 'Run';
            runBtn.addEventListener('click', () =>
                api.runScript(ctx.selectedIndex(), s.name, s.command));
            row.append(name, cmd, badge, runBtn);
            list.appendChild(row);
            rows.push({ name: s.name, command: s.command, el: row, badge, runBtn });
        }
        filter($('scripts-filter').value);
    }

    function filter(q) {
        q = (q || '').trim().toLowerCase();
        let visible = 0;
        for (const r of rows) {
            const match = !q || r.name.toLowerCase().includes(q) || r.command.toLowerCase().includes(q);
            r.el.style.display = match ? '' : 'none';
            if (match) visible++;
        }
        const empty = $('scripts-empty');
        if (rows.length && visible === 0) {
            empty.hidden = false;
            empty.textContent = `No scripts match '${q}'`;
        } else if (rows.length) {
            empty.hidden = true;
        }
    }

    async function load() {
        const i = ctx.selectedIndex();
        if (i < 0) {
            $('scripts-list').innerHTML = '';
            rows = [];
            const empty = $('scripts-empty');
            empty.hidden = false;
            empty.textContent = 'Select a project to view scripts';
            setActive(null);
            return;
        }
        const scripts = await api.getScripts(i);
        render(scripts);
        const st = await api.getScriptStatus(i);
        setActive(st.running ? st.activeName : null);
    }

    $('scripts-filter').addEventListener('input', (e) => filter(e.target.value));
    $('custom-run').addEventListener('click', () => {
        const input = $('custom-command');
        const cmd = input.value.trim();
        if (cmd) api.runScript(ctx.selectedIndex(), 'custom', cmd);
        input.value = '';
    });
    $('custom-command').addEventListener('keydown', (e) => {
        if (e.key === 'Enter') $('custom-run').click();
    });
    $('script-stop').addEventListener('click', () => api.stopScript(ctx.selectedIndex()));

    events().EventsOn('script:started', ({ index, name }) => {
        if (index === ctx.selectedIndex()) setActive(name);
    });
    events().EventsOn('script:finished', ({ index }) => {
        if (index === ctx.selectedIndex()) setActive(null);
    });
    events().EventsOn('script:log', ({ index, line, isError }) =>
        ctx.appendLog(index, line, isError));

    return { onProjectChanged: load };
}
