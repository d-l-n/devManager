const STATE_TEXT = {
    idle: ['Idle', 'idle'],
    starting: ['Starting...', 'starting'],
    running: ['Running...', 'running'],
    passed: ['Passed', 'passed'],
    failed: ['Failed', 'failed'],
    error: ['Error', 'error'],
};

export function mount(ctx) {
    const { $, api, events } = ctx;

    function apply(state) {
        const label = $('pw-status-label');
        const conf = STATE_TEXT[state] || STATE_TEXT.idle;
        label.className = `pw-status ${conf[1]}`;
        label.textContent = `Status: ${conf[0]}`;

        const busy = state === 'starting' || state === 'running';
        ['pw-run', 'pw-ui', 'pw-debug'].forEach((id) => { $(id).disabled = busy; });
        $('pw-stop').disabled = !busy;
        $('pw-report').disabled = false;
    }

    async function refresh() {
        const i = ctx.selectedIndex();
        if (i < 0) return;
        const s = await api.getPlaywrightStatus(i);
        apply(s.state);
    }

    $('pw-run').addEventListener('click', () => api.runTests(ctx.selectedIndex()));
    $('pw-ui').addEventListener('click', () => api.runUI(ctx.selectedIndex()));
    $('pw-debug').addEventListener('click', () => api.runDebug(ctx.selectedIndex()));
    $('pw-report').addEventListener('click', () => api.showReport(ctx.selectedIndex()));
    $('pw-stop').addEventListener('click', () => api.stopPlaywright(ctx.selectedIndex()));

    events().EventsOn('pw:state', ({ index, state }) => {
        if (index === ctx.selectedIndex()) apply(state);
    });
    events().EventsOn('pw:log', ({ index, line, isError }) =>
        ctx.appendLog(index, line, isError));

    return {
        onProjectChanged(project) {
            if (!project || !project.playwright || !project.playwright.enabled) {
                const label = $('pw-status-label');
                label.textContent = 'Status: Playwright disabled';
                label.className = 'pw-status disabled';
                ['pw-run', 'pw-ui', 'pw-debug', 'pw-stop'].forEach((id) => { $(id).disabled = true; });
                return;
            }
            apply('idle');
            refresh();
        },
        refresh,
    };
}
