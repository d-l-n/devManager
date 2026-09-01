/**
 * Keyboard Shortcuts Manager
 * Provides global keyboard shortcut handling for the application
 */

class ShortcutManager {
    constructor() {
        this.shortcuts = new Map();
        this.globalShortcuts = new Map();
        this.contextualShortcuts = new Map();
        this.isEnabled = true;
        this.currentContext = 'global';
        this.helpVisible = false;
        
        // Default shortcuts
        this.defaultShortcuts = {
            // Project management
            'Ctrl+N': { action: 'newProject', description: 'New Project', category: 'Project' },
            'Ctrl+O': { action: 'openProject', description: 'Open Project', category: 'Project' },
            'Ctrl+Shift+N': { action: 'addProject', description: 'Add Project', category: 'Project' },
            'Ctrl+R': { action: 'reloadProjects', description: 'Reload Projects', category: 'Project' },
            'Delete': { action: 'deleteProject', description: 'Delete Project', category: 'Project' },
            
            // Server control
            'Ctrl+S': { action: 'startServer', description: 'Start Server', category: 'Server' },
            'Ctrl+Shift+S': { action: 'stopServer', description: 'Stop Server', category: 'Server' },
            'F5': { action: 'restartServer', description: 'Restart Server', category: 'Server' },
            
            // Navigation
            'Ctrl+1': { action: 'switchToLogs', description: 'Switch to Logs', category: 'Navigation' },
            'Ctrl+2': { action: 'switchToPlaywright', description: 'Switch to Playwright', category: 'Navigation' },
            'Ctrl+3': { action: 'switchToScripts', description: 'Switch to Scripts', category: 'Navigation' },
            'Ctrl+4': { action: 'switchToGit', description: 'Switch to Git', category: 'Navigation' },
            'Ctrl+5': { action: 'switchToEvidence', description: 'Switch to Evidence', category: 'Navigation' },
            'Ctrl+6': { action: 'switchToBacklog', description: 'Switch to Backlog', category: 'Navigation' },
            'Ctrl+0': { action: 'switchToMonitor', description: 'Switch to Monitor', category: 'Navigation' },
            'Ctrl+9': { action: 'switchToSettings', description: 'Switch to Settings', category: 'Navigation' },
            
            // Tab navigation
            'Ctrl+Tab': { action: 'nextTab', description: 'Next Tab', category: 'Navigation' },
            'Ctrl+Shift+Tab': { action: 'previousTab', description: 'Previous Tab', category: 'Navigation' },
            'Alt+Right': { action: 'nextTab', description: 'Next Tab', category: 'Navigation' },
            'Alt+Left': { action: 'previousTab', description: 'Previous Tab', category: 'Navigation' },
            
            // Search
            'Ctrl+F': { action: 'search', description: 'Search', category: 'Search' },
            'Ctrl+Shift+F': { action: 'globalSearch', description: 'Global Search', category: 'Search' },
            'F3': { action: 'findNext', description: 'Find Next', category: 'Search' },
            'Shift+F3': { action: 'findPrevious', description: 'Find Previous', category: 'Search' },
            
            // Log operations
            'Ctrl+L': { action: 'clearLogs', description: 'Clear Logs', category: 'Logs' },
            'Ctrl+Shift+C': { action: 'copyLogs', description: 'Copy Logs', category: 'Logs' },
            'Ctrl+Shift+E': { action: 'saveLogs', description: 'Save Logs', category: 'Logs' },
            'Ctrl+E': { action: 'toggleErrorsOnly', description: 'Toggle Errors Only', category: 'Logs' },
            'Ctrl+W': { action: 'toggleWordWrap', description: 'Toggle Word Wrap', category: 'Logs' },
            'Ctrl+J': { action: 'toggleFollow', description: 'Toggle Follow', category: 'Logs' },
            
            // Git operations
            'Ctrl+G': { action: 'gitStatus', description: 'Git Status', category: 'Git' },
            'Ctrl+Shift+G': { action: 'gitPull', description: 'Git Pull', category: 'Git' },
            'Ctrl+Shift+P': { action: 'gitPush', description: 'Git Push', category: 'Git' },
            'Ctrl+Shift+M': { action: 'gitCommit', description: 'Git Commit', category: 'Git' },
            
            // Playwright
            'Ctrl+T': { action: 'runTests', description: 'Run Tests', category: 'Testing' },
            'Ctrl+Shift+T': { action: 'runTestsWithCoverage', description: 'Run Tests with Coverage', category: 'Testing' },
            'Ctrl+Shift+U': { action: 'openTestUI', description: 'Open Test UI', category: 'Testing' },
            
            // Application
            'Ctrl+Q': { action: 'quit', description: 'Quit Application', category: 'Application' },
            'Ctrl+,': { action: 'openSettings', description: 'Open Settings', category: 'Application' },
            'F11': { action: 'toggleFullscreen', description: 'Toggle Fullscreen', category: 'Application' },
            'Ctrl+H': { action: 'toggleHelp', description: 'Toggle Help', category: 'Application' },
            'F1': { action: 'showHelp', description: 'Show Help', category: 'Application' },
            
            // Theme
            'Ctrl+Shift+Y': { action: 'cycleTheme', description: 'Cycle Theme', category: 'Appearance' },
            'Ctrl+Shift+D': { action: 'toggleDarkMode', description: 'Toggle Dark Mode', category: 'Appearance' },
            
            // Quick actions
            'Ctrl+Shift+A': { action: 'quickActions', description: 'Quick Actions', category: 'Quick Actions' },
            'Ctrl+Space': { action: 'commandPalette', description: 'Command Palette', category: 'Quick Actions' },
            'Ctrl+P': { action: 'commandPalette', description: 'Command Palette', category: 'Quick Actions' }
        };
        
        this.initializeShortcuts();
        this.bindEvents();
    }

