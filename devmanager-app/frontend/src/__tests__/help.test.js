import { describe, it, expect, vi, afterEach } from 'vitest';
import {
    changelogRows, contextForTopic, contextLabel, faqGroups, filterFAQ, filterTopics,
    groupTopicsBySection, hitKind, matchesQuery, nextStepIndex, parseBody,
    progressSummary, shortcutRows, tutorialProgress,
} from '../help.js';
import { mount } from '../views/help.js';

describe('help parseBody', () => {
    it('separa headings, viñetas y párrafos', () => {
        const blocks = parseBody('Intro corta.\n\n## Sección\n- uno\n- dos\n\nCierre\ncon dos líneas');
        expect(blocks).toEqual([
            { type: 'paragraph', text: 'Intro corta.' },
            { type: 'heading', text: 'Sección' },
            { type: 'bullet', text: 'uno' },
            { type: 'bullet', text: 'dos' },
            { type: 'paragraph', text: 'Cierre con dos líneas' },
        ]);
    });

    it('cuerpo vacío o nulo', () => {
        expect(parseBody('')).toEqual([]);
        expect(parseBody(null)).toEqual([]);
        expect(parseBody('   \n  \n')).toEqual([]);
    });

    it('acepta asteriscos como viñeta', () => {
        expect(parseBody('* item')).toEqual([{ type: 'bullet', text: 'item' }]);
    });
});

describe('help filtrado y agrupado', () => {
    const topics = [
        { id: 'a', title: 'Alpha', section: 'Getting started', summary: 'arrancar', keywords: ['start'] },
        { id: 'b', title: 'Beta', section: 'Tools', summary: 'herramientas', keywords: ['k6'] },
        { id: 'c', title: 'Gamma', section: 'Tools', summary: 'más', keywords: [] },
    ];

    it('groupTopicsBySection respeta el orden del backend', () => {
        const groups = groupTopicsBySection(topics, ['Tools', 'Getting started']);
        expect(groups.map((g) => g.section)).toEqual(['Tools', 'Getting started']);
        expect(groups[0].topics.map((t) => t.title)).toEqual(['Beta', 'Gamma']);
        expect(groupTopicsBySection([], [])).toEqual([]);
    });

    it('filterTopics filtra por sección y texto', () => {
        expect(filterTopics(topics, { section: 'Tools' }).map((t) => t.id)).toEqual(['b', 'c']);
        expect(filterTopics(topics, { query: 'start' }).map((t) => t.id)).toEqual(['a']);
        expect(filterTopics(topics, { query: 'K6' }).map((t) => t.id)).toEqual(['b']);
        expect(filterTopics(topics, {})).toHaveLength(3);
        expect(filterTopics(topics, { query: 'nada' })).toEqual([]);
    });

    it('matchesQuery ignora mayúsculas y vacíos', () => {
        expect(matchesQuery('Playwright', 'play')).toBe(true);
        expect(matchesQuery('Playwright', '')).toBe(true);
        expect(matchesQuery(null, 'x')).toBe(false);
    });

    it('hitKind distingue FAQ de documentación', () => {
        expect(hitKind({ kind: 'faq' })).toBe('faq');
        expect(hitKind({ kind: 'topic' })).toBe('topic');
        expect(hitKind(null)).toBe('topic');
    });
});

describe('help shortcuts y FAQ', () => {
    it('shortcutRows agrupa sin perder filas', () => {
        const groups = shortcutRows([
            { group: 'Projects', keys: 'Ctrl+N', action: 'Add', context: 'Global' },
            { group: 'Projects', keys: 'Ctrl+E', action: 'Edit', context: 'Global' },
            { group: 'Servers', keys: 'F5', action: 'Start', context: 'Global' },
        ]);
        expect(groups.map((g) => g.group)).toEqual(['Projects', 'Servers']);
        expect(groups[0].rows).toHaveLength(2);
        expect(groups[1].rows[0].keys).toBe('F5');
    });

    it('faqGroups y filterFAQ', () => {
        const faq = [
            { id: '1', category: 'Servers', question: 'Port?', answer: 'Use monitor' },
            { id: '2', category: 'Testing', question: 'Flaky?', answer: 'Coverage panel' },
        ];
        const groups = faqGroups(faq, ['Servers']);
        expect(groups.map((g) => g.category)).toEqual(['Servers', 'Testing']);
        expect(filterFAQ(faq, 'monitor').map((f) => f.id)).toEqual(['1']);
        expect(filterFAQ(faq, '')).toHaveLength(2);
    });
});

