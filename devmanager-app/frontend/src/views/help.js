// Ventana de ayuda y documentación (Issue #72): documentación navegable,
// búsqueda full-text, tutoriales con progreso, FAQ, atajos, changelog y ayuda
// contextual por elemento de la UI.
//
// Integración (pendiente, a propósito): este módulo NO edita index.html ni
// main.js para no colisionar con #67. Para activarlo:
//   1. index.html: <section id="help-view" class="view" hidden></section>
//   2. main.js: import { mountHelpView } from './views/help.js';
//      const helpView = mountHelpView(ctx); y abrir con helpView.open()
//      (por ejemplo desde un botón del header o con switchView('help')).
// El teclado (F1 / Ctrl+?) se registra aquí mismo en cuanto el contenedor existe.

import {
    changelogRows, contextForTopic, faqGroups, filterFAQ, filterTopics,
    groupTopicsBySection, hitKind, matchesQuery, nextStepIndex, parseBody,
    progressSummary, shortcutRows, tutorialProgress,
} from '../help.js';

const VIEW_ID = 'help-view';
const STYLE_ID = 'help-styles';

const STYLES = `
.hp-wrap { display: flex; flex-direction: column; height: 100%; gap: 8px; padding: 10px; overflow: hidden; }
.hp-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.hp-header h2 { margin: 0; flex: 1; }
.hp-search { min-width: 220px; flex: 1; }
.hp-tabs { display: flex; gap: 4px; flex-wrap: wrap; border-bottom: 1px solid rgba(128,128,128,.3); padding-bottom: 6px; }
.hp-tab { background: transparent; border: 1px solid transparent; border-radius: 6px; padding: 4px 10px; cursor: pointer; }
.hp-tab.active { border-color: currentColor; font-weight: 600; }
.hp-body { display: flex; gap: 12px; min-height: 0; flex: 1; }
.hp-nav { width: 230px; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; }
.hp-nav-section { font-size: 11px; text-transform: uppercase; opacity: .7; margin-top: 6px; }
.hp-nav-topic { background: transparent; border: 0; text-align: left; padding: 3px 6px; border-radius: 4px; cursor: pointer; }
.hp-nav-topic:hover, .hp-nav-topic.active { background: rgba(128,128,128,.18); }
.hp-content { flex: 1; overflow-y: auto; min-height: 0; }
.hp-block { margin: 4px 0; }
.hp-block.heading { font-weight: 600; margin-top: 10px; }
.hp-block.bullet { padding-left: 14px; position: relative; }
.hp-block.bullet::before { content: '•'; position: absolute; left: 2px; }
.hp-card { border: 1px solid rgba(128,128,128,.3); border-radius: 6px; padding: 8px; margin: 6px 0; }
.hp-progress { height: 6px; background: rgba(128,128,128,.25); border-radius: 3px; overflow: hidden; margin: 4px 0; }
.hp-progress > div { height: 100%; background: #1f8a4c; }
.hp-step { border-left: 2px solid rgba(128,128,128,.4); padding-left: 8px; margin: 6px 0; }
.hp-step.current { border-color: #1f8a4c; }
.hp-step.done { opacity: .6; }
.hp-keys { font-family: monospace; border: 1px solid rgba(128,128,128,.4); border-radius: 4px; padding: 1px 6px; }
.hp-row { display: flex; gap: 8px; align-items: baseline; padding: 3px 0; }
.hp-row .hp-action { flex: 1; }
.hp-actions { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 8px; }
.hp-faq-q { width: 100%; text-align: left; background: transparent; border: 0; cursor: pointer; padding: 4px 0; font-weight: 600; }
.hp-faq-a { padding: 0 0 6px 12px; }
.hp-dim { opacity: .7; font-size: 12px; }
.hp-flash { outline: 2px solid #1f8a4c; outline-offset: 2px; }
.hp-release { border-bottom: 1px solid rgba(128,128,128,.2); padding-bottom: 6px; margin-bottom: 6px; }
`;

