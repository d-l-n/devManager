import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '../panels/workflows.js';

// Test mínimo del panel workflows: mount + lista + mapeo api (Issue #65).
function buildDOM() {
    document.body.innerHTML = `
        <button id="wf-add"></button>
        <button id="wf-refresh"></button>
        <select id="wf-event-select"><option value="server_started">server_started</option></select>
        <button id="wf-fire"></button>
        <input id="wf-hook-name"><input id="wf-hook-url">
        <select id="wf-hook-events"><option value="server_started">server_started</option></select>
        <button id="wf-hook-add"></button>
        <div id="wf-result" hidden></div>
        <span id="wf-count"></span>
        <div id="wf-list"></div>
        <div id="wf-empty" hidden></div>
        <div id="wf-editor" hidden></div>
        <span id="wf-runs-count"></span>
        <div id="wf-runs"></div>
        <div id="wf-runs-empty" hidden></div>
        <div id="wf-hooks"></div>
        <div id="wf-hooks-empty" hidden></div>
        <div id="wf-ci"></div>
        <span id="wf-listener"></span>`;
}

function stubCtx(overrides = {}) {
    const api = {
        listWorkflows: vi.fn(async () => [
            { id: 'w1', name: 'demo', enabled: true, trigger: 'manual', steps: [] },
        ]),
        saveWorkflow: vi.fn(async () => []),
        deleteWorkflow: vi.fn(async () => []),
        runWorkflow: vi.fn(async () => []),
        listWorkflowRuns: vi.fn(async () => []),
        getWorkflowRuns: vi.fn(async () => []),
        triggerEvent: vi.fn(async () => []),
        listWebhooks: vi.fn(async () => []),
        saveWebhook: vi.fn(async () => []),
        deleteWebhook: vi.fn(async () => []),
        saveCIConfig: vi.fn(async () => []),
        getCIConfig: vi.fn(async () => ({ github: {}, gitlab: {}, jenkins: {} })),
        triggerCIBuild: vi.fn(async () => ({ ok: true, message: 'queued' })),
        getCIStatus: vi.fn(async () => ({ status: 'unknown' })),
        getWorkflowListenerAddr: vi.fn(async () => '127.0.0.1:9876'),
        ...overrides,
    };
    return {
        $: (id) => document.getElementById(id),
        api,
        events: () => ({ EventsOn: vi.fn() }),
        selectedIndex: () => 0,
        messageDialog: { confirm: vi.fn(async () => false) },
    };
}

function tick() {
    return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('workflows panel', () => {
    beforeEach(() => {
        buildDOM();
    });

    it('lista workflows del proyecto via api.listWorkflows', async () => {
        const ctx = stubCtx();
        const panel = mount(ctx);
        panel.onProjectChanged({ name: 'p' });
        await tick();
        await tick();
        expect(ctx.api.listWorkflows).toHaveBeenCalledWith(0);
        expect(document.getElementById('wf-list').textContent).toContain('demo');
        expect(document.getElementById('wf-count').textContent).toContain('1 workflow');
    });

    it('botón Run llama api.runWorkflow con índice e id', async () => {
        const ctx = stubCtx();
        const panel = mount(ctx);
        panel.onProjectChanged({ name: 'p' });
        await tick();
        await tick();
        const runBtn = [...document.querySelectorAll('#wf-list button')]
            .find((b) => b.textContent === 'Run');
        expect(runBtn).toBeTruthy();
        runBtn.click();
        await tick();
        expect(ctx.api.runWorkflow).toHaveBeenCalledWith(0, 'w1');
    });

    it('muestra listener addr para webhooks entrantes', async () => {
        const ctx = stubCtx();
        const panel = mount(ctx);
        panel.onProjectChanged({ name: 'p' });
        await tick();
        await tick();
        expect(document.getElementById('wf-listener').textContent).toContain('127.0.0.1:9876');
    });
});