describe('help tutoriales', () => {
    const tutorial = { id: 't', steps: [{ title: 'a' }, { title: 'b' }, { title: 'c' }, { title: 'd' }] };

    it('tutorialProgress calcula avance', () => {
        expect(tutorialProgress(tutorial, {})).toMatchObject({ step: -1, done: 0, pct: 0, completed: false });
        expect(tutorialProgress(tutorial, { t: { step: 0 } })).toMatchObject({ done: 1, pct: 25 });
        expect(tutorialProgress(tutorial, { t: { step: 3 } })).toMatchObject({ done: 4, pct: 100, completed: true });
    });

    it('nextStepIndex acota al rango', () => {
        expect(nextStepIndex(tutorial, {})).toBe(0);
        expect(nextStepIndex(tutorial, { t: { step: 1 } })).toBe(2);
        expect(nextStepIndex(tutorial, { t: { step: 99 } })).toBe(4);
    });

    it('progressSummary', () => {
        expect(progressSummary(null)).toBe('No tutorials available');
        expect(progressSummary({ completed: 1, total: 4, pct: 25 })).toBe('1/4 tutorials completed (25%)');
    });
});

describe('help changelog y contexto', () => {
    it('changelogRows normaliza releases', () => {
        const rows = changelogRows([
            { version: '2.1.0', date: '2026-09-07', sections: [{ title: 'Features', items: ['x'] }] },
            { sections: [] },
        ]);
        expect(rows[0].version).toBe('2.1.0');
        expect(rows[0].sections[0].items).toEqual(['x']);
        expect(rows[1].version).toBe('Unreleased');
        expect(rows[1].date).toBe('');
        expect(changelogRows(null)).toEqual([]);
    });

    it('contextForTopic y contextLabel', () => {
        const context = [
            { target: 'btn-start', topicId: 'servers', keys: 'F5' },
            { target: 'pw-run', topicId: 'playwright', keys: '' },
        ];
        expect(contextForTopic(context, 'servers')).toHaveLength(1);
        expect(contextForTopic(context, 'nope')).toEqual([]);
        expect(contextLabel(context[0])).toBe('btn-start (F5)');
        expect(contextLabel(context[1])).toBe('pw-run');
    });
});

// ---- view ----

function indexFixture() {
    return {
        version: 'v2.1.0',
        topics: [
            {
                id: 'servers', title: 'Server lifecycle', section: 'Projects & servers',
                summary: 'Start and stop', body: 'Intro.\n\n## Controls\n- Start\n- Stop',
                keywords: ['start'], related: ['uptime'],
            },
            {
                id: 'uptime', title: 'Uptime', section: 'Projects & servers',
                summary: 'Ready check', body: 'Wait for port.', keywords: [], related: [],
            },
            { id: 'themes', title: 'Themes', section: 'Appearance', summary: 'Styles', body: 'Modes.', keywords: [] },
        ],
        sections: ['Projects & servers', 'Appearance'],
        faq: [{ id: 'f1', question: 'Where is config?', answer: 'projects.json', category: 'Configuration', topicId: 'servers' }],
        faqCategories: ['Configuration'],
        shortcuts: [{ keys: 'F5', action: 'Start server', context: 'Global', group: 'Servers' }],
        shortcutGroups: ['Servers'],
        tutorials: [
            {
                id: 'first-project', title: 'Start your first project', summary: 'Add and run',
                steps: [
                    { index: 0, title: 'Add', body: 'Add a project', target: '#btn-add', topicId: 'servers' },
                    { index: 1, title: 'Start', body: 'Press start', target: '#btn-start', topicId: 'servers' },
                ],
            },
        ],
        context: [{ target: 'btn-start', topicId: 'servers', title: 'Server lifecycle', text: 'Start it', keys: 'F5' }],
        progress: { tutorials: { 'first-project': { step: 0, total: 2, completed: false } }, lastTopic: 'servers' },
        summary: { completed: 0, total: 1, pct: 0 },
        changelog: [{ version: '2.1.0', date: '2026-09-07', sections: [{ title: 'Features', items: ['updater'] }] }],
    };
}

function mountView(apiOverrides = {}) {
    const section = document.createElement('section');
    section.id = 'help-view';
    section.hidden = true;
    document.body.appendChild(section);
    const api = {
        getHelpIndex: vi.fn(async () => indexFixture()),
        searchHelp: vi.fn(async (q) => (q === 'nada'
            ? []
            : [{ kind: 'topic', id: 'servers', title: 'Server lifecycle', section: 'Projects & servers', snippet: 'Start and stop' }])),
        setTutorialStep: vi.fn(async () => []),
        setHelpLastTopic: vi.fn(async () => []),
        resetHelpProgress: vi.fn(async () => []),
        ...apiOverrides,
    };
    const view = mount({ api });
    return { view, api, section };
}