function ensureStyles() {
    if (typeof document === 'undefined' || document.getElementById(STYLE_ID)) return;
    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = STYLES;
    document.head.appendChild(style);
}

function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

function clear(node) {
    while (node.firstChild) node.removeChild(node.firstChild);
}

export function createBindings(getApp) {
    return {
        getHelpIndex: () => getApp().GetHelpIndex(),
        searchHelp: (query, limit) => getApp().SearchHelp(query, limit),
        getHelpTopic: (id) => getApp().GetHelpTopic(id),
        getChangelog: (limit) => getApp().GetChangelog(limit),
        getHelpShortcuts: () => getApp().GetHelpShortcuts(),
        getContextHelp: (target) => getApp().GetContextHelp(target),
        getHelpProgress: () => getApp().GetHelpProgress(),
        setTutorialStep: (id, step) => getApp().SetTutorialStep(id, step),
        dismissHelpTip: (id) => getApp().DismissHelpTip(id),
        setHelpLastTopic: (id) => getApp().SetHelpLastTopic(id),
        resetHelpProgress: () => getApp().ResetHelpProgress(),
    };
}

export function resolveApi(ctx) {
    const fromCtx = (ctx && ctx.api) || null;
    if (fromCtx && typeof fromCtx.getHelpIndex === 'function') return fromCtx;
    const go = typeof window !== 'undefined' && window.go && window.go.main && window.go.main.App;
    if (!go) return null;
    return createBindings(() => go);
}

const TABS = [
    ['docs', 'Documentation'],
    ['tutorials', 'Tutorials'],
    ['faq', 'FAQ'],
    ['shortcuts', 'Shortcuts'],
    ['changelog', 'Changelog'],
    ['context', "What's this?"],
];

