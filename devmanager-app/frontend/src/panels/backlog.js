const BACKLOG_STATUSES = ['todo', 'in-progress', 'done'];
const BACKLOG_PRIORITIES = ['low', 'medium', 'high'];

export function mount(ctx) {
    const { $, api, events } = ctx;

    let currentBacklog = [];
    let draggedItem = null;

    function showResult(text, cls) {
        const el = $('backlog-result');
        el.textContent = text;
        el.className = `result-strip ${cls}`;
        el.hidden = false;
        setTimeout(() => { el.hidden = true; }, 3000);
    }

    function render() {
        const container = $('backlog-items');
        container.innerHTML = '';

        if (currentBacklog.length === 0) {
            container.innerHTML = '<div class="empty-state">No backlog items yet. Click "Add Item" to create one.</div>';
            return;
        }

        currentBacklog.forEach((item, index) => {
            const itemEl = document.createElement('div');
            itemEl.className = `backlog-item status-${item.status} priority-${item.priority}`;
            itemEl.draggable = true;
            itemEl.dataset.index = index;
            itemEl.dataset.id = item.id;

            const statusIcon = {
                'todo': '○',
                'in-progress': '◐',
                'done': '●'
            }[item.status] || '○';

            const priorityColor = {
                'low': '#6b7280',
                'medium': '#f59e0b',
                'high': '#ef4444'
            }[item.priority] || '#6b7280';

            itemEl.innerHTML = `
                <div class="backlog-item-header">
                    <div class="backlog-item-title">
                        <span class="status-icon">${statusIcon}</span>
                        <span class="title-text">${escapeHtml(item.title)}</span>
                        <span class="priority-dot" style="background-color: ${priorityColor}"></span>
                    </div>
                    <div class="backlog-item-actions">
                        <button class="btn-small" onclick="editBacklogItem('${item.id}')">Edit</button>
                        <button class="btn-small btn-danger" onclick="deleteBacklogItem('${item.id}')">Delete</button>
                    </div>
                </div>
                ${item.description ? `<div class="backlog-item-description">${escapeHtml(item.description)}</div>` : ''}
                <div class="backlog-item-meta">
                    <span class="status-badge status-${item.status}">${item.status}</span>
                    <span class="priority-badge priority-${item.priority}">${item.priority}</span>
                    <span class="date">Updated ${formatDate(item.updated_at)}</span>
                </div>
            `;

            // Drag and drop events
            itemEl.addEventListener('dragstart', handleDragStart);
            itemEl.addEventListener('dragover', handleDragOver);
            itemEl.addEventListener('drop', handleDrop);
            itemEl.addEventListener('dragend', handleDragEnd);

            container.appendChild(itemEl);
        });
    }

    function handleDragStart(e) {
        draggedItem = {
            id: e.target.dataset.id,
            index: parseInt(e.target.dataset.index)
        };
        e.target.classList.add('dragging');
        e.dataTransfer.effectAllowed = 'move';
    }

    function handleDragOver(e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        
        const afterElement = getDragAfterElement(e.currentTarget.parentNode, e.clientY);
        if (afterElement == null) {
            e.currentTarget.parentNode.appendChild(draggedItem.element);
        } else {
            e.currentTarget.parentNode.insertBefore(draggedItem.element, afterElement);
        }
    }

    function handleDrop(e) {
        e.preventDefault();
        const dropIndex = parseInt(e.target.closest('.backlog-item').dataset.index);
        
        if (draggedItem && draggedItem.index !== dropIndex) {
            moveBacklogItem(draggedItem.id, dropIndex);
        }
    }

    function handleDragEnd(e) {
        e.target.classList.remove('dragging');
        draggedItem = null;
    }

    function getDragAfterElement(container, y) {
        const draggableElements = [...container.querySelectorAll('.backlog-item:not(.dragging)')];
        
        return draggableElements.reduce((closest, child) => {
            const box = child.getBoundingClientRect();
            const offset = y - box.top - box.height / 2;
            
            if (offset < 0 && offset > closest.offset) {
                return { offset: offset, element: child };
            } else {
                return closest;
            }
        }, { offset: Number.NEGATIVE_INFINITY }).element;
    }

    async function refresh() {
        const i = ctx.selectedIndex();
        if (i < 0) {
            currentBacklog = [];
            render();
            return;
        }
        
        try {
            currentBacklog = await api.getBacklog(i);
            render();
        } catch (error) {
            showResult(`Error loading backlog: ${error.message}`, 'err');
        }
    }

    window.addBacklogItem = async () => {
        const i = ctx.selectedIndex();
        if (i < 0) {
            showResult('Please select a project first', 'err');
            return;
        }

        window.backlogItemDialog.open(null, i, async (projectIndex, itemId, title, description, status, priority) => {
            try {
                await api.addBacklogItem(projectIndex, title, description, status, priority);
                showResult('Backlog item added successfully', 'ok');
                refresh();
            } catch (error) {
                showResult(`Error adding item: ${error.message}`, 'err');
            }
        });
    };

    window.editBacklogItem = async (itemId) => {
        const i = ctx.selectedIndex();
        if (i < 0) return;

        const item = currentBacklog.find(item => item.id === itemId);
        if (!item) return;

        window.backlogItemDialog.open(item, i, async (projectIndex, itemId, title, description, status, priority) => {
            try {
                await api.updateBacklogItem(projectIndex, itemId, title, description, status, priority);
                showResult('Backlog item updated successfully', 'ok');
                refresh();
            } catch (error) {
                showResult(`Error updating item: ${error.message}`, 'err');
            }
        });
    };

    window.deleteBacklogItem = async (itemId) => {
        const i = ctx.selectedIndex();
        if (i < 0) return;

        if (!confirm('Are you sure you want to delete this backlog item?')) return;

        try {
            await api.deleteBacklogItem(i, itemId);
            showResult('Backlog item deleted successfully', 'ok');
            refresh();
        } catch (error) {
            showResult(`Error deleting item: ${error.message}`, 'err');
        }
    };

    window.moveBacklogItem = async (itemId, newIndex) => {
        const i = ctx.selectedIndex();
        if (i < 0) return;

        try {
            await api.moveBacklogItem(i, itemId, newIndex);
            refresh();
        } catch (error) {
            showResult(`Error moving item: ${error.message}`, 'err');
            refresh();
        }
    };

    // Event listeners
    $('backlog-add').addEventListener('click', () => addBacklogItem());
    $('backlog-refresh').addEventListener('click', refresh);

    // Listen for backlog changes from other parts of the app
    events().EventsOn('backlog:changed', ({ projectIndex }) => {
        if (projectIndex === ctx.selectedIndex()) {
            refresh();
        }
    });

    // Utility functions
    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    function formatDate(dateString) {
        const date = new Date(dateString);
        const now = new Date();
        const diffMs = now - date;
        const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
        
        if (diffDays === 0) return 'today';
        if (diffDays === 1) return 'yesterday';
        if (diffDays < 7) return `${diffDays} days ago`;
        if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`;
        return date.toLocaleDateString();
    }

    return { 
        onProjectChanged: () => { 
            $('backlog-result').hidden = true; 
            refresh(); 
        }, 
        refresh 
    };
}