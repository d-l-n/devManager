import { describe, it, expect, vi, afterEach } from 'vitest';
import {
    isValidEnvName, ensureEnvs, effectiveServer, activeEnvOf, isServerRunning,
} from '../envs.js';

// Fase 1 #67: helpers puros + CRUD del dialog + bloqueo con running.
describe('env helpers', () => {
    it('isValidEnvName acepta ^[a-z0-9_-]{1,32}$', () => {
        expect(isValidEnvName('dev')).toBe(true);
        expect(isValidEnvName('staging-1')).toBe(true);
        expect(isValidEnvName('feat_x')).toBe(true);
        expect(isValidEnvName('Dev!')).toBe(false);
        expect(isValidEnvName('')).toBe(false);
        expect(isValidEnvName('a'.repeat(33))).toBe(false);
        expect(isValidEnvName('a'.repeat(32))).toBe(true);
    });

    it('ensureEnvs sintetiza dev desde top-level legacy', () => {
        const legacy = { name: 'x', server: { port: 3000 }, playwright: { command: 'pw' }, user: { command: 'u' } };
        const out = ensureEnvs(legacy);
        expect(out.active_env).toBe('dev');
        expect(out.envs.dev.server.port).toBe(3000);
        expect(out.envs.dev.playwright.command).toBe('pw');
    });

    it('ensureEnvs respeta envs existentes', () => {
        const p = { active_env: 'staging', envs: { dev: { server: { port: 1 } }, staging: { server: { port: 2 } } } };
        const out = ensureEnvs(p);
        expect(out.active_env).toBe('staging');
        expect(Object.keys(out.envs)).toHaveLength(2);
    });

    it('effectiveServer devuelve el puerto del active', () => {
        const p = {
            server: { port: 9999 },
            active_env: 'staging',
            envs: { dev: { server: { port: 5173 } }, staging: { server: { port: 3000 } } },
        };
        expect(effectiveServer(p).port).toBe(3000);
        expect(activeEnvOf(p)).toBe('staging');
    });

    it('isServerRunning cubre running/starting/stopping', () => {
        expect(isServerRunning('running')).toBe(true);
        expect(isServerRunning('starting')).toBe(true);
        expect(isServerRunning('stopping')).toBe(true);
        expect(isServerRunning('stopped')).toBe(false);
        expect(isServerRunning('error')).toBe(false);
    });
});

describe('project dialog environments', () => {
    let dialog = null;

    afterEach(() => {
        if (dialog) {
            dialog.close();
            dialog.getElement().remove();
            dialog = null;
        }
    });

    async function openEdit(project) {
        const { mountProjectDialog } = await import('../dialogs/project.js');
        const d = mountProjectDialog(vi.fn());
        dialog = d;
        d.openEdit(0, project);
        return d;
    }

    const q = (sel) => dialog.getElement().querySelector(sel);
    const qa = (sel) => [...dialog.getElement().querySelectorAll(sel)];

    it('legacy sin envs muestra dev sintetizado', async () => {
        const d = await openEdit({ name: 'x', path: 'y', server: { port: 3000 } });
        const names = qa('#pf-env-list [data-env-name]').map((b) => b.dataset.envName);
        expect(names).toEqual(['dev']);
        const proj = d.collect();
        expect(proj.active_env).toBe('dev');
        expect(proj.envs.dev.server.port).toBe(3000);
    });

    it('add copia desde el env seleccionado y collect incluye envs', async () => {
        const d = await openEdit({
            name: 'x', path: 'y',
            server: { port: 5173 }, active_env: 'dev', envs: { dev: { server: { port: 5173 } } },
        });
        q('#pf-env-new').value = 'staging';
        q('#pf-env-add').click();
        const names = qa('#pf-env-list [data-env-name]').map((b) => b.dataset.envName);
        expect(names).toContain('staging');
        const proj = d.collect();
        expect(proj.envs.staging.server.port).toBe(5173);
        expect(proj.server.port).toBe(5173); // top-level = active
    });

    it('delete bloquea active y last', async () => {
        const d = await openEdit({
            name: 'x', path: 'y',
            server: { port: 1 }, active_env: 'dev',
            envs: { dev: { server: { port: 1 } }, staging: { server: { port: 2 } } },
        });
        // seleccionado = active → delete deshabilitado
        expect(q('#pf-env-delete-btn').disabled).toBe(true);
        // pasar a staging (no active) → delete habilitado
        qa('#pf-env-list [data-env-name]').find((b) => b.dataset.envName === 'staging').click();
        expect(q('#pf-env-delete-btn').disabled).toBe(false);
        q('#pf-env-delete-btn').click();
        expect(d._state.envs.staging).toBeUndefined();
        // ahora solo queda active → delete deshabilitado (last)
        expect(q('#pf-env-delete-btn').disabled).toBe(true);
        expect(d.validate()).toHaveLength(0);
    });

    it('validate rechaza nombre inválido y active huérfano', async () => {
        const d = await openEdit({ name: 'x', path: 'y', server: { port: 1 } });
        d._state.envs['Bad!'] = JSON.parse(JSON.stringify(d._state.envs.dev));
        expect(d.validate().length).toBeGreaterThan(0);
        delete d._state.envs['Bad!'];
        d._state.activeEnv = 'ghost';
        expect(d.validate()).toContain('Active environment does not exist');
    });
});
