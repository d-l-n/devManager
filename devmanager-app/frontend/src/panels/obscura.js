const MAX_LINES = 500;

export function mount(ctx) {
    const { $, api, events } = ctx;

    function currentUrl() {
        return $('obs-url').value.trim();
    }

    function setStatus(state) {
        const label = $('obs-status-label');
        const running = state === 'running';
        label.textContent = `Status: ${state}`;
        label.className = running ? 'strong' : 'dim';
        ['obs-screenshot', 'obs-dump-text', 'obs-dump-markdown', 'obs-eval-run', 'obs-fetch-run'].forEach((id) => {
            $(id).disabled = running;
        });
        $('obs-stop').disabled = !running;
    }

    function setBinInfo(bin) {
        const el = $('obs-bin-info');
        if (bin && bin.binaryExists) {
            el.textContent = `Binary: ready (${bin.binaryPath})`;
            el.className = 'dim';
        } else {
            el.textContent = 'Binary: missing — first action downloads it automatically';
            el.className = 'dim warn-text';
        }
    }

    function appendOutput(line, isError) {
        const out = $('obs-output');
        const div = document.createElement('div');
        div.className = `log-line${isError ? ' err' : ''}`;
        div.textContent = line;
        out.appendChild(div);
        while (out.childElementCount > MAX_LINES) out.removeChild(out.firstChild);
        out.scrollTop = out.scrollHeight;
    }

    function clearOutput() {
        $('obs-output').innerHTML = '';
    }

    async function syncStatus() {
        const i = ctx.selectedIndex();
        if (i < 0) return;
        try {
            const st = await api.getObscuraStatus(i);
            setStatus(st.state || 'idle');
            setBinInfo(st);
        } catch (e) { console.warn('[obscura status]', e?.message || e); }
    }

    async function runAction(fn, okMessage) {
        const i = ctx.selectedIndex();
        if (i < 0) return;
        try {
            await fn(i);
            if (okMessage) appendOutput(okMessage, false);
        } catch (err) {
            appendOutput(String(err && err.message ? err.message : err), true);
        }
    }

    $('obs-screenshot').addEventListener('click', () =>
        runAction(async (i) => {
            await api.obscuraScreenshot(i, currentUrl());
            appendOutput('Screenshot saved to test-results/obscura/', false);
        }));

    $('obs-dump-text').addEventListener('click', () =>
        runAction((i) => api.obscuraDump(i, currentUrl(), 'text')));

    $('obs-dump-markdown').addEventListener('click', () =>
        runAction((i) => api.obscuraDump(i, currentUrl(), 'markdown')));

    $('obs-eval-run').addEventListener('click', () => {
        const js = $('obs-eval').value.trim();
        if (!js) return;
        runAction((i) => api.obscuraEval(i, currentUrl(), js));
    });
    $('obs-eval').addEventListener('keydown', (e) => {
        if (e.key === 'Enter') $('obs-eval-run').click();
    });

    $('obs-fetch-run').addEventListener('click', () => {
        const cmd = $('obs-fetch').value.trim();
        if (!cmd) return;
        runAction((i) => api.obscuraFetch(i, cmd));
    });
    $('obs-fetch').addEventListener('keydown', (e) => {
        if (e.key === 'Enter') $('obs-fetch-run').click();
    });

    $('obs-stop').addEventListener('click', () => api.stopObscura(ctx.selectedIndex()));

    $('obs-open-url').addEventListener('click', () => {
        const url = currentUrl();
        if (url) api.openURL(url);
    });

    events().EventsOn('obs:state', ({ index, state }) => {
        if (index === ctx.selectedIndex()) setStatus(state);
    });
    events().EventsOn('obs:log', ({ index, line, isError }) => {
        // Al log global del proyecto (paridad panels) + al buffer local.
        ctx.appendLog(index, line, isError);
        if (index === ctx.selectedIndex()) appendOutput(line, isError);
    });

    async function onProjectChanged(p) {
        const url = p && p.server && p.server.url;
        $('obs-url').value = url || '';
        if (ctx.selectedIndex() >= 0) {
            clearOutput();
            await syncStatus();
        }
    }

    return { onProjectChanged };
}