describe('help view', () => {
    afterEach(() => {
        document.getElementById('help-view')?.remove();
        document.getElementById('help-styles')?.remove();
        document.body.innerHTML = '';
    });

    it('sin contenedor avisa y no rompe', () => {
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
        const view = mount({ api: {} });
        expect(warn).toHaveBeenCalled();
        expect(view.isOpen()).toBe(false);
        view.open();
        expect(view.isOpen()).toBe(false);
        warn.mockRestore();
    });

    it('open carga el índice y pinta la navegación y el tema', async () => {
        const { view, api, section } = mountView();
        await view.open();

        expect(api.getHelpIndex).toHaveBeenCalled();
        expect(view.isOpen()).toBe(true);
        expect(section.querySelector('h2').textContent).toBe('Help & documentation');
        const navTopics = [...section.querySelectorAll('[data-topic-id]')].map((b) => b.dataset.topicId);
        expect(navTopics).toEqual(['servers', 'uptime', 'themes']);
        // lastTopic del progreso se respeta.
        expect(view.getState().topicId).toBe('servers');
        expect(section.querySelector('h3').textContent).toBe('Server lifecycle');
        expect(section.textContent).toContain('Controls');
        expect(api.setHelpLastTopic).not.toHaveBeenCalled(); // sólo al navegar
    });

    it('navega entre temas y enlaza relacionadas', async () => {
        const { view, api, section } = mountView();
        await view.open();
        const uptime = [...section.querySelectorAll('[data-topic-id]')].find((b) => b.dataset.topicId === 'uptime');
        uptime.click();
        expect(view.getState().topicId).toBe('uptime');
        expect(section.querySelector('h3').textContent).toBe('Uptime');
        expect(api.setHelpLastTopic).toHaveBeenCalledWith('uptime');

        view.openTopic('servers');
        const related = section.querySelector('[data-related-id="uptime"]');
        expect(related).toBeTruthy();
        related.click();
        expect(view.getState().topicId).toBe('uptime');
    });

    it('la búsqueda usa el backend y lista resultados', async () => {
        const { view, api, section } = mountView();
        await view.open();
        await view.runSearch('playwright');
        expect(api.searchHelp).toHaveBeenCalledWith('playwright', 25);
        expect(view.getState().query).toBe('playwright');
        expect(section.textContent).toContain('Results for');
        expect(section.querySelector('[data-hit-id="servers"]')).toBeTruthy();

        await view.runSearch('nada');
        expect(section.textContent).toContain('Nothing found');
        await view.runSearch('');
        expect(view.getState().hits).toEqual([]);
    });

    it('busca localmente si no hay backend', async () => {
        const section = document.createElement('section');
        section.id = 'help-view';
        section.hidden = true;
        document.body.appendChild(section);
        const view = mount({});
        // Sin api el índice queda nulo: el visor no debe romper.
        await view.refresh();
        expect(view.getState().index).toBeNull();
        await view.runSearch('playwright');
        expect(view.getState().hits).toEqual([]);
    });

    it('pestañas: FAQ con acordeón, atajos, changelog y contexto', async () => {
        const { view, section } = mountView();
        await view.open();

        view.selectTab('faq');
        const question = section.querySelector('.hp-faq-q');
        expect(question.textContent).toBe('Where is config?');
        expect(section.textContent).not.toContain('projects.json');
        question.click();
        expect(section.textContent).toContain('projects.json');
        question.click();
        expect(section.textContent).not.toContain('projects.json');

        view.selectTab('shortcuts');
        expect(section.textContent).toContain('F5');
        expect(section.textContent).toContain('Start server');

        view.selectTab('changelog');
        expect(section.textContent).toContain('2.1.0');
        expect(section.textContent).toContain('updater');

        view.selectTab('context');
        expect(section.querySelectorAll('.hp-card')).toHaveLength(1);
        expect(section.querySelector('.hp-card').textContent).toContain('btn-start');
        expect(section.textContent).toContain('Start it');

        view.selectTab('docs');
        expect(section.querySelector('h3').textContent).toBe('Server lifecycle');
    });

    it('marca pasos de tutorial y persiste el progreso', async () => {
        const { view, api, section } = mountView();
        await view.open();
        view.openTutorial('first-project');
        expect(section.querySelector('[data-step-index="1"]').className).toContain('current');
        expect(section.textContent).toContain('0/1 tutorials completed');		// El progreso del fixture ya tiene el paso 0 hecho: se marca el 1.
		section.querySelector('[data-tutorial-done="first-project"]').click();
		await new Promise((r) => setTimeout(r, 0));
		expect(api.setTutorialStep).toHaveBeenCalledWith('first-project', 1);

        // Con el progreso al día, el paso 2 pasa a ser el actual.
        api.getHelpIndex.mockResolvedValue({
            ...indexFixture(),
            progress: { tutorials: { 'first-project': { step: 1, total: 2 } }, lastTopic: 'servers' },
            summary: { completed: 1, total: 1, pct: 100 },
        });
        await view.refresh();
        view.openTutorial('first-project');
        expect(section.textContent).toContain('1/1 tutorials completed (100%)');
    });

    it('reset de progreso y cerrar con Escape', async () => {
        const { view, api, section } = mountView();
        await view.open();
        view.selectTab('tutorials');
        [...section.querySelectorAll('.btn')].find((b) => b.textContent === 'Reset progress').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.resetHelpProgress).toHaveBeenCalled();

        window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
        expect(view.isOpen()).toBe(false);
        view.destroy();
    });

    it('flashTarget resalta el elemento y falla en silencio si no existe', async () => {
        const { view, section } = mountView();
        await view.open();
        const target = document.createElement('button');
        target.id = 'btn-start';
        document.body.appendChild(target);
        view.selectTab('context');
        [...section.querySelectorAll('.btn')].find((b) => b.textContent === 'Show me').click();
        expect(target.className).toContain('hp-flash');
        target.remove();
    });
});
