import { setIcon } from '../icons.js';

// Dialog for adding and editing backlog items.
// Follows the same pattern as settings.js: mount(ctx) → {open, close, init}.

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

function createSelect(id, options, selectedValue = '') {
    const select = el('select', 'form-select');
    select.id = id;
    options.forEach(option => {
        const optionEl = el('option', '', option);
        optionEl.value = option;
        if (option === selectedValue) {
            optionEl.selected = true;
        }
        select.appendChild(optionEl);
    });
    return select;
}

function createInput(id, type, value = '', placeholder = '') {
    const input = el('input', 'form-input');
    input.id = id;
    input.type = type;
    input.value = value;
    input.placeholder = placeholder;
    return input;
}

function createTextarea(id, value = '', placeholder = '') {
    const textarea = el('textarea', 'form-textarea');
    textarea.id = id;
    textarea.value = value;
    textarea.placeholder = placeholder;
    textarea.rows = 4;
    return textarea;
}

export function mountBacklogItemDialog() {
    let isOpen = false;
    let currentItem = null;
    let projectIndex = -1;
    let onSave = null;

    // ---- DOM ----
    const overlay = el('div', 'dialog-overlay');
    overlay.hidden = true;

    const card = el('div', 'dialog-card');

    const header = el('div', 'dialog-header');
    header.appendChild(el('h2', 'dialog-title', 'Backlog Item'));
    const closeBtn = el('button', 'dialog-close');
    closeBtn.title = 'Close dialog';
    closeBtn.setAttribute('aria-label', 'Close dialog');
    setIcon(closeBtn, 'stop');
    closeBtn.addEventListener('click', close);
    header.appendChild(closeBtn);
    card.appendChild(header);

    const form = el('form', 'dialog-form');
    form.noValidate = true;

    // Title field
    const titleInput = createInput('backlog-title', 'text', '', 'Enter item title...');
    titleInput.required = true;
    const titleError = el('span', 'form-error');
    titleError.id = 'backlog-title-error';
    titleError.style.display = 'none';
    const titleRow = formRow('Title *', titleInput);
    titleRow.appendChild(titleError);
    form.appendChild(titleRow);

    // Description field
    const descriptionTextarea = createTextarea('backlog-description', '', 'Enter description (optional)...');
    form.appendChild(formRow('Description', descriptionTextarea));

    // Status field
    const statusSelect = createSelect('backlog-status', ['todo', 'in-progress', 'done'], 'todo');
    form.appendChild(formRow('Status', statusSelect));

    // Priority field
    const prioritySelect = createSelect('backlog-priority', ['low', 'medium', 'high'], 'medium');
    form.appendChild(formRow('Priority', prioritySelect));

    // Buttons
    const buttonRow = el('div', 'dialog-buttons');
    const cancelBtn = el('button', 'btn btn-secondary', 'Cancel');
    cancelBtn.type = 'button';
    cancelBtn.addEventListener('click', close);
    const saveBtn = el('button', 'btn btn-primary', 'Save');
    saveBtn.type = 'submit';
    buttonRow.appendChild(cancelBtn);
    buttonRow.appendChild(saveBtn);
    form.appendChild(buttonRow);

    card.appendChild(form);
    overlay.appendChild(card);

    // ---- Form submission ----
    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const title = titleInput.value.trim();
        if (!title) {
            titleInput.classList.add('error');
            titleError.textContent = 'Title is required';
            titleError.style.display = 'block';
            titleInput.focus();
            return;
        }

        const description = descriptionTextarea.value.trim();
        const status = statusSelect.value;
        const priority = prioritySelect.value;

        try {
            if (currentItem) {
                // Edit existing item
                await onSave(projectIndex, currentItem.id, title, description, status, priority);
            } else {
                // Add new item
                await onSave(projectIndex, null, title, description, status, priority);
            }
            close();
        } catch (error) {
            window.messageDialog.alert({ title: 'Could not save backlog item', message: error.message, trigger: saveBtn });
        }
    });

    // ---- Public API ----
    function open(item = null, projIndex = -1, saveCallback) {
        currentItem = item;
        projectIndex = projIndex;
        onSave = saveCallback;

        // Reset form
        titleInput.classList.remove('error');
        titleError.textContent = '';
        titleError.style.display = 'none';
        
        if (item) {
            // Edit mode
            titleInput.value = item.title || '';
            descriptionTextarea.value = item.description || '';
            statusSelect.value = item.status || 'todo';
            prioritySelect.value = item.priority || 'medium';
            document.querySelector('.dialog-title').textContent = 'Edit Backlog Item';
            saveBtn.textContent = 'Update';
        } else {
            // Add mode
            titleInput.value = '';
            descriptionTextarea.value = '';
            statusSelect.value = 'todo';
            prioritySelect.value = 'medium';
            document.querySelector('.dialog-title').textContent = 'Add Backlog Item';
            saveBtn.textContent = 'Save';
        }

        overlay.hidden = false;
        isOpen = true;
        titleInput.focus();
    }

    function close() {
        overlay.hidden = true;
        isOpen = false;
        currentItem = null;
        projectIndex = -1;
        onSave = null;
    }

    // ---- Keyboard shortcuts ----
    document.addEventListener('keydown', (e) => {
        if (!isOpen) return;
        
        if (e.key === 'Escape') {
            close();
        } else if (e.key === 'Enter' && e.ctrlKey) {
            form.dispatchEvent(new Event('submit'));
        }
    });

    // ---- Click outside to close ----
    overlay.addEventListener('click', (e) => {
        if (e.target === overlay) {
            close();
        }
    });

    return {
        open,
        close,
        getElement: () => overlay
    };
}
