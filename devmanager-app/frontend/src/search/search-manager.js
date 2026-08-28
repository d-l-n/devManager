/**
 * Global Search Manager
 * Provides comprehensive search functionality across the entire application
 */

class SearchManager {
    constructor() {
        this.searchIndex = new Map();
        this.searchHistory = [];
        this.maxHistorySize = 50;
        this.isSearchModalOpen = false;
        this.currentQuery = '';
        this.currentResults = [];
        this.selectedResultIndex = 0;
        
        // Search providers
        this.providers = new Map();
        this.initializeProviders();
        
        // Search categories
        this.categories = {
            projects: { name: 'Projects', icon: '📁', priority: 1 },
            logs: { name: 'Logs', icon: '📋', priority: 2 },
            settings: { name: 'Settings', icon: '⚙️', priority: 3 },
            commands: { name: 'Commands', icon: '⚡', priority: 4 },
            help: { name: 'Help', icon: '❓', priority: 5 }
        };
        
        this.bindEvents();
    }

    /**
     * Initialize search providers
     */
    initializeProviders() {
        // Projects provider
        this.providers.set('projects', {
            name: 'Projects',
            search: (query) => this.searchProjects(query),
            getItem: (id) => this.getProject(id),
            getDetails: (item) => this.getProjectDetails(item)
        });

        // Logs provider
        this.providers.set('logs', {
            name: 'Logs',
            search: (query) => this.searchLogs(query),
            getItem: (id) => this.getLogEntry(id),
            getDetails: (item) => this.getLogDetails(item)
        });

        // Settings provider
        this.providers.set('settings', {
            name: 'Settings',
            search: (query) => this.searchSettings(query),
            getItem: (id) => this.getSetting(id),
            getDetails: (item) => this.getSettingDetails(item)
        });

        // Commands provider
        this.providers.set('commands', {
            name: 'Commands',
            search: (query) => this.searchCommands(query),
            getItem: (id) => this.getCommand(id),
            getDetails: (item) => this.getCommandDetails(item)
        });

        // Help provider
        this.providers.set('help', {
            name: 'Help',
            search: (query) => this.searchHelp(query),
            getItem: (id) => this.getHelpItem(id),
            getDetails: (item) => this.getHelpDetails(item)
        });
    }

