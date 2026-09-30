// Panel de herramientas de desarrollo (Issue #68): file browser del proyecto,
// editor simple, búsqueda por nombre/contenido, snippets persistentes y
// paleta de comandos (quick commands) con ejecución vía RunScript custom.
//
// El markup vive en index.html; este módulo cablea eventos y render. Los
// bindings del backend se resuelven vía ctx.api (api.js) con fallback directo
// a window.go.main.App para no acoplar el orden de imports.

const $tools = (id) => document.getElementById(id);

// Estilos propios con prefijo .tools-/.palette-: se inyectan desde el panel
// para no tocar los CSS de temas (theme.css/dracula.css/...) que están en
// edición por #67.
const STYLE_ID = 'tools-styles';
const STYLES = `
#panel-tools { display: flex; flex-direction: column; gap: 8px; overflow-y: auto; }
#tools-toolbar { display: flex; align-items: center; gap: 8px; }
#tools-toolbar .grow { flex: 1; }
#tools-toolbar .text-input { width: 100%; }
#tools-split { display: grid; grid-template-columns: minmax(220px, 34%) 1fr; gap: 8px; min-height: 260px; flex: 1; }
#tools-browser { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
#tools-editor-pane { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
#tools-editor { flex: 1; min-height: 220px; resize: vertical; font-size: 12.5px; line-height: 1.45; white-space: pre; overflow: auto; background: var(--bg-input, rgba(128,128,128,.08)); border: 1px solid var(--border, #444); border-radius: 6px; color: var(--fg, inherit); padding: 8px; }
.tools-crumbs { display: flex; flex-wrap: wrap; gap: 2px; }
.tools-crumb { background: none; border: none; color: var(--accent, #6366f1); cursor: pointer; padding: 2px 4px; font-size: 12.5px; }
.tools-crumb:hover { text-decoration: underline; }
.tools-list { display: flex; flex-direction: column; gap: 2px; overflow-y: auto; max-height: 320px; }
.tools-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 4px 6px; border-radius: 6px; cursor: pointer; }
.tools-row:hover { background: rgba(128,128,128,.15); }
.tools-row-name { flex-shrink: 0; }
.tools-row-cmd { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
#tools-palette-overlay { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: flex; align-items: flex-start; justify-content: center; padding-top: 12vh; z-index: 1000; }
.palette-box { width: min(520px, 92vw); background: var(--bg-panel, #1a1d29); border: 1px solid var(--border, #444); border-radius: 10px; padding: 10px; display: flex; flex-direction: column; gap: 8px; box-shadow: 0 12px 40px rgba(0,0,0,.4); }
.palette-list { display: flex; flex-direction: column; gap: 2px; max-height: 320px; overflow-y: auto; }
.palette-item { text-align: left; background: none; border: none; color: var(--fg, inherit); padding: 7px 10px; border-radius: 6px; cursor: pointer; font-size: 13px; }
.palette-item:hover { background: rgba(128,128,128,.2); }
.notify-row { flex-direction: row; align-items: center; justify-content: space-between; gap: 8px; }
.notify-form { display: flex; flex-direction: column; gap: 6px; border: 1px solid var(--border, #444); border-radius: 8px; padding: 8px; margin-top: 6px; }
.notify-history { display: flex; flex-direction: column; gap: 2px; max-height: 220px; overflow-y: auto; margin-top: 6px; }
.notify-history-row { display: grid; grid-template-columns: auto auto auto 1fr; gap: 8px; align-items: baseline; font-size: 12px; padding: 3px 0; border-top: 1px solid rgba(128,128,128,.15); }
.mini-badge.ok { background: #1f8a4c; } .mini-badge.warn { background: #b8860b; } .mini-badge.err { background: #a32020; }
`;

function ensureStyles() {
    if (typeof document === 'undefined' || document.getElementById(STYLE_ID)) return;
    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = STYLES;
    document.head.appendChild(style);
}


