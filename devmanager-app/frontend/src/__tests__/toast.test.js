import { describe, it, expect, vi, afterEach } from 'vitest';

describe('showToast', () => {
    afterEach(() => {
        vi.useRealTimers();
    });

    it('creates a toast element', async () => {
        const { showToast, setToastsEnabled } = await import('../widgets/toast.js');
        setToastsEnabled(true);
        const card = showToast('Title', 'Message');
        expect(card).not.toBeNull();
        expect(card.classList.contains('toast')).toBe(true);
    });

    it('toast is appended to a container in the DOM', async () => {
        const { showToast, setToastsEnabled } = await import('../widgets/toast.js');
        setToastsEnabled(true);
        const card = showToast('Title');
        expect(card.parentElement).not.toBeNull();
        expect(card.parentElement.id).toBe('toast-container');
    });

    it('sets title text', async () => {
        const { showToast, setToastsEnabled } = await import('../widgets/toast.js');
        setToastsEnabled(true);
        const card = showToast('Hello');
        const title = card.querySelector('.toast-title');
        expect(title.textContent).toBe('Hello');
    });

    it('sets message text when provided', async () => {
        const { showToast, setToastsEnabled } = await import('../widgets/toast.js');
        setToastsEnabled(true);
        const card = showToast('Title', 'Body text');
        const msg = card.querySelector('.toast-msg');
        expect(msg.textContent).toBe('Body text');
    });

    it('omits message element when not provided', async () => {
        const { showToast, setToastsEnabled } = await import('../widgets/toast.js');
        setToastsEnabled(true);
        const card = showToast('Title');
        const msg = card.querySelector('.toast-msg');
        expect(msg).toBeNull();
    });

    it('auto-dismisses after timeout', async () => {
        vi.useFakeTimers();
        const { showToast, setToastsEnabled } = await import('../widgets/toast.js');
        setToastsEnabled(true);
        const card = showToast('Auto');
        const container = card.parentElement;
        const countBefore = container.children.length;
        vi.advanceTimersByTime(4220); // 4000 + 220ms fade
        expect(container.children.length).toBe(countBefore - 1);
    });

    it('click dismisses the toast', async () => {
        vi.useFakeTimers();
        const { showToast, setToastsEnabled } = await import('../widgets/toast.js');
        setToastsEnabled(true);
        const card = showToast('Click me');
        const container = card.parentElement;
        const countBefore = container.children.length;
        card.click();
        vi.advanceTimersByTime(220); // fade
        expect(container.children.length).toBe(countBefore - 1);
    });

    it('returns null when disabled', async () => {
        const { showToast, setToastsEnabled } = await import('../widgets/toast.js');
        setToastsEnabled(false);
        const card = showToast('Nope');
        expect(card).toBeNull();
    });

    it('applies correct border color for level', async () => {
        const { showToast, setToastsEnabled } = await import('../widgets/toast.js');
        setToastsEnabled(true);
        const success = showToast('OK', '', 'success');
        expect(success.style.borderTopColor).not.toBe('');
        const error = showToast('ERR', '', 'error');
        expect(error.style.borderTopColor).not.toBe('');
    });
});
