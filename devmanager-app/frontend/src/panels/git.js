const GIT_ACTIONS = ['Pull', 'Fetch', 'Stash'];

export function mount(ctx) {
    const { $, api, events } = ctx;

    const busy = {}; // index -> hay op git en curso en ese proyecto
    let selBranch = null; // rama seleccionada en la lista
    let selTag = null;    // tag seleccionado en la lista

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
        document.querySelectorAll('#panel-git .group').forEach((g) => { g.style.display = hasRepo ? '' : 'none'; });
        if (!hasRepo) {
            $('git-result').hidden = true;
            return;
        }
        $('git-branch').textContent = `Branch: ${status.branch || 'unknown'}`;
        const dirty = $('git-dirty-badge');
        dirty.textContent = status.isDirty ? 'Uncommitted changes' : 'Clean';
        dirty.className = `mini-badge ${status.isDirty ? 'dirty' : 'clean'}`;
        const sync = $('git-sync');
        if (status.hasUpstream) {
            sync.textContent = `↑ ${status.ahead}   ↓ ${status.behind}`;
            sync.className = '';
        } else {
            sync.textContent = '(no upstream)';
            sync.className = 'dim';
        }
        const lc = status.lastCommit;
        $('git-commit').textContent = lc
            ? `Last commit: ${lc.hash} · ${lc.subject} · ${lc.dateRel}`
            : 'Last commit: —';
    }

    // Render del diff (Issue #63)
    function renderDiff(diffs) {
        const box = $('git-diff');
        const empty = $('git-diff-empty');
        box.innerHTML = '';
        if (!diffs || diffs.length === 0) {
            empty.hidden = false;
            box.hidden = true;
            return;
        }
        empty.hidden = true;
        box.hidden = false;
        diffs.forEach((file) => {
            const head = document.createElement('div');
            head.className = 'df-file';
            head.textContent = file.path;
            box.appendChild(head);
            file.lines.forEach((l) => {
                const div = document.createElement('div');
                div.className = `df-${l.kind}`;
                div.textContent = l.text;
                box.appendChild(div);
            });
        });
    }

    function renderBranches(branches) {
        const list = $('git-branches');
        list.innerHTML = '';
        selBranch = null;
        if (!branches || branches.length === 0) return;
        branches.forEach((b) => {
            const item = document.createElement('div');
            item.className = 'git-item' + (b.current ? ' current' : '');
            item.textContent = b.name + (b.current ? ' *' : '');
            item.addEventListener('click', () => {
                selBranch = b.name;
                list.querySelectorAll('.git-item').forEach((el) => el.classList.remove('current'));
                item.classList.add('current');
            });
            list.appendChild(item);
        });
    }

    function renderTags(tags) {
        const list = $('git-tags');
        list.innerHTML = '';
        selTag = null;
        if (!tags || tags.length === 0) return;
        tags.forEach((t) => {
            const item = document.createElement('div');
            item.className = 'git-item';
            item.textContent = `${t.name} · ${t.hash} · ${t.subject} · ${t.dateRel}`;
            item.addEventListener('click', () => {
                selTag = t.name;
                list.querySelectorAll('.git-item').forEach((el) => el.classList.remove('current'));
                item.classList.add('current');
            });
            list.appendChild(item);
        });
    }

    async function refresh() {
        const i = ctx.selectedIndex();
        if (i < 0) return;
        const status = await api.getGitStatus(i);
        render(status);
        if (status.isRepo) {
            renderDiff(await api.getGitDiff(i));
            renderBranches(await api.gitBranches(i));
            renderTags(await api.gitTags(i));
        }
    }

    function setButtons(enabled) {
        GIT_ACTIONS.forEach((a) => { $(`git-${a.toLowerCase()}`).disabled = !enabled; });
        $('git-refresh').disabled = !enabled;
    }

    GIT_ACTIONS.forEach((action) => {
        $(`git-${action.toLowerCase()}`).addEventListener('click', () => {
            busy[ctx.selectedIndex()] = true;
            showResult(`Running: git ${action.toLowerCase()}…`, 'info');
            setButtons(false);
            api.gitAction(ctx.selectedIndex(), action);
        });
    });
    $('git-refresh').addEventListener('click', refresh);

    // ---- Issue #63: acciones de ramas y tags ----
    $('git-create-branch').addEventListener('click', async () => {
        const name = $('git-branch-name').value.trim();
        if (!name) return showResult('Especificá un nombre de rama.', 'err');
        const err = await api.gitCreateBranch(ctx.selectedIndex(), name);
        if (err) showResult(`Error: ${err}.`, 'err');
        else { showResult(`Rama '${name}' creada.`, 'ok'); refresh(); }
    });
    $('git-checkout-branch').addEventListener('click', async () => {
        const name = selBranch || $('git-branch-name').value.trim();
        if (!name) return showResult('Seleccioná una rama o escribí un nombre.', 'err');
        const err = await api.gitCheckout(ctx.selectedIndex(), name);
        if (err) showResult(`Error: ${err}.`, 'err');
        else { showResult(`Checkout a '${name}'.`, 'ok'); refresh(); }
    });
    $('git-rename-branch').addEventListener('click', async () => {
        const name = $('git-branch-name').value.trim();
        if (!selBranch || !name) return showResult('Seleccioná una rama y un nuevo nombre.', 'err');
        const err = await api.gitRenameBranch(ctx.selectedIndex(), selBranch, name);
        if (err) showResult(`Error: ${err}.`, 'err');
        else { showResult(`Rama '${selBranch}' → '${name}'.`, 'ok'); refresh(); }
    });
    $('git-delete-branch').addEventListener('click', async () => {
        if (!selBranch) return showResult('Seleccioná una rama.', 'err');
        const err = await api.gitDeleteBranch(ctx.selectedIndex(), selBranch);
        if (err) showResult(`Error: ${err}.`, 'err');
        else { showResult(`Rama '${selBranch}' borrada.`, 'ok'); refresh(); }
    });

    $('git-create-tag').addEventListener('click', async () => {
        const name = $('git-branch-name').value.trim();
        if (!name) return showResult('Especificá un nombre de tag en el campo de arriba.', 'err');
        const err = await api.gitCreateTag(ctx.selectedIndex(), name);
        if (err) showResult(`Error: ${err}.`, 'err');
        else { showResult(`Tag '${name}' creado.`, 'ok'); refresh(); }
    });
    $('git-push-tag').addEventListener('click', async () => {
        const name = selTag || $('git-branch-name').value.trim();
        if (!name) return showResult('Seleccioná un tag.', 'err');
        const err = await api.gitPushTag(ctx.selectedIndex(), name);
        if (err) showResult(`Error: ${err}.`, 'err');
        else { showResult(`Tag '${name}' pusheado.`, 'ok'); }
    });
    $('git-delete-tag').addEventListener('click', async () => {
        if (!selTag) return showResult('Seleccioná un tag.', 'err');
        const err = await api.gitDeleteTag(ctx.selectedIndex(), selTag);
        if (err) showResult(`Error: ${err}.`, 'err');
        else { showResult(`Tag '${selTag}' borrado.`, 'ok'); refresh(); }
    });

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
                showResult('Nothing to stash — working tree clean.', 'info');
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