export function mount(ctx) {
    const root = document.getElementById(VIEW_ID);
    if (!root) {
        console.warn(`[help] falta <section id="${VIEW_ID}"> en index.html (ver cabecera del módulo)`);
        return { open() {}, close() {}, isOpen: () => false, refresh() {}, getState() { return {}; } };
    }
    ensureStyles();
    clear(root);

    const api = resolveApi(ctx);
    const state = { index: null, tab: 'docs', topicId: '', query: '', hits: [], tutorialId: '', faqOpen: {}, loading: false };

    const wrap = el('div', 'hp-wrap');
    const header = el('header', 'hp-header');
    const title = el('h2', '', 'Help & documentation');
    const search = el('input', 'text-input hp-search');
    search.type = 'search';
    search.placeholder = 'Search docs and FAQ...';
    search.setAttribute('aria-label', 'Search help');
    search.id = 'help-search';
    const progressLabel = el('span', 'hp-dim', '');
    const closeBtn = el('button', 'btn', 'Close');
    header.append(title, search, progressLabel, closeBtn);

    const tabs = el('div', 'hp-tabs');
    const tabButtons = {};
    for (const [key, label] of TABS) {
        // Ojo: NO usar la clase .tab (main.js la reserva para los paneles de
        // proyecto y switchTab(undefined) rompería la selección de tabs).
        const btn = el('button', `hp-tab${key === state.tab ? ' active' : ''}`, label);
        btn.dataset.helpTab = key;
        btn.setAttribute('role', 'tab');
        btn.addEventListener('click', () => selectTab(key));
        tabButtons[key] = btn;
        tabs.appendChild(btn);
    }

    const body = el('div', 'hp-body');
    const nav = el('div', 'hp-nav');
    const content = el('div', 'hp-content');
    body.append(nav, content);
    wrap.append(header, tabs, body);
    root.appendChild(wrap);

    function setVisible(visible) {
        root.hidden = !visible;
        if (visible) {
            root.classList.add('active');
            search.focus();
        } else {
            root.classList.remove('active');
        }
    }

    function selectTab(key) {
        state.tab = key;
        Object.entries(tabButtons).forEach(([name, btn]) => btn.classList.toggle('active', name === key));
        render();
    }

    function topicByID(id) {
        const topics = (state.index && state.index.topics) || [];
        return topics.find((t) => t.id === id) || null;
    }

    // ---- render piezas ----

    function renderNav() {
        clear(nav);
        const index = state.index || {};
        if (state.query) {
            nav.appendChild(el('div', 'hp-nav-section', `${state.hits.length} result(s)`));
            state.hits.forEach((hit) => {
                const btn = el('button', 'hp-nav-topic', hit.title);
                btn.dataset.hitId = hit.id;
                btn.dataset.hitKind = hitKind(hit);
                btn.addEventListener('click', () => {
                    if (hitKind(hit) === 'faq') {
                        state.faqOpen[hit.id] = true;
                        selectTab('faq');
                    } else {
                        openTopic(hit.id);
                    }
                });
                nav.appendChild(btn);
            });
            return;
        }
        groupTopicsBySection(filterTopics(index.topics, { query: '' }), index.sections).forEach((group) => {
            nav.appendChild(el('div', 'hp-nav-section', group.section));
            group.topics.forEach((topic) => {
                const btn = el('button', `hp-nav-topic${topic.id === state.topicId ? ' active' : ''}`, topic.title);
                btn.dataset.topicId = topic.id;
                btn.addEventListener('click', () => openTopic(topic.id));
                nav.appendChild(btn);
            });
        });
    }

    function renderTopic(id) {
        const topic = topicByID(id);
        if (!topic) {
            content.appendChild(el('div', 'hp-dim', 'Select a topic from the left.'));
            return;
        }
        content.appendChild(el('h3', '', topic.title));
        content.appendChild(el('div', 'hp-dim', `${topic.section} · ${topic.summary}`));
        parseBody(topic.body).forEach((block) => {
            if (block.type === 'heading') {
                content.appendChild(el('div', 'hp-block heading', block.text));
            } else if (block.type === 'bullet') {
                content.appendChild(el('div', 'hp-block bullet', block.text));
            } else {
                content.appendChild(el('p', 'hp-block', block.text));
            }
        });

        const related = (topic.related || []).map(topicByID).filter(Boolean);
        if (related.length) {
            const row = el('div', 'hp-actions');
            related.forEach((r) => {
                const btn = el('button', 'btn', r.title);
                btn.dataset.relatedId = r.id;
                btn.addEventListener('click', () => openTopic(r.id));
                row.appendChild(btn);
            });
            content.appendChild(el('div', 'hp-dim', 'Related topics'));
            content.appendChild(row);
        }

        const context = contextForTopic((state.index || {}).context, topic.id);
        if (context.length) {
            content.appendChild(el('div', 'hp-block heading', 'Where to find it'));
            context.forEach((entry) => {
                const card = el('div', 'hp-card');
                card.appendChild(el('div', '', `${entry.target} — ${entry.text}`));
                if (entry.keys) card.appendChild(el('div', 'hp-dim', `Shortcut: ${entry.keys}`));
                const actions = el('div', 'hp-actions');
                const flash = el('button', 'btn', 'Show me');
                flash.addEventListener('click', () => flashTarget(entry.target));
                actions.appendChild(flash);
                if (entry.keys) {
                    actions.appendChild(el('span', 'hp-keys', entry.keys));
                }
                card.appendChild(actions);
                content.appendChild(card);
            });
        }

        const tutorials = ((state.index || {}).tutorials || []).filter((t) =>
            t.steps.some((s) => s.topicId === topic.id));
        if (tutorials.length) {
            content.appendChild(el('div', 'hp-block heading', 'Tutorials for this topic'));
            tutorials.forEach((t) => {
                const btn = el('button', 'btn btn-accent', t.title);
                btn.dataset.tutorialId = t.id;
                btn.addEventListener('click', () => openTutorial(t.id));
                content.appendChild(btn);
            });
        }
    }

    function renderHits() {
        content.appendChild(el('h3', '', `Results for “${state.query}”`));
        if (!state.hits.length) {
            content.appendChild(el('div', 'hp-dim', 'Nothing found. Try another term.'));
            return;
        }
        state.hits.forEach((hit) => {
            const card = el('div', 'hp-card');
            const btn = el('button', 'hp-faq-q', hit.title);
            btn.addEventListener('click', () => {
                if (hitKind(hit) === 'faq') {
                    state.faqOpen[hit.id] = true;
                    selectTab('faq');
                } else {
                    openTopic(hit.id);
                }
            });
            card.appendChild(btn);
            card.appendChild(el('div', 'hp-dim', `${hit.section}${hit.kind === 'faq' ? ' · FAQ' : ''}`));
            if (hit.snippet) card.appendChild(el('div', '', hit.snippet));
            content.appendChild(card);
        });
    }

    function renderTutorials() {
        const index = state.index || {};
        content.appendChild(el('h3', '', 'Interactive tutorials'));
        content.appendChild(el('div', 'hp-dim', progressSummary(index.summary)));
        const resetBtn = el('button', 'btn btn-warn', 'Reset progress');
        resetBtn.addEventListener('click', async () => {
            if (!api) return;
            await api.resetHelpProgress();
            await refresh();
        });
        content.appendChild(resetBtn);

        (index.tutorials || []).forEach((tutorial) => {
            const info = tutorialProgress(tutorial, (index.progress || {}).tutorials);
            const card = el('div', 'hp-card');
            card.appendChild(el('div', 'strong', tutorial.title));
            card.appendChild(el('div', 'hp-dim', tutorial.summary));
            const bar = el('div', 'hp-progress');
            const fill = el('div');
            fill.style.width = `${info.pct}%`;
            bar.appendChild(fill);
            card.appendChild(bar);
            card.appendChild(el('div', 'hp-dim', `${info.done}/${info.total} steps${info.completed ? ' · completed' : ''}`));

            if (state.tutorialId === tutorial.id) {
                const current = nextStepIndex(tutorial, (index.progress || {}).tutorials);
                tutorial.steps.forEach((step, i) => {
                    const cls = i < current ? 'hp-step done' : i === current ? 'hp-step current' : 'hp-step';
                    const node = el('div', cls);
                    node.dataset.stepIndex = String(i);
                    node.appendChild(el('div', 'strong', `${i + 1}. ${step.title}`));
                    node.appendChild(el('div', '', step.body));
                    if (step.target) node.appendChild(el('div', 'hp-dim', `Target: ${step.target}`));
                    if (step.topicId) {
                        const link = el('button', 'hp-nav-topic', 'Read more');
                        link.addEventListener('click', () => openTopic(step.topicId));
                        node.appendChild(link);
                    }
                    card.appendChild(node);
                });
                const actions = el('div', 'hp-actions');
                const done = el('button', 'btn btn-success', 'Mark step done');
                done.dataset.tutorialDone = tutorial.id;
                done.addEventListener('click', () => markStep(tutorial.id, current));
                const hide = el('button', 'btn', 'Hide steps');
                hide.addEventListener('click', () => {
                    state.tutorialId = '';
                    render();
                });
                actions.append(done, hide);
                card.appendChild(actions);
            } else {
                const start = el('button', 'btn btn-accent', 'Open steps');
                start.dataset.tutorialOpen = tutorial.id;
                start.addEventListener('click', () => openTutorial(tutorial.id));
                card.appendChild(start);
            }
            content.appendChild(card);
        });
    }

    function renderFAQ() {
        const index = state.index || {};
        content.appendChild(el('h3', '', 'Frequently asked questions'));
        const list = filterFAQ(index.faq, state.query);
        if (!list.length) {
            content.appendChild(el('div', 'hp-dim', 'No FAQ matching your search.'));
            return;
        }
        faqGroups(list, index.faqCategories).forEach((group) => {
            content.appendChild(el('div', 'hp-nav-section', group.category));
            group.entries.forEach((entry) => {
                const card = el('div', 'hp-card');
                card.dataset.faqId = entry.id;
                const question = el('button', 'hp-faq-q', entry.question);
                question.addEventListener('click', () => {
                    state.faqOpen[entry.id] = !state.faqOpen[entry.id];
                    render();
                });
                card.appendChild(question);
                if (state.faqOpen[entry.id]) {
                    card.appendChild(el('div', 'hp-faq-a', entry.answer));
                    if (entry.topicId) {
                        const link = el('button', 'hp-nav-topic', 'Open topic');
                        link.addEventListener('click', () => openTopic(entry.topicId));
                        card.appendChild(link);
                    }
                }
                content.appendChild(card);
            });
        });
    }

    function renderShortcuts() {
        const index = state.index || {};
        content.appendChild(el('h3', '', 'Keyboard shortcuts'));
        shortcutRows(index.shortcuts).forEach((group) => {
            content.appendChild(el('div', 'hp-nav-section', group.group));
            group.rows.forEach((row) => {
                const line = el('div', 'hp-row');
                line.appendChild(el('span', 'hp-keys', row.keys));
                line.appendChild(el('span', 'hp-action', row.action));
                line.appendChild(el('span', 'hp-dim', row.context));
                content.appendChild(line);
            });
        });
    }

    function renderChangelog() {
        const rows = changelogRows((state.index || {}).changelog);
        content.appendChild(el('h3', '', 'Changelog'));
        if (!rows.length) {
            content.appendChild(el('div', 'hp-dim', 'No CHANGELOG.md found next to the app.'));
            return;
        }
        rows.forEach((release) => {
            const card = el('div', 'hp-release');
            card.appendChild(el('div', 'strong', `${release.version}${release.date ? ` — ${release.date}` : ''}`));
            release.sections.forEach((section) => {
                card.appendChild(el('div', 'hp-dim', section.title));
                section.items.forEach((item) => {
                    card.appendChild(el('div', 'hp-block bullet', item));
                });
            });
            content.appendChild(card);
        });
    }

    function renderContext() {
        const index = state.index || {};
        content.appendChild(el('h3', '', "What's this?"));
        content.appendChild(el('div', 'hp-dim', 'Click a button in the app and find it here, or search above.'));
        const entries = (index.context || []).filter((entry) =>
            !state.query || matchesQuery(`${entry.target} ${entry.text} ${entry.title}`, state.query));
        if (!entries.length) {
            content.appendChild(el('div', 'hp-dim', 'No matching UI element.'));
            return;
        }
        const byTopic = new Map();
        entries.forEach((entry) => {
            const list = byTopic.get(entry.topicId) || [];
            list.push(entry);
            byTopic.set(entry.topicId, list);
        });
        for (const [topicId, list] of byTopic) {
            const topic = topicByID(topicId);
            content.appendChild(el('div', 'hp-nav-section', topic ? topic.title : topicId));
            list.forEach((entry) => {
                const card = el('div', 'hp-card');
                card.appendChild(el('div', 'mono', entry.target));
                card.appendChild(el('div', '', entry.text));
                const actions = el('div', 'hp-actions');
                const flash = el('button', 'btn', 'Show me');
                flash.addEventListener('click', () => flashTarget(entry.target));
                actions.appendChild(flash);
                if (topic) {
                    const link = el('button', 'btn', 'Open topic');
                    link.addEventListener('click', () => openTopic(topic.id));
                    actions.appendChild(link);
                }
                card.appendChild(actions);
                content.appendChild(card);
            });
        }
    }

    function render() {
        clear(content);
        if (state.query) {
            renderHits();
        } else if (state.tab === 'tutorials') {
            renderTutorials();
        } else if (state.tab === 'faq') {
            renderFAQ();
        } else if (state.tab === 'shortcuts') {
            renderShortcuts();
        } else if (state.tab === 'changelog') {
            renderChangelog();
        } else if (state.tab === 'context') {
            renderContext();
        } else {
            renderTopic(state.topicId);
        }
        renderNav();
        const index = state.index || {};
        progressLabel.textContent = progressSummary(index.summary);
    }

    // ---- acciones ----

    function flashTarget(selector) {
        if (typeof document === 'undefined' || !selector) return;
        let node = null;
        try {
            node = document.querySelector(selector.startsWith('.') || selector.startsWith('#') ? selector : `#${selector}`);
        } catch (err) {
            node = null;
        }
        if (!node) return;
        node.classList.add('hp-flash');
        setTimeout(() => node.classList.remove('hp-flash'), 1600);
    }

    function openTopic(id) {
        state.topicId = id;
        state.tab = 'docs';
        state.query = '';
        search.value = '';
        Object.entries(tabButtons).forEach(([name, btn]) => btn.classList.toggle('active', name === 'docs'));
        render();
        if (api && api.setHelpLastTopic) api.setHelpLastTopic(id).catch(() => {});
    }

    function openTutorial(id) {
        state.tutorialId = id;
        state.query = '';
        search.value = '';
        state.tab = 'tutorials';
        Object.entries(tabButtons).forEach(([name, btn]) => btn.classList.toggle('active', name === 'tutorials'));
        render();
    }

    async function markStep(tutorialId, step) {
        if (!api) return;
        await api.setTutorialStep(tutorialId, step);
        await refresh();
    }

    async function runSearch(query) {
        state.query = String(query || '').trim();
        if (!state.query) {
            state.hits = [];
            render();
            return;
        }
        if (!api) {
            // Sin backend: búsqueda local sobre el índice ya cargado.
            const index = state.index || {};
            const needle = state.query;
            state.hits = [
                ...filterTopics(index.topics, { query: needle }).map((t) => ({
                    kind: 'topic', id: t.id, title: t.title, section: t.section, snippet: t.summary,
                })),
                ...filterFAQ(index.faq, needle).map((f) => ({
                    kind: 'faq', id: f.id, title: f.question, section: f.category, snippet: f.answer,
                })),
            ];
            render();
            return;
        }
        try {
            state.hits = await api.searchHelp(state.query, 25);
        } catch (err) {
            state.hits = [];
        }
        render();
    }

    async function refresh() {
        if (!api) {
            render();
            return;
        }
        if (state.loading) return;
        state.loading = true;
        try {
            state.index = await api.getHelpIndex();
            const lastTopic = (state.index.progress || {}).lastTopic;
            if (!state.topicId && lastTopic && topicByID(lastTopic)) state.topicId = lastTopic;
        } catch (err) {
            title.textContent = `Help unavailable: ${(err && err.message) || err}`;
        } finally {
            state.loading = false;
            render();
        }
    }

    // ---- eventos ----
    let searchTimer = null;
    search.addEventListener('input', () => {
        clearTimeout(searchTimer);
        searchTimer = setTimeout(() => runSearch(search.value), 120);
    });
    closeBtn.addEventListener('click', () => setVisible(false));

    function onKeydown(e) {
        if (root.hidden) return;
        if (e.key === 'Escape') {
            setVisible(false);
        }
    }
    window.addEventListener('keydown', onKeydown);

    render();
    setVisible(false);

    return {
        open(topicId) {
            setVisible(true);
            if (topicId) {
                openTopic(topicId);
                return refresh();
            }
            return refresh();
        },
        close() {
            setVisible(false);
        },
        isOpen() {
            return !root.hidden;
        },
        selectTab,
        openTopic,
        openTutorial,
        runSearch,
        refresh,
        getState() {
            return { ...state };
        },
        destroy() {
            window.removeEventListener('keydown', onKeydown);
        },
    };
}
