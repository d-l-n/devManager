import { describe, it, expect, vi, afterEach } from 'vitest';

// Fase 2 #67: dialog env-vars con api mockeada (Fase 3: secrets + diff).
describe('env-vars dialog', () => {
    let dialog = null;

    afterEach(() => {
        if (dialog) {
            dialog.close();
            dialog.getElement().remove();
            dialog = null;
        }
    });

    function mockApi(over = {}) {
        return {
            getEnvVars: vi.fn().mockResolvedValue({}),
            setEnvVars: vi.fn().mockResolvedValue([]),
            loadDotEnvFile: vi.fn().mockResolvedValue({}),
            saveDotEnvFile: vi.fn().mockResolvedValue([]),
            getServerStatus: vi.fn().mockResolvedValue({ state: 'stopped' }),
            getSecrets: vi.fn().mockResolvedValue([]),
            setSecretKey: vi.fn().mockResolvedValue([]),
            unsetSecretKey: vi.fn().mockResolvedValue([]),
            getEnvDiff: vi.fn().mockResolvedValue([]),
            ...over,
        };
    }

    async function openDialog(api, initialVars, extra) {
        const { mount } = await import('../dialogs/env-vars.js');
        const d = mount({ api, messageDialog: { alert: vi.fn(), confirm: vi.fn() } });
        dialog = d;
        await d.open(0, 'dev', initialVars, extra);
        return d;
    }

    const q = (sel) => dialog.getElement().querySelector(sel);
    const qa = (sel) => [...dialog.getElement().querySelectorAll(sel)];
    const keys = () => qa('.ev-row .ev-key').map((i) => i.value);

    it('open renderiza vars iniciales ordenadas', async () => {
        const api = mockApi();
        await openDialog(api, { ZED: '1', ALPHA: '2' });
        expect(api.getEnvVars).not.toHaveBeenCalled(); // initialVars evita fetch
        expect(keys()).toEqual(['ALPHA', 'ZED']);
    });

    it('open sin initialVars pide vars al backend (enmascarado)', async () => {
        const api = mockApi({ getEnvVars: vi.fn().mockResolvedValue({ FOO: 'bar' }) });
        await openDialog(api, undefined);
        expect(api.getEnvVars).toHaveBeenCalledWith(0, 'dev', false);
        expect(keys()).toEqual(['FOO']);
    });

    it('key inválida bloquea save', async () => {
        const api = mockApi();
        await openDialog(api, { FOO: 'x' });
        qa('.ev-row .ev-key')[0].value = 'bad-key';
        q('#ev-save').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.setEnvVars).not.toHaveBeenCalled();
        expect(q('#ev-error').textContent).toMatch(/Invalid variable name/);
    });

    it('duplicada bloquea save', async () => {
        const api = mockApi();
        await openDialog(api, {});
        q('#ev-new-key').value = 'DUP';
        q('#ev-new-value').value = '1';
        q('#ev-add').click();
        q('#ev-new-key').value = 'DUP';
        q('#ev-new-value').value = '2';
        q('#ev-add').click(); // add también bloquea dup
        expect(q('#ev-error').textContent).toMatch(/Duplicate/);
        expect(api.setEnvVars).not.toHaveBeenCalled();
    });

    it('save válido llama api con el mapa', async () => {
        const api = mockApi();
        await openDialog(api, { FOO: 'bar' });
        q('#ev-new-key').value = 'NEW_KEY';
        q('#ev-new-value').value = 'val';
        q('#ev-add').click();
        q('#ev-save').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.setEnvVars).toHaveBeenCalledTimes(1);
        expect(api.setEnvVars).toHaveBeenCalledWith(0, 'dev', { FOO: 'bar', NEW_KEY: 'val' });
    });

    it('running bloquea save a proyecto', async () => {
        const api = mockApi({ getServerStatus: vi.fn().mockResolvedValue({ state: 'running' }) });
        await openDialog(api, { FOO: 'bar' });
        expect(q('#ev-save').disabled).toBe(true);
        expect(q('#ev-running-banner').textContent).toMatch(/running/);
        q('#ev-save').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.setEnvVars).not.toHaveBeenCalled();
    });

    it('load desde archivo reemplaza tabla', async () => {
        const api = mockApi({ loadDotEnvFile: vi.fn().mockResolvedValue({ FROM_FILE: '1' }) });
        await openDialog(api, { OLD: 'x' });
        expect(keys()).toEqual(['OLD']);
        q('#ev-load').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.loadDotEnvFile).toHaveBeenCalledWith(0, 'dev');
        expect(keys()).toEqual(['FROM_FILE']);
    });

    it('save a archivo llama api con el mapa', async () => {
        const api = mockApi();
        await openDialog(api, { FOO: 'bar' });
        q('#ev-save-file').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.saveDotEnvFile).toHaveBeenCalledWith(0, 'dev', { FOO: 'bar' });
    });

    it('delete quita la fila del collect', async () => {
        const api = mockApi();
        const d = await openDialog(api, { KEEP: '1', DROP: '2' });
        qa('.ev-row .ev-del')[0].click(); // filas ordenadas: DROP primero
        expect(d.collect()).toEqual({ KEEP: '1' });
    });

    it('secret checkbox marca key y pasa input a password', async () => {
        const api = mockApi();
        await openDialog(api, { TOKEN: 'real' });
        const box = q('.ev-row .ev-secret');
        expect(box.checked).toBe(false);
        box.checked = true;
        box.dispatchEvent(new Event('change'));
        await new Promise((r) => setTimeout(r, 0));
        expect(api.setSecretKey).toHaveBeenCalledWith(0, 'dev', 'TOKEN');
        expect(q('.ev-row .ev-value').type).toBe('password');
    });

    it('secret checkbox error revierte', async () => {
        const api = mockApi({ setSecretKey: vi.fn().mockResolvedValue(['boom']) });
        await openDialog(api, { TOKEN: 'real' });
        const box = q('.ev-row .ev-secret');
        box.checked = true;
        box.dispatchEvent(new Event('change'));
        await new Promise((r) => setTimeout(r, 0));
        expect(box.checked).toBe(false);
        expect(q('#ev-error').textContent).toMatch(/boom/);
    });

    it('uncheck desmarca via api', async () => {
        const api = mockApi();
        await openDialog(api, { TOKEN: '***' }, { secrets: ['TOKEN'] });
        const box = q('.ev-row .ev-secret');
        expect(box.checked).toBe(true);
        expect(q('.ev-row .ev-value').type).toBe('password');
        box.checked = false;
        box.dispatchEvent(new Event('change'));
        await new Promise((r) => setTimeout(r, 0));
        expect(api.unsetSecretKey).toHaveBeenCalledWith(0, 'dev', 'TOKEN');
        expect(q('.ev-row .ev-value').type).toBe('text');
    });

    it('reveal trae reales con flag true y hide re-enmascara', async () => {
        const api = mockApi({
            getEnvVars: vi.fn()
                .mockResolvedValueOnce({ TOKEN: '***' })
                .mockResolvedValueOnce({ TOKEN: 'real-secret' })
                .mockResolvedValue({ TOKEN: '***' }),
            getSecrets: vi.fn().mockResolvedValue(['TOKEN']),
        });
        await openDialog(api, undefined);
        expect(q('.ev-row .ev-value').value).toBe('***');
        q('#ev-reveal').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.getEnvVars).toHaveBeenCalledWith(0, 'dev', true);
        expect(q('.ev-row .ev-value').value).toBe('real-secret');
        q('#ev-reveal').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(q('.ev-row .ev-value').value).toBe('***');
    });

    it('diff renderiza filas con status', async () => {
        const api = mockApi({
            getEnvDiff: vi.fn().mockResolvedValue([
                { key: 'ADD', a: '', b: 'new', status: 'added' },
                { key: 'CHG', a: '1', b: '2', status: 'changed' },
                { key: 'SAME', a: 'x', b: 'x', status: 'same' },
            ]),
        });
        await openDialog(api, {}, { envNames: ['dev', 'staging'] });
        q('#ev-diff-a').value = 'dev';
        q('#ev-diff-b').value = 'staging';
        q('#ev-compare').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(api.getEnvDiff).toHaveBeenCalledWith(0, 'dev', 'staging');
        const rows = qa('.ev-diff-row');
        expect(rows.map((r) => r.dataset.diffKey)).toEqual(['ADD', 'CHG', 'SAME']);
        expect(rows.map((r) => r.dataset.diffStatus)).toEqual(['added', 'changed', 'same']);
        expect(rows[1].textContent).toMatch(/changed/);
    });

    it('diff sin api muestra error', async () => {
        const api = mockApi();
        delete api.getEnvDiff;
        await openDialog(api, {});
        q('#ev-compare').click();
        await new Promise((r) => setTimeout(r, 0));
        expect(q('#ev-error').textContent).toMatch(/not available/);
    });
});