    /**
     * Bind keyboard events
     */
    bindEvents() {
        // Global search shortcut (Ctrl+Shift+F)
        document.addEventListener('keydown', (e) => {
            if ((e.ctrlKey && e.shiftKey && e.key === 'F') || 
                (e.ctrlKey && e.key === 'p')) {
                e.preventDefault();
                this.openSearchModal();
            }
        });

        // Escape to close search
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && this.isSearchModalOpen) {
                this.closeSearchModal();
            }
        });
    }

    /**
     * Open search modal
     */
    openSearchModal() {
        if (this.isSearchModalOpen) {
            this.closeSearchModal();
            return;
        }

        const modal = this.createSearchModal();
        document.body.appendChild(modal);
        this.isSearchModalOpen = true;

        // Focus input after a short delay
        setTimeout(() => {
            const input = document.getElementById('global-search-input');
            if (input) {
                input.focus();
                input.select();
            }
        }, 100);
    }

    /**
     * Close search modal
     */
    closeSearchModal() {
        const modal = document.getElementById('global-search-modal');
        if (modal) {
            modal.remove();
        }
        this.isSearchModalOpen = false;
        this.currentQuery = '';
        this.currentResults = [];
        this.selectedResultIndex = 0;
    }

    /**
     * Create search modal
     */
    createSearchModal() {
        const modal = document.createElement('div');
        modal.id = 'global-search-modal';
        modal.className = 'search-modal';
        modal.innerHTML = `
            <div class="search-modal-content">
                <div class="search-input-container">
                    <input 
                        type="text" 
                        id="global-search-input" 
                        class="search-input" 
                        placeholder="Search projects, logs, settings, commands..."
                        autocomplete="off"
                    >
                    <div class="search-shortcut">Ctrl+Shift+F</div>
                </div>
                <div class="search-results-container">
                    <div id="search-results" class="search-results"></div>
                    <div id="search-empty" class="search-empty" style="display: none;">
                        <div class="search-empty-icon">🔍</div>
                        <div class="search-empty-text">No results found</div>
                        <div class="search-empty-hint">Try different keywords or check spelling</div>
                    </div>
                </div>
                <div class="search-footer">
                    <div class="search-stats">
                        <span id="search-result-count">0</span> results
                    </div>
                    <div class="search-tips">
                        <kbd>↑↓</kbd> Navigate • <kbd>Enter</kbd> Select • <kbd>Esc</kbd> Close
                    </div>
                </div>
            </div>
        `;

        // Bind events
        this.bindSearchEvents(modal);
        return modal;
    }

    /**
     * Bind search modal events
     */
    bindSearchEvents(modal) {
        const input = modal.querySelector('#global-search-input');
        const resultsContainer = modal.querySelector('#search-results');

        // Input event
        input.addEventListener('input', (e) => {
            this.handleSearchInput(e.target.value);
        });

        // Keyboard navigation
        input.addEventListener('keydown', (e) => {
            this.handleSearchKeydown(e);
        });

        // Click on results
        resultsContainer.addEventListener('click', (e) => {
            const resultItem = e.target.closest('.search-result-item');
            if (resultItem) {
                const resultId = resultItem.dataset.resultId;
                this.selectResult(resultId);
            }
        });
    }

    /**
     * Handle search input
     */
    handleSearchInput(query) {
        this.currentQuery = query.trim();
        
        if (this.currentQuery.length === 0) {
            this.showEmptyState();
            return;
        }

        // Add to search history
        this.addToSearchHistory(this.currentQuery);

        // Perform search
        this.performSearch(this.currentQuery);
    }

    /**
     * Handle keyboard navigation in search
     */
    handleSearchKeydown(e) {
        const results = this.currentResults;
        
        switch (e.key) {
            case 'ArrowDown':
                e.preventDefault();
                this.selectedResultIndex = Math.min(this.selectedResultIndex + 1, results.length - 1);
                this.updateResultSelection();
                break;
                
            case 'ArrowUp':
                e.preventDefault();
                this.selectedResultIndex = Math.max(this.selectedResultIndex - 1, 0);
                this.updateResultSelection();
                break;
                
            case 'Enter':
                e.preventDefault();
                if (results.length > 0 && this.selectedResultIndex >= 0) {
                    const result = results[this.selectedResultIndex];
                    this.selectResult(result.id);
                }
                break;
                
            case 'Escape':
                e.preventDefault();
                this.closeSearchModal();
                break;
        }
    }

    /**
     * Perform global search
     */
    async performSearch(query) {
        const allResults = [];
        const searchPromises = [];

        // Search all providers
        this.providers.forEach((provider, providerId) => {
            const promise = provider.search(query).then(results => {
                return results.map(result => ({
                    ...result,
                    provider: providerId,
                    providerName: provider.name
                }));
            });
            searchPromises.push(promise);
        });

        try {
            const providerResults = await Promise.all(searchPromises);
            providerResults.forEach(results => {
                allResults.push(...results);
            });

            // Sort results by relevance and priority
            this.currentResults = this.sortResults(allResults, query);
            this.displayResults(this.currentResults);
            
        } catch (error) {
            console.error('Search error:', error);
            this.showErrorState();
        }
    }

    /**
     * Sort search results
     */
    sortResults(results, query) {
        return results.sort((a, b) => {
            // Priority by category
            const aPriority = this.categories[a.category]?.priority || 999;
            const bPriority = this.categories[b.category]?.priority || 999;
            
            if (aPriority !== bPriority) {
                return aPriority - bPriority;
            }
            
            // Then by relevance score
            return (b.relevance || 0) - (a.relevance || 0);
        });
    }

    /**
     * Display search results
     */
    displayResults(results) {
        const resultsContainer = document.getElementById('search-results');
        const emptyState = document.getElementById('search-empty');
        const resultCount = document.getElementById('search-result-count');

        if (results.length === 0) {
            this.showEmptyState();
            return;
        }

        // Hide empty state
        emptyState.style.display = 'none';
        resultsContainer.style.display = 'block';

        // Update result count
        resultCount.textContent = results.length;

        // Group results by category
        const groupedResults = this.groupResultsByCategory(results);

        // Generate HTML
        let html = '';
        Object.entries(groupedResults).forEach(([category, categoryResults]) => {
            const categoryInfo = this.categories[category];
            html += `
                <div class="search-category">
                    <div class="search-category-header">
                        <span class="search-category-icon">${categoryInfo.icon}</span>
                        <span class="search-category-name">${categoryInfo.name}</span>
                        <span class="search-category-count">${categoryResults.length}</span>
                    </div>
                    <div class="search-category-results">
            `;

            categoryResults.forEach((result, index) => {
                const resultIndex = results.indexOf(result);
                const isSelected = resultIndex === this.selectedResultIndex;
                html += this.createResultItemHTML(result, isSelected);
            });

            html += `
                    </div>
                </div>
            `;
        });

        resultsContainer.innerHTML = html;
    }

    /**
     * Group results by category
     */
    groupResultsByCategory(results) {
        const grouped = {};
        results.forEach(result => {
            if (!grouped[result.category]) {
                grouped[result.category] = [];
            }
            grouped[result.category].push(result);
        });
        return grouped;
    }

    /**
     * Create HTML for a result item
     */
    createResultItemHTML(result, isSelected = false) {
        const categoryInfo = this.categories[result.category];
        const selectedClass = isSelected ? 'selected' : '';
        
        return `
            <div class="search-result-item ${selectedClass}" data-result-id="${result.id}">
                <div class="search-result-icon">${categoryInfo.icon}</div>
                <div class="search-result-content">
                    <div class="search-result-title">${this.highlightQuery(result.title)}</div>
                    <div class="search-result-description">${this.highlightQuery(result.description || '')}</div>
                    ${result.metadata ? this.createMetadataHTML(result.metadata) : ''}
                </div>
                <div class="search-result-action">
                    ${result.action ? `<kbd>${result.action}</kbd>` : ''}
                </div>
            </div>
        `;
    }

    /**
     * Create metadata HTML
     */
    createMetadataHTML(metadata) {
        if (!metadata || Object.keys(metadata).length === 0) {
            return '';
        }

        const items = Object.entries(metadata).map(([key, value]) => {
            return `<span class="search-metadata-item">${key}: ${value}</span>`;
        });

        return `<div class="search-result-metadata">${items.join(' • ')}</div>`;
    }

    /**
     * Highlight query in text
     */
    highlightQuery(text) {
        if (!this.currentQuery || !text) {
            return text;
        }

        const regex = new RegExp(`(${this.escapeRegex(this.currentQuery)})`, 'gi');
        return text.replace(regex, '<mark>$1</mark>');
    }

    /**
     * Escape regex special characters
     */
    escapeRegex(string) {
        return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }

    /**
     * Update result selection
     */
    updateResultSelection() {
        const resultItems = document.querySelectorAll('.search-result-item');
        
        resultItems.forEach((item, index) => {
            const result = this.currentResults[index];
            if (result && index === this.selectedResultIndex) {
                item.classList.add('selected');
                // Scroll into view if needed
                item.scrollIntoView({ block: 'nearest' });
            } else {
                item.classList.remove('selected');
            }
        });
    }

    /**
     * Select a result
     */
    selectResult(resultId) {
        const result = this.currentResults.find(r => r.id === resultId);
        if (!result) {
            return;
        }

        const provider = this.providers.get(result.provider);
        if (provider && provider.getItem) {
            const item = provider.getItem(result.itemId);
            if (item) {
                this.executeResultAction(result, item);
            }
        }

        this.closeSearchModal();
    }

    /**
     * Execute action for selected result
     */
    executeResultAction(result, item) {
        switch (result.category) {
            case 'projects':
                this.selectProject(item);
                break;
            case 'logs':
                this.jumpToLogEntry(item);
                break;
            case 'settings':
                this.openSetting(item);
                break;
            case 'commands':
                this.executeCommand(item);
                break;
            case 'help':
                this.openHelpItem(item);
                break;
        }
    }

    /**
     * Show empty state
     */
    showEmptyState() {
        const resultsContainer = document.getElementById('search-results');
        const emptyState = document.getElementById('search-empty');
        const resultCount = document.getElementById('search-result-count');

        resultsContainer.style.display = 'none';
        emptyState.style.display = 'flex';
        resultCount.textContent = '0';
    }

    /**
     * Show error state
     */
    showErrorState() {
        const resultsContainer = document.getElementById('search-results');
        const emptyState = document.getElementById('search-empty');

        resultsContainer.style.display = 'none';
        emptyState.style.display = 'flex';
        emptyState.querySelector('.search-empty-text').textContent = 'Search Error';
        emptyState.querySelector('.search-empty-hint').textContent = 'Please try again later';
    }

    /**
     * Add to search history
     */
    addToSearchHistory(query) {
        if (query.length < 2) {
            return;
        }

        // Remove existing entry
        this.searchHistory = this.searchHistory.filter(item => item !== query);
        
        // Add to beginning
        this.searchHistory.unshift(query);
        
        // Limit size
        if (this.searchHistory.length > this.maxHistorySize) {
            this.searchHistory = this.searchHistory.slice(0, this.maxHistorySize);
        }

        // Save to localStorage
        this.saveSearchHistory();
    }

    /**
     * Save search history
     */
    saveSearchHistory() {
        try {
            localStorage.setItem('devmanager-search-history', JSON.stringify(this.searchHistory));
        } catch (error) {
            console.warn('Could not save search history:', error);
        }
    }

    /**
     * Load search history
     */
    loadSearchHistory() {
        try {
            const saved = localStorage.getItem('devmanager-search-history');
            if (saved) {
                this.searchHistory = JSON.parse(saved);
            }
        } catch (error) {
            console.warn('Could not load search history:', error);
        }
    }

    // Search provider implementations
    async searchProjects(query) {
        const projects = window.state?.projects || [];
        const results = [];
        
        projects.forEach((project, index) => {
            const relevance = this.calculateRelevance(query, [project.name, project.path, project.description].join(' '));
            
            if (relevance > 0) {
                results.push({
                    id: `project-${index}`,
                    itemId: index,
                    category: 'projects',
                    title: project.name,
                    description: project.path,
                    relevance,
                    metadata: {
                        type: project.type || 'unknown',
                        status: project.status || 'unknown'
                    },
                    action: 'Select'
                });
            }
        });

        return results;
    }

    async searchLogs(query) {
        const results = [];
        const logOutput = document.getElementById('log-output');
        
        if (!logOutput) {
            return results;
        }

        const logLines = logOutput.textContent.split('\n');
        
        logLines.forEach((line, index) => {
            const relevance = this.calculateRelevance(query, line);
            
            if (relevance > 0) {
                results.push({
                    id: `log-${index}`,
                    itemId: index,
                    category: 'logs',
                    title: `Line ${index + 1}`,
                    description: line.substring(0, 100) + (line.length > 100 ? '...' : ''),
                    relevance,
                    metadata: {
                        line: index + 1,
                        isError: line.toLowerCase().includes('error')
                    },
                    action: 'Jump to'
                });
            }
        });

        // Limit log results
        return results.slice(0, 20);
    }

    async searchSettings(query) {
        const results = [];
        const settings = [
            { key: 'theme', name: 'Theme', description: 'Change application theme' },
            { key: 'style', name: 'Style', description: 'Change visual style' },
            { key: 'autoStart', name: 'Auto Start', description: 'Auto-start servers' },
            { key: 'logLevel', name: 'Log Level', description: 'Set logging verbosity' },
            { key: 'shortcuts', name: 'Keyboard Shortcuts', description: 'Configure shortcuts' }
        ];

        settings.forEach(setting => {
            const relevance = this.calculateRelevance(query, [setting.name, setting.description, setting.key].join(' '));
            
            if (relevance > 0) {
                results.push({
                    id: `setting-${setting.key}`,
                    itemId: setting.key,
                    category: 'settings',
                    title: setting.name,
                    description: setting.description,
                    relevance,
                    action: 'Open'
                });
            }
        });

        return results;
    }

    async searchCommands(query) {
        const results = [];
        
        if (window.shortcutManager) {
            const shortcuts = window.shortcutManager.getAllShortcuts();
            
            shortcuts.forEach(shortcut => {
                const relevance = this.calculateRelevance(query, [shortcut.action, shortcut.description].join(' '));
                
                if (relevance > 0) {
                    results.push({
                        id: `command-${shortcut.action}`,
                        itemId: shortcut.action,
                        category: 'commands',
                        title: shortcut.description,
                        description: `Execute: ${shortcut.action}`,
                        relevance,
                        metadata: {
                            shortcut: shortcut.key,
                            category: shortcut.category
                        },
                        action: shortcut.key
                    });
                }
            });
        }

        return results;
    }

    async searchHelp(query) {
        const results = [];
        const helpTopics = [
            { id: 'getting-started', title: 'Getting Started', description: 'Learn the basics of devManager' },
            { id: 'shortcuts', title: 'Keyboard Shortcuts', description: 'View all available shortcuts' },
            { id: 'troubleshooting', title: 'Troubleshooting', description: 'Common issues and solutions' },
            { id: 'api-reference', title: 'API Reference', description: 'Developer documentation' }
        ];

        helpTopics.forEach(topic => {
            const relevance = this.calculateRelevance(query, [topic.title, topic.description].join(' '));
            
            if (relevance > 0) {
                results.push({
                    id: `help-${topic.id}`,
                    itemId: topic.id,
                    category: 'help',
                    title: topic.title,
                    description: topic.description,
                    relevance,
                    action: 'Open'
                });
            }
        });

        return results;
    }

    /**
     * Calculate relevance score for search
     */
    calculateRelevance(query, text) {
        if (!query || !text) {
            return 0;
        }

        const queryLower = query.toLowerCase();
        const textLower = text.toLowerCase();
        
        let score = 0;
        
        // Exact match gets highest score
        if (textLower === queryLower) {
            return 100;
        }
        
        // Starts with query gets high score
        if (textLower.startsWith(queryLower)) {
            score += 50;
        }
        
        // Contains query gets medium score
        if (textLower.includes(queryLower)) {
            score += 25;
        }
        
        // Word matches
        const queryWords = queryLower.split(' ');
        const textWords = textLower.split(' ');
        
        queryWords.forEach(queryWord => {
            textWords.forEach(textWord => {
                if (textWord === queryWord) {
                    score += 10;
                } else if (textWord.includes(queryWord)) {
                    score += 5;
                }
            });
        });
        
        return score;
    }

    // Action implementations
    selectProject(project) {
        // Select the project in the UI
        const projectList = document.getElementById('project-list');
        const projectItems = projectList.querySelectorAll('li');
        
        if (projectItems[project.index]) {
            projectItems[project.index].click();
        }
    }

    jumpToLogEntry(logEntry) {
        // Switch to logs tab and scroll to line
        const logsTab = document.querySelector('[data-tab="logs"]');
        if (logsTab) {
            logsTab.click();
        }
        
        setTimeout(() => {
            const logOutput = document.getElementById('log-output');
            if (logOutput) {
                const lines = logOutput.textContent.split('\n');
                const targetLine = Math.min(logEntry.line, lines.length - 1);
                
                // Simple scroll to approximate position
                const scrollPosition = (targetLine / lines.length) * logOutput.scrollHeight;
                logOutput.scrollTop = scrollPosition;
            }
        }, 100);
    }

    openSetting(setting) {
        // Open settings and navigate to specific setting
        const settingsButton = document.getElementById('btn-settings');
        if (settingsButton) {
            settingsButton.click();
        }
        
        // Focus on specific setting after a delay
        setTimeout(() => {
            const settingElement = document.querySelector(`[data-setting="${setting}"]`);
            if (settingElement) {
                settingElement.scrollIntoView({ behavior: 'smooth' });
                settingElement.focus();
            }
        }, 200);
    }

    executeCommand(command) {
        // Execute the command via shortcut manager
        if (window.shortcutManager) {
            window.shortcutManager.executeAction(command);
        }
    }

    openHelpItem(helpItem) {
        // Open help documentation
        if (helpItem.id === 'shortcuts') {
            if (window.shortcutManager) {
                window.shortcutManager.showHelpModal();
            }
        } else {
            // Open other help topics
            window.open(`/help/${helpItem.id}`, '_blank');
        }
    }

    // Provider helper methods
    getProject(index) {
        const projects = window.state?.projects || [];
        return projects[index];
    }

    getProjectDetails(project) {
        return {
            name: project.name,
            path: project.path,
            type: project.type,
            status: project.status
        };
    }

    getLogEntry(index) {
        const logOutput = document.getElementById('log-output');
        if (!logOutput) {
            return null;
        }
        
        const lines = logOutput.textContent.split('\n');
        return {
            line: index + 1,
            content: lines[index] || ''
        };
    }

    getLogDetails(logEntry) {
        return {
            line: logEntry.line,
            content: logEntry.content,
            isError: logEntry.content.toLowerCase().includes('error')
        };
    }

    getSetting(key) {
        return { key };
    }

    getSettingDetails(setting) {
        return { key: setting.key };
    }

    getCommand(action) {
        return { action };
    }

    getCommandDetails(command) {
        return { action: command.action };
    }

    getHelpItem(id) {
        return { id };
    }

    getHelpDetails(helpItem) {
        return { id: helpItem.id };
    }
}

// Create global search manager instance
const searchManager = new SearchManager();

// Load search history on initialization
searchManager.loadSearchHistory();

export { SearchManager, searchManager };