function esc(s) {
    return String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// QUICK_COMMANDS alimenta la paleta: se combinan con los snippets guardados.
const QUICK_COMMANDS = [
    { label: 'Run tests (Playwright)', run: (ctx) => ctx.api.runTests(ctx.index) },
    { label: 'Open terminal', run: (ctx) => ctx.api.openTerminal(ctx.index) },
    { label: 'Open in VS Code', run: (ctx) => ctx.api.openVSCode(ctx.index) },
    { label: 'Open project folder', run: (ctx) => ctx.api.openInExplorer(ctx.index) },
    { label: 'Git status', run: (ctx) => ctx.api.gitAction(ctx.index, 'status') },
    { label: 'Git pull', run: (ctx) => ctx.api.gitAction(ctx.index, 'pull') },
    { label: 'Reload projects', run: (ctx) => ctx.api.reloadProjects() },
    { label: 'Show HTML report', run: (ctx) => ctx.api.openHTMLReport(ctx.index) },
];

export function mountTools(ctx) {
    ensureStyles();
    let index = -1;
    let cwd = '.';          // carpeta actual del browser (rel al proyecto)
    let openFile = null;    // { rel, dirty, original }
    let searchMode = false;
    let snippets = [];

    // Paridad del resto de paneles: el índice activo vive en ctx.selectedIndex().
    const selected = () => (ctx && typeof ctx.selectedIndex === 'function' ? ctx.selectedIndex() : -1);

    const api2 = () => window.go.main.App;

    async function refreshSnippets() {
        try {
            snippets = (await api2().DevGetSnippets()) || [];
        } catch {
            snippets = [];
        }
        renderSnippets();
    }

    function renderSnippets() {
        const list = $tools('tools-snippets');
        const empty = $tools('tools-snippets-empty');
        if (!list) return;
        list.innerHTML = '';
        snippets.forEach((sn) => {
            const row = document.createElement('div');
            row.className = 'tools-row';
            row.innerHTML = `<span class="tools-row-name strong">${esc(sn.name)}</span>` +
                `<span class="tools-row-cmd mono dim">${esc(sn.command)}</span>`;
            const actions = document.createElement('span');
            actions.className = 'row-gap';
            const runBtn = document.createElement('button');
            runBtn.className = 'btn btn-small btn-success';
            runBtn.textContent = 'Run';
            runBtn.title = 'Run as custom script';
            runBtn.addEventListener('click', async () => {
                const i = selected();
                if (i < 0) return;
                const errs = await ctx.api.runScript(i, sn.name, sn.command);
                showResult(errs);
            });
            const delBtn = document.createElement('button');
            delBtn.className = 'btn btn-small btn-danger';
            delBtn.textContent = 'Delete';
            delBtn.addEventListener('click', async () => {
                const errs = await api2().DevDeleteSnippet(sn.id);
                showResult(errs);
                refreshSnippets();
            });
            actions.appendChild(runBtn);
            actions.appendChild(delBtn);
            row.appendChild(actions);
            list.appendChild(row);
        });
        if (empty) empty.hidden = snippets.length > 0;
    }

    function showResult(errs) {
        const strip = $tools('tools-result');
        if (!strip) return;
        if (errs && errs.length) {
            strip.hidden = false;
            strip.className = 'result-strip err';
            strip.textContent = errs.join('\n');
        } else {
            strip.hidden = true;
        }
    }

    async function listDir(rel) {
        if (index < 0) return;
        try {
            const entries = await api2().DevListDir(index, rel);
            cwd = rel || '.';
            searchMode = false;
            renderCrumbs();
            renderList(entries || []);
        } catch (err) {
            showResult([String(err)]);
        }
    }

    function renderCrumbs() {
        const crumbs = $tools('tools-crumbs');
        if (!crumbs) return;
        crumbs.innerHTML = '';
        const parts = cwd === '.' ? [] : cwd.split('/');
        const mk = (label, rel) => {
            const b = document.createElement('button');
            b.className = 'tools-crumb';
            b.textContent = label;
            b.addEventListener('click', () => listDir(rel));
            return b;
        };
        crumbs.appendChild(mk('project root', '.'));
        let acc = '';
        parts.forEach((p) => {
            acc = acc ? `${acc}/${p}` : p;
            crumbs.appendChild(mk(p, acc));
        });
    }

    function renderList(entries) {
        const list = $tools('tools-list');
        const searchList = $tools('tools-search-results');
        if (!list) return;
        if (searchList) searchList.hidden = true;
        list.hidden = false;
        list.innerHTML = '';
        entries.forEach((e) => {
            const row = document.createElement('div');
            row.className = 'tools-row';
            row.innerHTML = `<span class="tools-row-name">${e.isDir ? '📁' : '📄'} ${esc(e.name)}</span>` +
                (e.isDir ? '' : `<span class="dim small">${e.size} B</span>`);
            row.addEventListener('click', () => {
                if (e.isDir) listDir(e.rel);
                else openInEditor(e.rel);
            });
            list.appendChild(row);
        });
        if (!entries.length) {
            list.innerHTML = '<div class="dim center padded">Empty folder</div>';
        }
    }

    async function runSearch(q) {
        if (index < 0 || !q || q.length < 2) return;
        try {
            const results = await api2().DevSearchFiles(index, q);
            searchMode = true;
            const list = $tools('tools-list');
            const searchList = $tools('tools-search-results');
            if (list) list.hidden = true;
            if (!searchList) return;
            searchList.hidden = false;
            searchList.innerHTML = '';
            (results || []).forEach((r) => {
                const row = document.createElement('div');
                row.className = 'tools-row';
                const where = r.line > 0 ? `:${r.line}` : '';
                row.innerHTML = `<span class="tools-row-name mono">${esc(r.path)}${where}</span>` +
                    `<span class="tools-row-cmd dim small">${esc(r.preview)}</span>`;
                row.addEventListener('click', () => openInEditor(r.path));
                searchList.appendChild(row);
            });
            if (!results || !results.length) {
                searchList.innerHTML = '<div class="dim center padded">No matches</div>';
            }
        } catch (err) {
            showResult([String(err)]);
        }
    }

    async function openInEditor(rel) {
        try {
            const content = await api2().DevReadFile(index, rel);
            openFile = { rel, original: content, dirty: false };
            const editor = $tools('tools-editor');
            const empty = $tools('tools-editor-empty');
            const path = $tools('tools-editor-path');
            if (!editor) return;
            editor.hidden = false;
            editor.value = content;
            if (empty) empty.hidden = true;
            if (path) path.textContent = rel;
            updateEditorButtons();
        } catch (err) {
            showResult([String(err)]);
        }
    }

    function updateEditorButtons() {
        const save = $tools('tools-save');
        const close = $tools('tools-close');
        if (save) save.disabled = !openFile || !openFile.dirty;
        if (close) close.disabled = !openFile;
        const path = $tools('tools-editor-path');
        if (path && openFile) path.textContent = openFile.rel + (openFile.dirty ? ' *' : '');
    }

    function closeEditor() {
        openFile = null;
        const editor = $tools('tools-editor');
        const empty = $tools('tools-editor-empty');
        if (editor) { editor.hidden = true; editor.value = ''; }
        if (empty) empty.hidden = false;
        updateEditorButtons();
    }

    // ---- Command palette (Ctrl+K) ----

    function openPalette() {
        let overlay = document.getElementById('tools-palette-overlay');
        if (overlay) { overlay.remove(); }
        overlay = document.createElement('div');
        overlay.id = 'tools-palette-overlay';
        overlay.innerHTML = `<div class="palette-box" role="dialog" aria-label="Command palette">` +
            `<input id="palette-input" class="text-input" placeholder="Type a command..." aria-label="Command">` +
            `<div id="palette-list" class="palette-list"></div></div>`;
        overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
        document.body.appendChild(overlay);
        const input = overlay.querySelector('#palette-input');
        const list = overlay.querySelector('#palette-list');

        const commands = [...QUICK_COMMANDS.map((c) => ({ ...c }))];
        snippets.forEach((sn) => commands.push({
            label: `Snippet: ${sn.name}`,
            run: (c) => c.api.runScript(c.index, sn.name, sn.command),
        }));

        const renderPalette = (q) => {
            const filtered = commands.filter((c) => c.label.toLowerCase().includes(q.toLowerCase()));
            list.innerHTML = '';
            filtered.slice(0, 12).forEach((c) => {
                const item = document.createElement('button');
                item.className = 'palette-item';
                item.textContent = c.label;
                item.addEventListener('click', async () => {
                    overlay.remove();
                    const i = selected();
                    if (i < 0) return;
                    try { await c.run({ api: ctx.api, index: i }); } catch (e) { showResult([String(e)]); }
                });
                list.appendChild(item);
            });
            if (!filtered.length) {
                list.innerHTML = '<div class="dim center padded">No matching commands</div>';
            }
        };
        renderPalette('');
        input.addEventListener('input', () => renderPalette(input.value));
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') overlay.remove();
            if (e.key === 'Enter') {
                const first = list.querySelector('.palette-item');
                if (first) first.click();
            }
        });
        input.focus();
    }

    function wire() {
        const search = $tools('tools-search');
        if (search && !search.hasAttribute('data-listener-added')) {
            let debounce = null;
            search.addEventListener('input', () => {
                clearTimeout(debounce);
                debounce = setTimeout(() => {
                    const q = search.value.trim();
                    if (q.length >= 2) runSearch(q);
                    else if (searchMode) listDir(cwd);
                }, 250);
            });
            search.setAttribute('data-listener-added', 'true');
        }
        const save = $tools('tools-save');
        if (save && !save.hasAttribute('data-listener-added')) {
            save.addEventListener('click', async () => {
                if (!openFile) return;
                const editor = $tools('tools-editor');
                const errs = await api2().DevWriteFile(index, openFile.rel, editor.value);
                showResult(errs);
                if (!errs || !errs.length) {
                    openFile.original = editor.value;
                    openFile.dirty = false;
                    updateEditorButtons();
                }
            });
            save.setAttribute('data-listener-added', 'true');
        }
        const close = $tools('tools-close');
        if (close && !close.hasAttribute('data-listener-added')) {
            close.addEventListener('click', closeEditor);
            close.setAttribute('data-listener-added', 'true');
        }
        const editor = $tools('tools-editor');
        if (editor && !editor.hasAttribute('data-listener-added')) {
            editor.addEventListener('input', () => {
                if (!openFile) return;
                openFile.dirty = editor.value !== openFile.original;
                updateEditorButtons();
            });
            editor.setAttribute('data-listener-added', 'true');
        }
        const addSnippet = $tools('tools-snippet-add');
        if (addSnippet && !addSnippet.hasAttribute('data-listener-added')) {
            addSnippet.addEventListener('click', async () => {
                const name = $tools('tools-snippet-name').value.trim();
                const cmd = $tools('tools-snippet-cmd').value.trim();
                if (!name || !cmd) { showResult(['Snippet name and command are required']); return; }
                const errs = await api2().DevSaveSnippet({ id: '', name, command: cmd });
                showResult(errs);
                if (!errs || !errs.length) {
                    $tools('tools-snippet-name').value = '';
                    $tools('tools-snippet-cmd').value = '';
                    refreshSnippets();
                }
            });
            addSnippet.setAttribute('data-listener-added', 'true');
        }
        const paletteBtn = $tools('tools-palette');
        if (paletteBtn && !paletteBtn.hasAttribute('data-listener-added')) {
            paletteBtn.addEventListener('click', openPalette);
            paletteBtn.setAttribute('data-listener-added', 'true');
        }
        document.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
                e.preventDefault();
                openPalette();
            }
        });
    }

    wire();
    refreshSnippets();

    return {
        onProjectChanged(p) {
            const prev = index;
            index = selected();
            cwd = '.';
            searchMode = false;
            const search = $tools('tools-search');
            if (search) search.value = '';
            closeEditor();
            if (index !== prev) listDir('.');
            refreshSnippets();
        },
    };
}
