const CAPTION_MAX = 42;
const THUMB_BATCH = 20;
const KIND_ICON = {
    image: '\uD83D\uDDBC\uFE0F',
    video: '\uD83C\uDF9E\uFE0F',
    trace: '\uD83D\uDDDC\uFE0F',
};

export function mount(ctx) {
    const { $, api, events } = ctx;

    const gallery = $('ev-gallery');
    const grid = $('ev-grid');
    const split = $('ev-split');
    const empty = $('ev-empty');
    const countLbl = $('ev-count');
    const reportBtn = $('ev-report');
    const previewImg = $('ev-preview-img');
    const previewPh = $('ev-preview-placeholder');

    let files = [];
    let byPath = new Map();
    let observer = null;
    let loadSeq = 0;
    let pending = [];   // [card, file] esperando batch de thumbnails
    let inFlight = 0;   // thumbnails en curso (batch m├íx THUMB_BATCH)
    let menuEl = null;

    function caption(relPath) {
        const s = relPath.replace(/\\/g, '/');
        return s.length > CAPTION_MAX ? '\u2026' + s.slice(-(CAPTION_MAX - 1)) : s;
    }

    function clearPreview() {
        previewImg.hidden = true;
        previewImg.removeAttribute('src');
        previewPh.hidden = false;
        previewPh.textContent = 'Select an image to preview';
    }

    async function showPreview(f) {
        if (!f._url) {
            try {
                f._url = await api.getEvidenceThumbnail(f.path);
            } catch { f._url = ''; }
        }
        if (!f._url) return; // sin datos (<2MB): se queda el placeholder
        previewImg.src = f._url;
        previewImg.hidden = false;
        previewPh.hidden = true;
    }

    function selectCard(card) {
        grid.querySelectorAll('.ev-card.selected')
            .forEach((c) => c.classList.remove('selected'));
        if (card) card.classList.add('selected');
    }

    function openTrace(f) {
        if (!confirm('Open Playwright Trace Viewer for this file?')) return;
        api.openTraceViewer(ctx.selectedIndex(), f.path);
    }

    function activate(f) {
        if (f.kind === 'image') showPreview(f);
        else if (f.kind === 'video') api.openExternally(f.path);
        else openTrace(f);
    }

    function cardEl(f) {
        const card = document.createElement('div');
        card.className = 'ev-card';
        card.dataset.path = f.path;
        card.title = f.relPath;

        const thumb = document.createElement('div');
        thumb.className = 'ev-thumb' + (f.kind === 'trace' ? ' trace' : '');
        thumb.textContent = KIND_ICON[f.kind] || KIND_ICON.image;

        const cap = document.createElement('div');
        cap.className = 'ev-caption';
        cap.textContent = caption(f.relPath);

        card.append(thumb, cap);
        card.addEventListener('click', () => {
            selectCard(card);
            if (f.kind === 'image') showPreview(f);
        });
        card.addEventListener('dblclick', () => activate(f));
        return card;
    }

    // ---- Thumbnails lazy: IntersectionObserver + batch ----

    function ensureObserver() {
        if (observer) return;
        observer = new IntersectionObserver((entries) => {
            for (const e of entries) {
                if (!e.isIntersecting) continue;
                observer.unobserve(e.target);
                const f = byPath.get(e.target.dataset.path);
                if (f && f.kind === 'image' && !f._url) queueThumb(e.target, f);
            }
        }, { root: gallery, rootMargin: '120px' });
    }

    function queueThumb(card, f) {
        pending.push([card, f]);
        pump();
    }

    function pump() {
        while (inFlight < THUMB_BATCH && pending.length) {
            const [card, f] = pending.shift();
            inFlight += 1;
            api.getEvidenceThumbnail(f.path)
                .then((url) => {
                    if (!url || !card.isConnected || !byPath.has(f.path)) return;
                    f._url = url;
                    const img = document.createElement('img');
                    img.alt = f.relPath;
                    img.src = url;
                    card.querySelector('.ev-thumb').replaceChildren(img);
                })
                .catch(() => {})
                .finally(() => { inFlight -= 1; pump(); });
        }
    }

    // ---- Men├║ contextual ----

    function closeMenu() {
        if (!menuEl) return;
        menuEl.remove();
        menuEl = null;
        document.removeEventListener('click', onDocClick, true);
        document.removeEventListener('contextmenu', onDocContext, true);
    }

    function onDocClick(e) {
        if (menuEl && !menuEl.contains(e.target)) closeMenu();
    }

    function onDocContext() { closeMenu(); }

    function menuItem(label, fn) {
        const b = document.createElement('button');
        b.textContent = label;
        b.addEventListener('click', () => { closeMenu(); fn(); });
        return b;
    }

    function openMenu(x, y, f) {
        closeMenu();
        menuEl = document.createElement('div');
        menuEl.className = 'ev-context';
        menuEl.appendChild(menuItem('Open Externally', () => api.openExternally(f.path)));
        menuEl.appendChild(menuItem('Open Containing Folder', () => api.openContainingFolder(f.path)));
        if (f.kind === 'trace') {
            menuEl.appendChild(menuItem('Show Trace Viewer', () => openTrace(f)));
        }
        document.body.appendChild(menuEl);
        const r = menuEl.getBoundingClientRect();
        menuEl.style.left = `${Math.max(4, Math.min(x, window.innerWidth - r.width - 8))}px`;
        menuEl.style.top = `${Math.max(4, Math.min(y, window.innerHeight - r.height - 8))}px`;
        document.addEventListener('click', onDocClick, true);
        document.addEventListener('contextmenu', onDocContext, true);
    }

    // ---- Carga / render ----

    async function load() {
        const i = ctx.selectedIndex();
        const seq = ++loadSeq;

        if (observer) observer.disconnect();
        pending = [];
        grid.innerHTML = '';
        byPath = new Map();
        files = [];
        split.hidden = true;
        empty.hidden = true;
        countLbl.textContent = '';
        reportBtn.disabled = true;
        clearPreview();
        if (i < 0) return;

        let list = [];
        try { list = await api.getEvidence(i) || []; } catch { list = []; }
        if (seq !== loadSeq) return;
        files = list;

        const has = files.length > 0;
        split.hidden = !has;
        empty.hidden = has;
        countLbl.textContent = `${files.length} artifact(s)`;
        // Sin binding de existencia del report: aproximaci├│n con artifacts.
        reportBtn.disabled = !has;
        if (!has) return;

        ensureObserver();
        for (const f of files) {
            byPath.set(f.path, f);
            const card = cardEl(f);
            grid.appendChild(card);
            observer.observe(card);
        }
    }

    gallery.addEventListener('contextmenu', (e) => {
        const card = e.target.closest('.ev-card');
        if (!card) return;
        e.preventDefault();
        e.stopPropagation();
        const f = byPath.get(card.dataset.path);
        if (f) {
            selectCard(card);
            openMenu(e.clientX, e.clientY, f);
        }
    });

    $('ev-refresh').addEventListener('click', load);
    reportBtn.addEventListener('click', () => api.openHTMLReport(ctx.selectedIndex()));

    events().EventsOn('trace:log', ({ index, line, isError }) =>
        ctx.appendLog(index, line, isError));

    return { onProjectChanged: load };
}
