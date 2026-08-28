const GIT_ACTIONS = ['Pull', 'Fetch', 'Stash'];

export function mount(ctx) {
    const { $, api, events } = ctx;

    const busy = {}; // index -> hay op git en curso en ese proyecto

    function showResult(text, cls) {
        const el = $('git-result');
        el.textContent = text;
        el.className = `result-strip ${cls}`;
        el.hidden = false;
    }

    function render(status) {
        const hasRepo = !!status.isRepo;
        ['git-refresh', 'git-pull', 'git-fetch', 'git-stash']
            .forEach((id) => { $(id).disabled = !hasRepo; });
        $('git-empty').hidden = hasRepo;
        document.querySelector('#panel-git .group').style.display = hasRepo ? '' : 'none';
        if (!hasRepo) {
            $('git-result').hidden = true;
            return;
        }
        $('git-branch').textContent = `Branch: ${status.branch || 'unknown'}`;
        const dirty = $('git-dirty-badge');
        dirty.textContent = status.isDirty ? 'ÔùÅ Uncommitted changes' : 'ÔùÅ Clean';
        dirty.className = `mini-badge ${status.isDirty ? 'dirty' : 'clean'}`;
        const sync = $('git-sync');
        if (status.hasUpstream) {
            sync.textContent = `Ôåæ ${status.ahead}   Ôåô ${status.behind}`;
            sync.className = '';
        } else {
            sync.textContent = '(no upstream)';
            sync.className = 'dim';
        }
        const lc = status.lastCommit;
        $('git-commit').textContent = lc
            ? `Last commit: ${lc.hash} ┬À ${lc.subject} ┬À ${lc.dateRel}`
            : 'Last commit: ÔÇö';
    }

    async function refresh() {
        const i = ctx.selectedIndex();
        if (i < 0) return;
        render(await api.getGitStatus(i));
    }

    function setButtons(enabled) {
        GIT_ACTIONS.forEach((a) => { $(`git-${a.toLowerCase()}`).disabled = !enabled; });
        $('git-refresh').disabled = !enabled;
    }

    GIT_ACTIONS.forEach((action) => {
        $(`git-${action.toLowerCase()}`).addEventListener('click', () => {
            busy[ctx.selectedIndex()] = true;
            showResult(`Running: git ${action.toLowerCase()}ÔÇª`, 'info');
            setButtons(false);
            api.gitAction(ctx.selectedIndex(), action);
        });
    });
    $('git-refresh').addEventListener('click', refresh);

    events().EventsOn('git:output', ({ index, text, isError }) =>
        ctx.appendLog(index, text, isError));
    events().EventsOn('git:finished', ({ index, name, exitCode, cleanStash }) => {
        busy[index] = false;
        if (index !== ctx.selectedIndex()) {
            refresh();
            setButtons(!busy[ctx.selectedIndex()]);
            return;
        }
        if (exitCode === 0) {
            if (cleanStash) {
                showResult('Nothing to stash ÔÇö working tree clean.', 'info');
            } else {
                showResult(`${name} completed successfully.`, 'ok');
            }
        } else {
            showResult(`${name} failed (exit code ${exitCode}). See Logs tab.`, 'err');
        }
        setButtons(true);
        refresh();
    });

    return {
        onProjectChanged: () => {
            $('git-result').hidden = true;
            refresh();
            setButtons(!busy[ctx.selectedIndex()]);
        },
        refresh,
    };
}
