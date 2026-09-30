// Helpers puros del sistema de ayuda (Issue #72). Sin DOM ni red: el visor
// (views/help.js) solo pinta lo que estas funciones devuelven.

// parseBody convierte el body de un tema en bloques pintables.
// Formato soportado: "## Subtítulo", "- viñeta" y párrafos separados por línea
// vacía (el mismo que usa internal/help/catalog.go).
export function parseBody(body) {
    const blocks = [];
    const lines = String(body || '').split('\n');
    let paragraph = [];

    const flush = () => {
        if (paragraph.length) {
            blocks.push({ type: 'paragraph', text: paragraph.join(' ') });
            paragraph = [];
        }
    };

    for (const raw of lines) {
        const line = raw.trim();
        if (!line) {
            flush();
            continue;
        }
        if (line.startsWith('## ')) {
            flush();
            blocks.push({ type: 'heading', text: line.slice(3).trim() });
            continue;
        }
        if (line.startsWith('- ')) {
            flush();
            blocks.push({ type: 'bullet', text: line.slice(2).trim() });
            continue;
        }
        if (line.startsWith('* ')) {
            flush();
            blocks.push({ type: 'bullet', text: line.slice(2).trim() });
            continue;
        }
        paragraph.push(line);
    }
    flush();
    return blocks;
}

// groupTopicsBySection respeta el orden de `sections` del backend.
export function groupTopicsBySection(topics, sections) {
    const list = topics || [];
    const order = (sections && sections.length ? sections : []);
    const names = order.length ? [...order] : [];
    for (const topic of list) {
        if (!names.includes(topic.section)) names.push(topic.section);
    }
    return names
        .map((section) => ({
            section,
            topics: list
                .filter((t) => t.section === section)
                .sort((a, b) => a.title.localeCompare(b.title)),
        }))
        .filter((group) => group.topics.length > 0);
}

export function matchesQuery(text, query) {
    const needle = String(query || '').trim().toLowerCase();
    if (!needle) return true;
    return String(text || '').toLowerCase().includes(needle);
}

// filterTopics filtra por sección y por texto (título, resumen y keywords).
export function filterTopics(topics, { section = '', query = '' } = {}) {
    return (topics || []).filter((topic) => {
        if (section && topic.section !== section) return false;
        if (!query) return true;
        const haystack = [topic.title, topic.summary, (topic.keywords || []).join(' ')].join(' ');
        return matchesQuery(haystack, query);
    });
}

// hitForTopic resuelve el destino de un resultado de búsqueda.
export function hitKind(hit) {
    return hit && hit.kind === 'faq' ? 'faq' : 'topic';
}

export function shortcutRows(shortcuts) {
    const list = shortcuts || [];
    const groups = [];
    for (const s of list) {
        let group = groups.find((g) => g.group === s.group);
        if (!group) {
            group = { group: s.group, rows: [] };
            groups.push(group);
        }
        group.rows.push({ keys: s.keys, action: s.action, context: s.context });
    }
    return groups;
}

export function faqGroups(faq, categories) {
    const list = faq || [];
    const names = (categories && categories.length ? [...categories] : []);
    for (const entry of list) {
        if (!names.includes(entry.category)) names.push(entry.category);
    }
    return names
        .map((category) => ({ category, entries: list.filter((f) => f.category === category) }))
        .filter((g) => g.entries.length > 0);
}

export function filterFAQ(faq, query) {
    if (!String(query || '').trim()) return faq || [];
    return (faq || []).filter((f) => matchesQuery(`${f.question} ${f.answer}`, query));
}

// ---- tutoriales ----

export function tutorialProgress(tutorial, progressMap) {
    const entry = (progressMap || {})[tutorial.id] || { step: -1, total: tutorial.steps.length };
    const total = tutorial.steps.length || 1;
    const done = Math.max(0, Math.min(total, (entry.step + 1)));
    return {
        step: entry.step,
        total,
        done,
        pct: Math.round((done / total) * 100),
        completed: done >= total,
    };
}

export function nextStepIndex(tutorial, progressMap) {
    const entry = (progressMap || {})[tutorial.id];
    const next = (entry ? entry.step : -1) + 1;
    return Math.max(0, Math.min(tutorial.steps.length, next));
}

export function progressSummary(summary) {
    const s = summary || { completed: 0, total: 0, pct: 0 };
    if (!s.total) return 'No tutorials available';
    return `${s.completed}/${s.total} tutorials completed (${Math.round(s.pct)}%)`;
}

// ---- changelog ----

export function changelogRows(releases) {
    return (releases || []).map((release) => ({
        version: release.version || 'Unreleased',
        date: release.date || '',
        sections: (release.sections || []).map((section) => ({
            title: section.title,
            items: section.items || [],
        })),
    }));
}

// ---- ayuda contextual ----

// contextForTopic devuelve los targets de UI enlazados a un tema.
export function contextForTopic(context, topicId) {
    return (context || []).filter((entry) => entry.topicId === topicId);
}

export function contextLabel(entry) {
    return entry.keys ? `${entry.target} (${entry.keys})` : entry.target;
}