    /**
     * Initialize default shortcuts
     */
    initializeShortcuts() {
        Object.entries(this.defaultShortcuts).forEach(([key, config]) => {
            this.registerShortcut(key, config.action, config.description, config.category);
        });
    }

    /**
     * Register a new shortcut
     */
    registerShortcut(key, action, description = '', category = 'Custom', context = 'global') {
        const shortcut = {
            key,
            action,
            description,
            category,
            context,
            enabled: true
        };

        if (context === 'global') {
            this.globalShortcuts.set(key, shortcut);
        } else {
            if (!this.contextualShortcuts.has(context)) {
                this.contextualShortcuts.set(context, new Map());
            }
            this.contextualShortcuts.get(context).set(key, shortcut);
        }

        this.shortcuts.set(key, shortcut);
        return shortcut;
    }

    /**
     * Unregister a shortcut
     */
    unregisterShortcut(key) {
        this.shortcuts.delete(key);
        this.globalShortcuts.delete(key);
        
        this.contextualShortcuts.forEach((contextMap) => {
            contextMap.delete(key);
        });
    }

    /**
     * Enable/disable shortcuts
     */
    setEnabled(enabled) {
        this.isEnabled = enabled;
    }

    /**
     * Set current context
     */
    setContext(context) {
        this.currentContext = context;
    }

    /**
     * Bind keyboard events
     */
    bindEvents() {
        document.addEventListener('keydown', this.handleKeyDown.bind(this));
        document.addEventListener('keyup', this.handleKeyUp.bind(this));
    }

    /**
     * Handle keydown events
     */
    handleKeyDown(event) {
        if (!this.isEnabled) {
            return;
        }

        // Don't trigger shortcuts when typing in input fields
        if (this.isInputElement(event.target)) {
            // Allow some shortcuts even in inputs (like Ctrl+A, Ctrl+C, etc.)
            const allowedInInputs = [
                'Ctrl+A', 'Ctrl+C', 'Ctrl+V', 'Ctrl+X', 'Ctrl+Z', 'Ctrl+Y',
                'Ctrl+F', 'Ctrl+Shift+F', 'F3', 'Shift+F3'
            ];
            
            const key = this.getKeyString(event);
            if (!allowedInInputs.includes(key)) {
                return;
            }
        }

        const key = this.getKeyString(event);
        const shortcut = this.getShortcutForContext(key);

        if (shortcut && shortcut.enabled) {
            event.preventDefault();
            event.stopPropagation();
            this.executeShortcut(shortcut, event);
        }
    }

    /**
     * Handle keyup events
     */
    handleKeyUp(event) {
        // Handle any keyup logic if needed
    }

