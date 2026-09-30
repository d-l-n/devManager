import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mountExternalNotifications } from '../widgets/external-notifications.js';

const notifyMock = {
    NotifyGetConfig: vi.fn(),
    NotifyHistory: vi.fn(),
    NotifySavePlatform: vi.fn(),
    NotifyDeletePlatform: vi.fn(),
    NotifySaveRule: vi.fn(),
    NotifyDeleteRule: vi.fn(),
    NotifyEventFire: vi.fn(),
};

function section() {
    document.body.innerHTML = '<div id="settings-notifications-section"></div>';
    return document.getElementById('settings-notifications-section');
}

beforeEach(() => {
    vi.clearAllMocks();
    window.go = { main: { App: notifyMock } };
    notifyMock.NotifyGetConfig.mockResolvedValue({
        platforms: [{ id: 'platform', name: 'Ops', platform: 'slack', webhook_url: 'https://x', min_priority: 'info', events: [], enabled: true }],
        rules: [{ id: 'rule', name: 'Críticos', event: 'all', min_priority: 'critical', enabled: true }],
        events: ['app'],
        priorities: ['info'],
    });
    notifyMock.NotifyHistory.mockResolvedValue([
        { id: 'd1', at: new Date().toISOString(), platform: 'slack', name: 'Ops', event: 'app', priority: 'info', title: 't', status: 'sent', attempts: 1 },
    ]);
});

describe('mountExternalNotifications', () => {
    it('renderiza plataformas, reglas e historial desde el backend', async () => {
        const panel = mountExternalNotifications(section());
        await new Promise((r) => setTimeout(r, 0));
        expect(notifyMock.NotifyGetConfig).toHaveBeenCalled();
        expect(notifyMock.NotifyHistory).toHaveBeenCalledWith(50);
        expect(document.body.textContent).toContain('Ops — Slack');
        expect(document.body.textContent).toContain('Críticos');
        expect(document.body.textContent).toContain('sent');
        expect(panel).not.toBeNull();
    });

    it('guardar plataforma llama al binding y refresca', async () => {
        notifyMock.NotifySavePlatform.mockResolvedValue(null);
        const panel = mountExternalNotifications(section());
        await new Promise((r) => setTimeout(r, 0));
        const add = [...document.querySelectorAll('button')].find((b) => b.textContent.includes('Add platform'));
        add.click();
        await new Promise((r) => setTimeout(r, 0));
        const name = document.querySelector('.notify-form input');
        name.value = 'Canal QA';
        const save = [...document.querySelectorAll('.notify-form button')].find((b) => b.textContent === 'Save platform');
        save.click();
        await new Promise((r) => setTimeout(r, 0));
        expect(notifyMock.NotifySavePlatform).toHaveBeenCalled();
        const arg = notifyMock.NotifySavePlatform.mock.calls[0][0];
        expect(arg.name).toBe('Canal QA');
        expect(arg.platform).toBe('slack');
    });

    it('muestra fallback si el backend falla', async () => {
        notifyMock.NotifyGetConfig.mockRejectedValue(new Error('no backend'));
        mountExternalNotifications(section());
        await new Promise((r) => setTimeout(r, 0));
        expect(document.body.textContent).toContain('unavailable');
    });
});
