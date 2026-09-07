import { setIcon } from '../icons.js';

// Dialog de creación de usuario para el proyecto gestionado.
// El comando configurado en user.command recibe los datos via env vars
// DM_USER_* — el password se genera en el cliente (crypto) y NUNCA se
// loguea; viaja solo en el entorno del proceso.
// Patrón widgets: mount() → { open, close, getElement }.

// Alfabeto sin ambiguos (0/O, 1/l/I) para passwords legibles.
const CHARS = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789';

function randomPassword(length = 16) {
    const bytes = new Uint32Array(length);
    crypto.getRandomValues(bytes);
    let out = '';
    for (let i = 0; i < length; i++) out += CHARS[bytes[i] % CHARS.length];
    return out;
}

function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
}

function formRow(labelText, input) {
    const row = el('div', 'form-row');
    const label = el('label', 'form-label', labelText);
    label.htmlFor = input.id;
    row.appendChild(label);
    row.appendChild(input);
    return row;
}

export function mountCreateUserDialog() {
    let isOpen = false;
    let projectIndex = -1;
    let onRun = null;

    // ---- DOM ----
    const overlay = el('div', 'dialog-overlay');
    overlay.hidden = true;
    const card = el('div', 'dialog-card');

    const header = el('div', 'dialog-header');
    header.appendChild(el('h2', 'dialog-title', 'Create User'));
    const closeBtn = el('button', 'dialog-close');
    closeBtn.title = 'Close dialog';
    closeBtn.setAttribute('aria-label', 'Close dialog');
    setIcon(closeBtn, 'stop');
    closeBtn.addEventListener('click', close);
    header.appendChild(closeBtn);
    card.appendChild(header);

    const form = el('form', 'dialog-form');
    form.noValidate = true;

    // Email
    const emailInput = el('input', 'form-input');
    emailInput.id = 'create-user-email';
    emailInput.type = 'email';
    emailInput.required = true;
    emailInput.placeholder = 'ana@example.com';
    emailInput.autocomplete = 'off';
    const emailError = el('span', 'form-error');
    emailError.id = 'create-user-email-error';
    emailError.style.display = 'none';
    const emailRow = formRow('Email *', emailInput);
    emailRow.appendChild(emailError);
    form.appendChild(emailRow);

    // Name
    const nameInput = el('input', 'form-input');
    nameInput.id = 'create-user-name';
    nameInput.type = 'text';
    nameInput.placeholder = 'Ana García';
    nameInput.autocomplete = 'off';
    form.appendChild(formRow('Display Name', nameInput));

    // Password + generate
    const pwdRow = el('div', 'form-row');
    const pwdLabel = el('label', 'form-label', 'Password');
    pwdLabel.htmlFor = 'create-user-password';
    const pwdInput = el('input', 'form-input mono');
    pwdInput.id = 'create-user-password';
    pwdInput.type = 'text'; // visible para copiar; el dialog es una acción de dev
    pwdInput.autocomplete = 'off';
    const pwdBtns = el('div', 'row-gap');
    const btnGenerate = el('button', 'btn btn-accent', 'Generate');
    btnGenerate.type = 'button';
    btnGenerate.addEventListener('click', () => {
        pwdInput.value = randomPassword();
        pwdInput.focus();
    });
    pwdBtns.appendChild(pwdInput);
    pwdBtns.appendChild(btnGenerate);
    pwdRow.appendChild(pwdLabel);
    pwdRow.appendChild(pwdBtns);
    form.appendChild(pwdRow);

    // Role
    const roleInput = el('input', 'form-input');
    roleInput.id = 'create-user-role';
    roleInput.type = 'text';
    roleInput.value = 'user';
    roleInput.placeholder = 'user / admin / editor...';
    roleInput.autocomplete = 'off';
    form.appendChild(formRow('Role', roleInput));

    // Hint del comando configurado
    const hint = el('div', 'form-hint dim');
    hint.id = 'create-user-hint';
    form.appendChild(hint);

    // Raw command echo (no password — se muestra solo en el Logs con env vars)
    const commandInfo = el('div', 'form-hint dim small');
    commandInfo.id = 'create-user-command';
    form.appendChild(commandInfo);

    // Buttons
    const buttonRow = el('div', 'dialog-buttons');
    const cancelBtn = el('button', 'btn btn-secondary', 'Cancel');
    cancelBtn.type = 'button';
    cancelBtn.addEventListener('click', close);
    const runBtn = el('button', 'btn btn-primary', 'Create User');
    runBtn.type = 'submit';
    buttonRow.appendChild(cancelBtn);
    buttonRow.appendChild(runBtn);
    form.appendChild(buttonRow);

    card.appendChild(form);
    overlay.appendChild(card);

    // ---- Submit ----
    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const email = emailInput.value.trim();
        if (!email) {
            emailInput.classList.add('error');
            emailError.textContent = 'Email is required';
            emailError.style.display = 'block';
            emailInput.focus();
            return;
        }
        if (!email.includes('@') || /\s/.test(email)) {
            emailInput.classList.add('error');
            emailError.textContent = 'Enter a valid email address';
            emailError.style.display = 'block';
            emailInput.focus();
            return;
        }
        const password = pwdInput.value || randomPassword();
        const name = nameInput.value.trim();
        const role = roleInput.value.trim() || 'user';
        try {
            const errs = await onRun(projectIndex, email, name, password, role);
            if (errs && errs.length) {
                window.messageDialog.alert({ title: 'Could not create user', message: errs.join('\n'), trigger: runBtn });
                return;
            }
            close();
            window.showToast('Create User', `Creating ${email}… check Logs for output`, 'info');
        } catch (error) {
            window.messageDialog.alert({ title: 'Could not create user', message: error.message, trigger: runBtn });
        }
    });

    // ---- Public API ----
    function open(index, project) {
        projectIndex = index;
        const user = (project && project.user) || { enabled: true, command: '' };

        emailInput.value = '';
        nameInput.value = '';
        pwdInput.value = randomPassword();
        roleInput.value = 'user';
        emailInput.classList.remove('error');
        emailError.textContent = '';
        emailError.style.display = 'none';

        const command = (user.command || '').trim();
        if (!user.enabled) {
            hint.textContent = 'User creation is disabled for this project.';
            runBtn.disabled = true;
        } else if (!command) {
            hint.textContent = 'No create-user command configured — edit the project (User section).';
            runBtn.disabled = true;
        } else {
            hint.textContent = '';
            runBtn.disabled = false;
        }
        commandInfo.textContent = command ? `Command: ${command}` : '';
        commandInfo.title = command;

        overlay.hidden = false;
        isOpen = true;
        emailInput.focus();
    }

    function close() {
        overlay.hidden = true;
        isOpen = false;
        projectIndex = -1;
        onRun = null;
    }

    document.addEventListener('keydown', (e) => {
        if (!isOpen) return;
        if (e.key === 'Escape') close();
        else if (e.key === 'Enter' && e.ctrlKey) form.dispatchEvent(new Event('submit'));
    });
    overlay.addEventListener('click', (e) => {
        if (e.target === overlay) close();
    });

    // reset del error inline al re-tipear
    emailInput.addEventListener('input', () => {
        emailInput.classList.remove('error');
        emailError.style.display = 'none';
    });

    return {
        open,
        close,
        getElement: () => overlay,
        setRunner: (fn) => { onRun = fn; },
    };
}