import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Valida el flujo del dialog de tabs: visibilidad (hidden) + orden.
// El saver se inyecta en mount(); window.messageDialog se stubea (solo
// se usa en paths de error).
describe('tabs dialog', () => {
    let dialog = null;

    beforeEach(() => {
        window.messageDialog = { alert: vi.fn() };
    });

    afterEach(() => {
        if (dialog) {
            dialog.close();
            dialog.getElement().remove();
            dialog = null;
        }
    });

    async function openDialog() {
        const { mountTabsDialog, CUSTOMIZABLE } = await import('../dialogs/tabs.js');
        const d = mountTabsDialog(vi.fn(), vi.fn().mockResolvedValue([]));
        document.body.appendChild(d.getElement());
        dialog = d;
        return { d, CUSTOMIZABLE };
    }

    const $in = (d, id) => d.getElement().querySelector(`#${id}`);
    const rows = (d) => [...d.getElement().querySelectorAll('[data-tab-id]')];

    function tick() {
        return new Promise((resolve) => setTimeout(resolve, 0));
    }

    it('lista solo tabs personalizables (Logs no)', async () => {
        const { d, CUSTOMIZABLE } = await openDialog();
        d.open(0, { name: 'x', tabs: {} });
        const ids = rows(d).map((r) => r.dataset.tabId);
        expect(ids).toHaveLength(CUSTOMIZABLE.length);
        expect(ids).not.toContain('logs');
    });

    it('checboxes reflejan project.tabs.hidden', async () => {
        const { d } = await openDialog();
        d.open(0, { name: 'x', tabs: { hidden: ['obscura'], order: [] } });
        expect($in(d, 'tc-obscura').checked).toBe(false);
        expect($in(d, 'tc-scripts').checked).toBe(true);
    });

    it('guardar envía hidden + order correctos', async () => {
        const { d } = await openDialog();
        const saver = vi.fn().mockResolvedValue([]);
        d.setSaver(saver);
        d.open(0, { name: 'x', tabs: { hidden: [], order: [] } });

        // ocultar playwright
        $in(d, 'tc-playwright').checked = false;
        $in(d, 'tc-playwright').dispatchEvent(new Event('change'));
        // mover scripts un lugar abajo → [git, scripts, ...]
        rows(d)[0].querySelectorAll('.tabs-custom-move')[1].click();
        d.getElement().querySelector('.dialog-buttons .btn-primary').click();
        await tick();

        expect(saver).toHaveBeenCalledTimes(1);
        const [index, updated] = saver.mock.calls[0];
        expect(index).toBe(0);
        expect(updated.tabs.hidden).toEqual(['playwright']);
        expect(updated.tabs.order).toEqual(['git', 'scripts', 'deps', 'playwright', 'evidence', 'obscura', 'backlog']);
        // resto del proyecto preservado
        expect(updated.name).toBe('x');
    });

    it('flechas mueven el orden', async () => {
        const { d } = await openDialog();
        d.open(0, { name: 'x', tabs: {} });
        let ids = rows(d).map((r) => r.dataset.tabId);
        expect(ids[0]).toBe('scripts');

        rows(d)[0].querySelectorAll('.tabs-custom-move')[1].click(); // down
        ids = rows(d).map((r) => r.dataset.tabId);
        expect(ids[0]).toBe('git');
        expect(ids[1]).toBe('scripts');
    });

    it('mergeOrder preserva orden guardado y agrega el resto al final', async () => {
        const { mountTabsDialog, mergeOrder } = await import('../dialogs/tabs.js');
        expect(mergeOrder(['deps', 'scripts'])).toEqual(['deps', 'scripts', 'git', 'playwright', 'evidence', 'obscura', 'backlog']);
        expect(mergeOrder(['bogus', 'deps', 'deps']).length).toBe(7); // ignora unknown + dupes
        expect(mergeOrder(undefined)).toHaveLength(7);
        expect(mountTabsDialog).toBeTypeOf('function');
    });
});