    /**
     * Get key string from event
     */
    getKeyString(event) {
        const parts = [];
        
        if (event.ctrlKey) parts.push('Ctrl');
        if (event.altKey) parts.push('Alt');
        if (event.shiftKey) parts.push('Shift');
        if (event.metaKey) parts.push('Meta');

        let key = event.key;
        
        // Handle special keys
        const specialKeys = {
            ' ': 'Space',
            'ArrowUp': 'Up',
            'ArrowDown': 'Down',
            'ArrowLeft': 'Left',
            'ArrowRight': 'Right',
            'Escape': 'Esc'
        };

        if (specialKeys[key]) {
            key = specialKeys[key];
        }

        // For single letters, use uppercase
        if (key.length === 1) {
            key = key.toUpperCase();
        }

        parts.push(key);
        return parts.join('+');
    }

    /**
     * Get shortcut for current context
     */
    getShortcutForContext(key) {
        // First check contextual shortcuts
        if (this.currentContext !== 'global' && this.contextualShortcuts.has(this.currentContext)) {
            const contextShortcut = this.contextualShortcuts.get(this.currentContext).get(key);
            if (contextShortcut) {
                return contextShortcut;
            }
        }

        // Fall back to global shortcuts
        return this.globalShortcuts.get(key);
    }

    /**
     * Check if element is an input element
     */
    isInputElement(element) {
        const inputTypes = ['input', 'textarea', 'select'];
        const contentEditable = element.isContentEditable;
        
        return inputTypes.includes(element.tagName.toLowerCase()) || contentEditable;
    }

    /**
     * Execute a shortcut action
     */
    executeShortcut(shortcut, event) {
        try {
            this.executeAction(shortcut.action, event);
        } catch (error) {
            console.error(`Error executing shortcut ${shortcut.key}:`, error);
        }
    }

    /**
     * Execute an action
     */
    executeAction(action, event = null) {
        const actions = {
            // Project management
            newProject: () => this.createNewProject(),
            openProject: () => this.openProject(),
            addProject: () => this.addProject(),
            reloadProjects: () => this.reloadProjects(),
            deleteProject: () => this.deleteSelectedProject(),

            // Server control
            startServer: () => this.startServer(),
            stopServer: () => this.stopServer(),
            restartServer: () => this.restartServer(),

            // Navigation
            switchToLogs: () => this.switchToTab('logs'),
            switchToPlaywright: () => this.switchToTab('playwright'),
            switchToScripts: () => this.switchToTab('scripts'),
            switchToGit: () => this.switchToTab('git'),
            switchToEvidence: () => this.switchToTab('evidence'),
            switchToBacklog: () => this.switchToTab('backlog'),
            switchToMonitor: () => this.switchToView('monitor'),
            switchToSettings: () => this.switchToView('settings'),
            nextTab: () => this.switchToNextTab(),
            previousTab: () => this.switchToPreviousTab(),

            // Search
            search: () => this.focusSearch(),
            globalSearch: () => this.openGlobalSearch(),
            findNext: () => this.findNext(),
            findPrevious: () => this.findPrevious(),

            // Log operations
            clearLogs: () => this.clearLogs(),
            copyLogs: () => this.copyLogs(),
            saveLogs: () => this.saveLogs(),
            toggleErrorsOnly: () => this.toggleErrorsOnly(),
            toggleWordWrap: () => this.toggleWordWrap(),
            toggleFollow: () => this.toggleFollow(),

            // Git operations
            gitStatus: () => this.gitStatus(),
            gitPull: () => this.gitPull(),
            gitPush: () => this.gitPush(),
            gitCommit: () => this.gitCommit(),

            // Playwright
            runTests: () => this.runTests(),
            runTestsWithCoverage: () => this.runTestsWithCoverage(),
            openTestUI: () => this.openTestUI(),

            // Application
            quit: () => this.quitApplication(),
            openSettings: () => this.openSettings(),
            toggleFullscreen: () => this.toggleFullscreen(),
            toggleHelp: () => this.toggleHelp(),
            showHelp: () => this.showHelp(),

            // Theme
            cycleTheme: () => this.cycleTheme(),
            toggleDarkMode: () => this.toggleDarkMode(),

            // Quick actions
            quickActions: () => this.openQuickActions(),
            commandPalette: () => this.openCommandPalette()
        };

        const actionFn = actions[action];
        if (actionFn) {
            actionFn();
        } else {
            console.warn(`Unknown action: ${action}`);
        }
    }

