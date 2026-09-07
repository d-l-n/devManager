import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Valida el flujo del dialog de creación de usuario: validación de email y
// submit con password generada vía crypto. window.* (messageDialog/showToast)
// se stubean; el dialog solo los usa en paths de error/success.
describe('create-user dialog', () => {
    let dialog = null;

    beforeEach(() => {
        window.messageDialog = { alert: vi.fn() };
        window.showToast = vi.fn();
    });

    afterEach(() => {
        if (dialog) {
            dialog.close();
            dialog.getElement().remove();
            dialog = null;
        }
    });

    async function openDialog() {
        const { mountCreateUserDialog } = await import('../dialogs/create-user.js');
        const d = mountCreateUserDialog();
        document.body.appendChild(d.getElement());
        dialog = d;
        return d;
    }

    // Queries acotadas al overlay del dialog (evita colisiones de IDs entre dialogs).
    const $in = (d, id) => d.getElement().querySelector(`#${id}`);
    const submitBtn = (d) => [...d.getElement().querySelectorAll('.dialog-buttons button')]
        .find((b) => b.type === 'submit');

    function dialogSubmit(d) {
        d.getElement().querySelector('form').dispatchEvent(new Event('submit', { cancelable: true }));
    }

    function tick() {
        return new Promise((resolve) => setTimeout(resolve, 0));
    }

    it('email inválido no llama al runner', async () => {
        const d = await openDialog();
        const onCreate = vi.fn().mockResolvedValue([]);
        d.setRunner(onCreate);
        d.open(0, { user: { enabled: true, command: 'npm run create-user' } });

        $in(d, 'create-user-email').value = 'no-es-email';
        $in(d, 'create-user-role').value = 'admin';

        dialogSubmit(d);
        await tick();

        expect(onCreate).not.toHaveBeenCalled();
        expect($in(d, 'create-user-email').classList.contains('error')).toBe(true);
    });

    it('submit válido llama al runner con password generada y rol default', async () => {
        const d = await openDialog();
        const onCreate = vi.fn().mockResolvedValue([]);
        d.setRunner(onCreate);
        d.open(0, { user: { enabled: true, command: 'npm run create-user' } });

        $in(d, 'create-user-email').value = 'ana@example.com';
        $in(d, 'create-user-name').value = 'Ana';
        $in(d, 'create-user-password').value = ''; // forzar generación
        $in(d, 'create-user-role').value = '';

        dialogSubmit(d);
        await tick();

        expect(onCreate).toHaveBeenCalledTimes(1);
        const [index, email, name, password, role] = onCreate.mock.calls[0];
        expect(index).toBe(0);
        expect(email).toBe('ana@example.com');
        expect(name).toBe('Ana');
        expect(password.length).toBeGreaterThanOrEqual(8);
        expect(role).toBe('user');
    });

    it('comando no configurado deshabilita el botón submit', async () => {
        const d = await openDialog();
        d.open(0, { user: { enabled: true, command: '' } });
        expect(submitBtn(d).disabled).toBe(true);
    });
});