    // Action implementations
    createNewProject() {
        // Trigger add project button
        document.getElementById('btn-add')?.click();
    }

    openProject() {
        // Focus on project search
        document.getElementById('search')?.focus();
    }

    addProject() {
        document.getElementById('btn-add')?.click();
    }

    reloadProjects() {
        document.getElementById('btn-reload')?.click();
    }

    deleteSelectedProject() {
        // Get selected project and trigger delete
        const selectedProject = document.querySelector('#project-list li.selected');
        if (selectedProject) {
            // Trigger context menu or delete action
            selectedProject.dispatchEvent(new MouseEvent('contextmenu', {
                bubbles: true,
                clientX: selectedProject.offsetLeft + 10,
                clientY: selectedProject.offsetTop + 10
            }));
        }
    }

    startServer() {
        document.getElementById('btn-start')?.click();
    }

    stopServer() {
        document.getElementById('btn-stop')?.click();
    }

    restartServer() {
        document.getElementById('btn-restart')?.click();
    }

    switchToTab(tabName) {
        const tabButton = document.querySelector(`[data-tab="${tabName}"]`);
        if (tabButton) {
            tabButton.click();
        }
    }

    switchToView(viewName) {
        const viewButton = document.getElementById(`view-${viewName}`);
        if (viewButton) {
            viewButton.click();
        }
    }

    switchToNextTab() {
        const activeTab = document.querySelector('.tab.active');
        if (activeTab) {
            const nextTab = activeTab.nextElementSibling;
            if (nextTab && nextTab.classList.contains('tab')) {
                nextTab.click();
            }
        }
    }

    switchToPreviousTab() {
        const activeTab = document.querySelector('.tab.active');
        if (activeTab) {
            const prevTab = activeTab.previousElementSibling;
            if (prevTab && prevTab.classList.contains('tab')) {
                prevTab.click();
            }
        }
    }

    focusSearch() {
        const searchInput = document.getElementById('search');
        if (searchInput) {
            searchInput.focus();
            searchInput.select();
        }
    }

    openGlobalSearch() {
        // Implement global search modal
        this.showGlobalSearchModal();
    }

    findNext() {
        // Implement find next functionality
        this.findInLogs(true);
    }

    findPrevious() {
        // Implement find previous functionality
        this.findInLogs(false);
    }

    clearLogs() {
        document.getElementById('btn-clear-log')?.click();
    }

    copyLogs() {
        document.getElementById('log-copy')?.click();
    }

    saveLogs() {
        document.getElementById('log-save')?.click();
    }

    toggleErrorsOnly() {
        const errorsCheckbox = document.getElementById('errors-only');
        if (errorsCheckbox) {
            errorsCheckbox.checked = !errorsCheckbox.checked;
            errorsCheckbox.dispatchEvent(new Event('change'));
        }
    }

    toggleWordWrap() {
        document.getElementById('log-wrap')?.click();
    }

    toggleFollow() {
        document.getElementById('log-follow')?.click();
    }

    gitStatus() {
        document.getElementById('git-refresh')?.click();
    }

    gitPull() {
        document.getElementById('git-pull')?.click();
    }

    gitPush() {
        // Implement git push
        console.log('Git push not implemented yet');
    }

    gitCommit() {
        // Implement git commit
        console.log('Git commit not implemented yet');
    }

    runTests() {
        document.getElementById('pw-run')?.click();
    }

    runTestsWithCoverage() {
        // Implement tests with coverage
        console.log('Tests with coverage not implemented yet');
    }

    openTestUI() {
        document.getElementById('pw-ui')?.click();
    }

    quitApplication() {
        if (confirm('Are you sure you want to quit devManager?')) {
            // Use Wails API to quit
            if (window.go && window.go.main && window.go.main.AppQuit) {
                window.go.main.AppQuit();
            } else {
                window.close();
            }
        }
    }

    openSettings() {
        document.getElementById('btn-settings')?.click();
    }

    toggleFullscreen() {
        if (!document.fullscreenElement) {
            document.documentElement.requestFullscreen();
        } else {
            document.exitFullscreen();
        }
    }

    toggleHelp() {
        this.showHelpModal();
    }

    showHelp() {
        this.showHelpModal();
    }

    cycleTheme() {
        // Cycle through themes: dark -> light -> oled -> dark
        const currentTheme = document.documentElement.getAttribute('data-theme') || 'dark';
        const themes = ['dark', 'light', 'oled'];
        const currentIndex = themes.indexOf(currentTheme);
        const nextTheme = themes[(currentIndex + 1) % themes.length];
        
        document.documentElement.setAttribute('data-theme', nextTheme);
        
        // Save theme preference
        localStorage.setItem('theme', nextTheme);
    }

    toggleDarkMode() {
        const currentTheme = document.documentElement.getAttribute('data-theme') || 'dark';
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        
        document.documentElement.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);
    }

    openQuickActions() {
        this.showQuickActionsModal();
    }

    openCommandPalette() {
        this.showCommandPalette();
    }

    // Helper methods for modals and advanced features
    showGlobalSearchModal() {
        // Implement global search modal
        console.log('Global search modal not implemented yet');
    }

    findInLogs(forward = true) {
        // Implement find in logs functionality
        console.log('Find in logs not implemented yet');
    }

    showHelpModal() {
        if (this.helpVisible) {
            this.hideHelp();
            return;
        }

        const helpModal = this.createHelpModal();
        document.body.appendChild(helpModal);
        this.helpVisible = true;

        const closeButton = document.getElementById('shortcut-help-close');
        if (closeButton) {
            closeButton.addEventListener('click', () => this.hideHelp());
        }
    }

    hideHelp() {
        const helpModal = document.getElementById('shortcut-help-modal');
        if (helpModal) {
            helpModal.remove();
        }
        this.helpVisible = false;
    }

    createHelpModal() {
        const modal = document.createElement('div');
        modal.id = 'shortcut-help-modal';
        modal.className = 'modal';
        modal.innerHTML = `
            <div class="modal-content">
                <div class="modal-header">
                    <h2>Keyboard Shortcuts</h2>
                    <button class="modal-close" id="shortcut-help-close">&times;</button>
                </div>
                <div class="modal-body">
                    ${this.generateHelpHTML()}
                </div>
            </div>
        `;

        // Add styles
        modal.style.cssText = `
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.5);
            display: flex;
            align-items: center;
            justify-content: center;
            z-index: 10000;
        `;

        return modal;
    }

    generateHelpHTML() {
        const categories = {};
        
        // Group shortcuts by category
        this.shortcuts.forEach(shortcut => {
            if (!categories[shortcut.category]) {
                categories[shortcut.category] = [];
            }
            categories[shortcut.category].push(shortcut);
        });

        let html = '';
        Object.entries(categories).forEach(([category, shortcuts]) => {
            html += `<h3>${category}</h3><table class="shortcut-table">`;
            shortcuts.forEach(shortcut => {
                html += `
                    <tr>
                        <td><kbd>${shortcut.key}</kbd></td>
                        <td>${shortcut.description}</td>
                    </tr>
                `;
            });
            html += '</table>';
        });

        return html;
    }

    showQuickActionsModal() {
        // Implement quick actions modal
        console.log('Quick actions modal not implemented yet');
    }

    showCommandPalette() {
        // Implement command palette
        console.log('Command palette not implemented yet');
    }

    /**
     * Get all shortcuts
     */
    getAllShortcuts() {
        return Array.from(this.shortcuts.values());
    }

    /**
     * Get shortcuts by category
     */
    getShortcutsByCategory(category) {
        return Array.from(this.shortcuts.values()).filter(s => s.category === category);
    }

    /**
     * Export shortcuts configuration
     */
    exportShortcuts() {
        const config = {};
        this.shortcuts.forEach((shortcut, key) => {
            config[key] = {
                action: shortcut.action,
                description: shortcut.description,
                category: shortcut.category,
                enabled: shortcut.enabled
            };
        });
        return config;
    }

    /**
     * Import shortcuts configuration
     */
    importShortcuts(config) {
        Object.entries(config).forEach(([key, shortcutConfig]) => {
            this.registerShortcut(
                key,
                shortcutConfig.action,
                shortcutConfig.description,
                shortcutConfig.category
            );
        });
    }
}

// Create global shortcut manager instance
const shortcutManager = new ShortcutManager();

export { ShortcutManager, shortcutManager };