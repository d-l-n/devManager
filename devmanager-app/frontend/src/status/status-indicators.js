/**
 * Enhanced Status Indicators System
 * Provides visual feedback for application states with animations and colors
 */

class StatusIndicatorManager {
    constructor() {
        this.indicators = new Map();
        this.statusTypes = {
            SUCCESS: 'success',
            WARNING: 'warning', 
            ERROR: 'error',
            INFO: 'info',
            LOADING: 'loading',
            IDLE: 'idle',
            RUNNING: 'running',
            STOPPED: 'stopped',
            PENDING: 'pending',
            UNKNOWN: 'unknown'
        };
        
        this.animationTypes = {
            PULSE: 'pulse',
            SPIN: 'spin',
            BOUNCE: 'bounce',
            FADE: 'fade',
            SLIDE: 'slide',
            GLOW: 'glow',
            NONE: 'none'
        };
        
        this.initializeStyles();
        this.bindEvents();
    }

    /**
     * Initialize dynamic styles
     */
    initializeStyles() {
        const style = document.createElement('style');
        style.textContent = this.getIndicatorStyles();
        document.head.appendChild(style);
    }

    /**
     * Bind global events
     */
    bindEvents() {
        // Listen for status changes from backend
        if (window.events) {
            window.events.on('server-status-changed', (data) => {
                this.updateServerStatus(data.projectId, data.status);
            });
            
            window.events.on('process-status-changed', (data) => {
                this.updateProcessStatus(data.processId, data.status);
            });
        }
    }

    /**
     * Create a status indicator
     */
    createIndicator(id, type, options = {}) {
        const indicator = {
            id,
            type,
            status: this.statusTypes.IDLE,
            animation: options.animation || this.animationTypes.NONE,
            size: options.size || 'medium',
            shape: options.shape || 'circle',
            showText: options.showText !== false,
            text: options.text || '',
            tooltip: options.tooltip || '',
            color: options.color || this.getDefaultColor(type),
            element: null,
            container: options.container || document.body
        };

        this.indicators.set(id, indicator);
        this.renderIndicator(indicator);
        
        return indicator;
    }

    /**
     * Update indicator status
     */
    updateStatus(id, status, options = {}) {
        const indicator = this.indicators.get(id);
        if (!indicator) {
            console.warn(`Indicator with id '${id}' not found`);
            return;
        }

        const oldStatus = indicator.status;
        indicator.status = status;
        
        if (options.text !== undefined) {
            indicator.text = options.text;
        }
        
        if (options.tooltip !== undefined) {
            indicator.tooltip = options.tooltip;
        }
        
        if (options.animation !== undefined) {
            indicator.animation = options.animation;
        }
        
        if (options.color !== undefined) {
            indicator.color = options.color;
        }

        this.updateIndicatorElement(indicator, oldStatus);
        this.triggerStatusChange(indicator, oldStatus);
    }

    /**
     * Render indicator element
     */
    renderIndicator(indicator) {
        const element = document.createElement('div');
        element.className = `status-indicator status-indicator-${indicator.type} status-indicator-${indicator.size} status-indicator-${indicator.shape}`;
        element.setAttribute('data-status', indicator.status);
        element.setAttribute('data-indicator-id', indicator.id);
        
        if (indicator.tooltip) {
            element.setAttribute('title', indicator.tooltip);
            element.setAttribute('aria-label', indicator.tooltip);
        }

        // Create inner structure
        element.innerHTML = this.getIndicatorHTML(indicator);
        
        // Add to container
        indicator.container.appendChild(element);
        indicator.element = element;
        
        // Apply initial animations
        this.applyAnimation(indicator);
    }

    /**
     * Get indicator HTML structure
     */
    getIndicatorHTML(indicator) {
        let html = '';
        
        // Status dot
        html += `<div class="status-dot" data-status="${indicator.status}">`;
        
        if (indicator.status === this.statusTypes.LOADING) {
            html += '<div class="status-spinner"></div>';
        } else {
            html += '<div class="status-core"></div>';
        }
        
        html += '</div>';
        
        // Status text
        if (indicator.showText && indicator.text) {
            html += `<div class="status-text">${indicator.text}</div>`;
        }
        
        // Status badge (for additional info)
        if (indicator.showBadge) {
            html += `<div class="status-badge">${indicator.badgeText || ''}</div>`;
        }
        
        return html;
    }

    /**
     * Update indicator element
     */
    updateIndicatorElement(indicator, oldStatus) {
        if (!indicator.element) {
            return;
        }

        const element = indicator.element;
        
        // Update status attribute
        element.setAttribute('data-status', indicator.status);
        
        // Update status dot
        const statusDot = element.querySelector('.status-dot');
        if (statusDot) {
            statusDot.setAttribute('data-status', indicator.status);
        }
        
        // Update text
        if (indicator.showText) {
            const textElement = element.querySelector('.status-text');
            if (textElement) {
                textElement.textContent = indicator.text;
            } else {
                // Add text element if it doesn't exist
                const textDiv = document.createElement('div');
                textDiv.className = 'status-text';
                textDiv.textContent = indicator.text;
                element.appendChild(textDiv);
            }
        }
        
        // Update tooltip
        if (indicator.tooltip) {
            element.setAttribute('title', indicator.tooltip);
            element.setAttribute('aria-label', indicator.tooltip);
        }
        
        // Apply new animation
        this.applyAnimation(indicator);
        
        // Add transition effect
        element.classList.add('status-transitioning');
        setTimeout(() => {
            element.classList.remove('status-transitioning');
        }, 300);
    }

    /**
     * Apply animation to indicator
     */
    applyAnimation(indicator) {
        if (!indicator.element) {
            return;
        }

        const element = indicator.element;
        
        // Remove all animation classes
        element.classList.remove(
            'status-animate-pulse',
            'status-animate-spin',
            'status-animate-bounce',
            'status-animate-fade',
            'status-animate-slide',
            'status-animate-glow'
        );
        
        // Add new animation class
        if (indicator.animation && indicator.animation !== this.animationTypes.NONE) {
            element.classList.add(`status-animate-${indicator.animation}`);
        }
    }

    /**
     * Get default color for status type
     */
    getDefaultColor(type) {
        const colors = {
            [this.statusTypes.SUCCESS]: '#28a745',
            [this.statusTypes.WARNING]: '#ffc107',
            [this.statusTypes.ERROR]: '#dc3545',
            [this.statusTypes.INFO]: '#17a2b8',
            [this.statusTypes.LOADING]: '#007bff',
            [this.statusTypes.IDLE]: '#6c757d',
            [this.statusTypes.RUNNING]: '#28a745',
            [this.statusTypes.STOPPED]: '#6c757d',
            [this.statusTypes.PENDING]: '#ffc107',
            [this.statusTypes.UNKNOWN]: '#6c757d'
        };
        
        return colors[type] || colors[this.statusTypes.UNKNOWN];
    }

    /**
     * Trigger status change event
     */
    triggerStatusChange(indicator, oldStatus) {
        const event = new CustomEvent('statusIndicatorChanged', {
            detail: {
                indicatorId: indicator.id,
                type: indicator.type,
                oldStatus,
                newStatus: indicator.status,
                text: indicator.text,
                tooltip: indicator.tooltip
            }
        });
        
        document.dispatchEvent(event);
    }

    /**
     * Update server status indicator
     */
    updateServerStatus(projectId, status) {
        const indicatorId = `server-${projectId}`;
        let indicator = this.indicators.get(indicatorId);
        
        if (!indicator) {
            // Create new indicator for this server
            const badgeElement = document.querySelector(`#badge-server[data-project-id="${projectId}"]`);
            if (badgeElement) {
                indicator = this.createIndicator(indicatorId, 'server', {
                    container: badgeElement,
                    size: 'small',
                    shape: 'circle',
                    showText: true,
                    animation: status === 'running' ? this.animationTypes.PULSE : this.animationTypes.NONE
                });
            }
        }
        
        if (indicator) {
            const statusMap = {
                'running': this.statusTypes.RUNNING,
                'stopped': this.statusTypes.STOPPED,
                'starting': this.statusTypes.LOADING,
                'stopping': this.statusTypes.LOADING,
                'error': this.statusTypes.ERROR,
                'unknown': this.statusTypes.UNKNOWN
            };
            
            const mappedStatus = statusMap[status] || this.statusTypes.UNKNOWN;
            const statusText = status.charAt(0).toUpperCase() + status.slice(1);
            
            this.updateStatus(indicatorId, mappedStatus, {
                text: statusText,
                tooltip: `Server status: ${statusText}`,
                animation: status === 'running' ? this.animationTypes.PULSE : this.animationTypes.NONE
            });
        }
    }

    /**
     * Update process status indicator
     */
    updateProcessStatus(processId, status) {
        const indicatorId = `process-${processId}`;
        let indicator = this.indicators.get(indicatorId);
        
        if (!indicator) {
            // Create new indicator for this process
            indicator = this.createIndicator(indicatorId, 'process', {
                size: 'small',
                shape: 'circle',
                showText: false,
                animation: this.animationTypes.NONE
            });
        }
        
        if (indicator) {
            const statusMap = {
                'running': this.statusTypes.RUNNING,
                'stopped': this.statusTypes.STOPPED,
                'starting': this.statusTypes.LOADING,
                'stopping': this.statusTypes.LOADING,
                'error': this.statusTypes.ERROR,
                'unknown': this.statusTypes.UNKNOWN
            };
            
            const mappedStatus = statusMap[status] || this.statusTypes.UNKNOWN;
            
            this.updateStatus(indicatorId, mappedStatus, {
                tooltip: `Process status: ${status}`,
                animation: status === 'running' ? this.animationTypes.SPIN : this.animationTypes.NONE
            });
        }
    }

    /**
     * Create project status indicator
     */
    createProjectStatus(projectId, container) {
        const indicatorId = `project-${projectId}`;
        return this.createIndicator(indicatorId, 'project', {
            container,
            size: 'small',
            shape: 'dot',
            showText: false,
            animation: this.animationTypes.NONE
        });
    }

    /**
     * Show temporary status notification
     */
    showNotification(message, type = this.statusTypes.INFO, duration = 3000) {
        const notification = document.createElement('div');
        notification.className = `status-notification status-notification-${type}`;
        notification.innerHTML = `
            <div class="status-notification-content">
                <div class="status-notification-indicator"></div>
                <div class="status-notification-message">${message}</div>
                <button class="status-notification-close" aria-label="Close">&times;</button>
            </div>
        `;
        
        // Add to page
        document.body.appendChild(notification);
        
        // Trigger animation
        setTimeout(() => {
            notification.classList.add('status-notification-show');
        }, 10);
        
        // Auto remove
        const removeNotification = () => {
            notification.classList.remove('status-notification-show');
            setTimeout(() => {
                if (notification.parentNode) {
                    notification.parentNode.removeChild(notification);
                }
            }, 300);
        };
        
        // Close button
        const closeBtn = notification.querySelector('.status-notification-close');
        closeBtn.addEventListener('click', removeNotification);
        
        // Auto remove after duration
        if (duration > 0) {
            setTimeout(removeNotification, duration);
        }
        
        return notification;
    }

    /**
     * Create progress indicator
     */
    createProgressIndicator(id, container, options = {}) {
        const progressContainer = document.createElement('div');
        progressContainer.className = 'status-progress';
        progressContainer.setAttribute('data-indicator-id', id);
        
        progressContainer.innerHTML = `
            <div class="status-progress-bar">
                <div class="status-progress-fill" style="width: ${options.value || 0}%"></div>
            </div>
            ${options.showText !== false ? `
                <div class="status-progress-text">${options.value || 0}%</div>
            ` : ''}
        `;
        
        if (container) {
            container.appendChild(progressContainer);
        }
        
        return {
            element: progressContainer,
            updateValue: (value) => {
                const fill = progressContainer.querySelector('.status-progress-fill');
                const text = progressContainer.querySelector('.status-progress-text');
                
                if (fill) {
                    fill.style.width = `${value}%`;
                }
                
                if (text) {
                    text.textContent = `${value}%`;
                }
            }
        };
    }

    /**
     * Get indicator styles
     */
    getIndicatorStyles() {
        return `
            /* Base indicator styles */
            .status-indicator {
                display: inline-flex;
                align-items: center;
                gap: 6px;
                font-family: var(--font-family, system-ui);
                font-size: var(--font-size, 14px);
                color: var(--text-color, #333);
                position: relative;
            }

            /* Status dot */
            .status-dot {
                position: relative;
                display: inline-flex;
                align-items: center;
                justify-content: center;
                flex-shrink: 0;
            }

            .status-core {
                width: 100%;
                height: 100%;
                border-radius: 50%;
                transition: all 0.3s ease;
            }

            /* Status colors */
            .status-dot[data-status="success"] .status-core {
                background: var(--status-success, #28a745);
                box-shadow: 0 0 0 2px rgba(40, 167, 69, 0.2);
            }

            .status-dot[data-status="warning"] .status-core {
                background: var(--status-warning, #ffc107);
                box-shadow: 0 0 0 2px rgba(255, 193, 7, 0.2);
            }

            .status-dot[data-status="error"] .status-core {
                background: var(--status-error, #dc3545);
                box-shadow: 0 0 0 2px rgba(220, 53, 69, 0.2);
            }

            .status-dot[data-status="info"] .status-core {
                background: var(--status-info, #17a2b8);
                box-shadow: 0 0 0 2px rgba(23, 162, 184, 0.2);
            }

            .status-dot[data-status="loading"] .status-core {
                background: var(--status-loading, #007bff);
                box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.2);
            }

            .status-dot[data-status="idle"] .status-core,
            .status-dot[data-status="stopped"] .status-core {
                background: var(--status-idle, #6c757d);
                box-shadow: 0 0 0 2px rgba(108, 117, 125, 0.2);
            }

            .status-dot[data-status="running"] .status-core {
                background: var(--status-running, #28a745);
                box-shadow: 0 0 0 2px rgba(40, 167, 69, 0.2);
            }

            .status-dot[data-status="pending"] .status-core {
                background: var(--status-pending, #ffc107);
                box-shadow: 0 0 0 2px rgba(255, 193, 7, 0.2);
            }

            .status-dot[data-status="unknown"] .status-core {
                background: var(--status-unknown, #6c757d);
                box-shadow: 0 0 0 2px rgba(108, 117, 125, 0.2);
            }

            /* Size variations */
            .status-indicator-small .status-dot {
                width: 8px;
                height: 8px;
            }

            .status-indicator-medium .status-dot {
                width: 12px;
                height: 12px;
            }

            .status-indicator-large .status-dot {
                width: 16px;
                height: 16px;
            }

            /* Shape variations */
            .status-indicator-circle .status-core {
                border-radius: 50%;
            }

            .status-indicator-square .status-core {
                border-radius: 2px;
            }

            .status-indicator-dot .status-core {
                border-radius: 50%;
                width: 6px;
                height: 6px;
            }

            /* Status text */
            .status-text {
                font-weight: 500;
                font-size: 0.875em;
                color: var(--text-secondary, #666);
            }

            /* Status badge */
            .status-badge {
                background: var(--badge-bg, #e0e0e0);
                color: var(--badge-color, #333);
                font-size: 0.75em;
                font-weight: 600;
                padding: 2px 6px;
                border-radius: 10px;
                margin-left: 4px;
            }

            /* Spinner for loading state */
            .status-spinner {
                width: 100%;
                height: 100%;
                border: 2px solid transparent;
                border-top: 2px solid currentColor;
                border-radius: 50%;
                animation: statusSpin 1s linear infinite;
            }

            /* Animations */
            .status-animate-pulse .status-core {
                animation: statusPulse 2s ease-in-out infinite;
            }

            .status-animate-spin .status-core {
                animation: statusSpin 1s linear infinite;
            }

            .status-animate-bounce .status-core {
                animation: statusBounce 1s ease-in-out infinite;
            }

            .status-animate-fade .status-core {
                animation: statusFade 1.5s ease-in-out infinite alternate;
            }

            .status-animate-glow .status-core {
                animation: statusGlow 2s ease-in-out infinite alternate;
            }

            /* Transition effect */
            .status-transitioning .status-core {
                transition: all 0.3s ease;
            }

            /* Progress indicator */
            .status-progress {
                width: 100%;
                max-width: 200px;
            }

            .status-progress-bar {
                width: 100%;
                height: 4px;
                background: var(--progress-bg, #e0e0e0);
                border-radius: 2px;
                overflow: hidden;
            }

            .status-progress-fill {
                height: 100%;
                background: var(--progress-fill, #007bff);
                transition: width 0.3s ease;
                border-radius: 2px;
            }

            .status-progress-text {
                text-align: center;
                font-size: 0.75em;
                color: var(--text-secondary, #666);
                margin-top: 4px;
            }

            /* Notification styles */
            .status-notification {
                position: fixed;
                top: 20px;
                right: 20px;
                max-width: 400px;
                background: var(--surface-color, #ffffff);
                border: 1px solid var(--border-color, #e0e0e0);
                border-radius: 8px;
                box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
                z-index: 10000;
                transform: translateX(100%);
                opacity: 0;
                transition: all 0.3s ease;
            }

            .status-notification-show {
                transform: translateX(0);
                opacity: 1;
            }

            .status-notification-content {
                display: flex;
                align-items: center;
                padding: 12px 16px;
                gap: 12px;
            }

            .status-notification-indicator {
                width: 12px;
                height: 12px;
                border-radius: 50%;
                flex-shrink: 0;
            }

            .status-notification-success .status-notification-indicator {
                background: var(--status-success, #28a745);
            }

            .status-notification-warning .status-notification-indicator {
                background: var(--status-warning, #ffc107);
            }

            .status-notification-error .status-notification-indicator {
                background: var(--status-error, #dc3545);
            }

            .status-notification-info .status-notification-indicator {
                background: var(--status-info, #17a2b8);
            }

            .status-notification-message {
                flex: 1;
                font-weight: 500;
                color: var(--text-color, #333);
            }

            .status-notification-close {
                background: none;
                border: none;
                font-size: 1.2em;
                color: var(--text-secondary, #666);
                cursor: pointer;
                padding: 0;
                width: 20px;
                height: 20px;
                display: flex;
                align-items: center;
                justify-content: center;
                border-radius: 50%;
                transition: background-color 0.2s ease;
            }

            .status-notification-close:hover {
                background: var(--hover-bg, #f5f5f5);
            }

            /* Keyframe animations */
            @keyframes statusSpin {
                0% { transform: rotate(0deg); }
                100% { transform: rotate(360deg); }
            }

            @keyframes statusPulse {
                0%, 100% { opacity: 1; transform: scale(1); }
                50% { opacity: 0.7; transform: scale(0.9); }
            }

            @keyframes statusBounce {
                0%, 100% { transform: translateY(0); }
                50% { transform: translateY(-20%); }
            }

            @keyframes statusFade {
                0% { opacity: 0.3; }
                100% { opacity: 1; }
            }

            @keyframes statusGlow {
                0% { box-shadow: 0 0 2px currentColor; }
                100% { box-shadow: 0 0 8px currentColor, 0 0 16px currentColor; }
            }

            /* Dark theme support */
            :root[data-theme="dark"] .status-notification {
                background: var(--surface-color, #1e1e1e);
                border-color: var(--border-color, #555);
                box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
            }

            :root[data-theme="dark"] .status-notification-message {
                color: var(--text-color, #ffffff);
            }

            :root[data-theme="dark"] .status-notification-close {
                color: var(--text-secondary, #cccccc);
            }

            :root[data-theme="dark"] .status-notification-close:hover {
                background: var(--hover-bg, #2d2d2d);
            }

            /* Brutalist theme support */
            :root[data-style="brutalist"] .status-dot .status-core {
                border: 2px solid;
                box-shadow: var(--brutalist-shadow-sm);
            }

            :root[data-style="brutalist"] .status-notification {
                border: 2px solid var(--brutalist-border);
                border-radius: 0;
                box-shadow: var(--brutalist-shadow-md);
            }

            :root[data-style="brutalist"] .status-progress-bar {
                border: 2px solid var(--brutalist-border);
                height: 6px;
            }

            :root[data-style="brutalist"] .status-progress-fill {
                border: 2px solid var(--brutalist-accent);
                background: var(--brutalist-accent);
            }

            /* Responsive design */
            @media (max-width: 768px) {
                .status-notification {
                    top: 10px;
                    right: 10px;
                    left: 10px;
                    max-width: none;
                }
            }

            /* High contrast mode */
            @media (prefers-contrast: high) {
                .status-dot .status-core {
                    border: 2px solid;
                }
                
                .status-notification {
                    border-width: 2px;
                }
            }

            /* Reduced motion */
            @media (prefers-reduced-motion: reduce) {
                .status-animate-pulse,
                .status-animate-spin,
                .status-animate-bounce,
                .status-animate-fade,
                .status-animate-glow {
                    animation: none;
                }
                
                .status-transitioning .status-core {
                    transition: none;
                }
            }
        `;
    }

    /**
     * Remove indicator
     */
    removeIndicator(id) {
        const indicator = this.indicators.get(id);
        if (indicator && indicator.element) {
            indicator.element.remove();
        }
        this.indicators.delete(id);
    }

    /**
     * Get all indicators
     */
    getAllIndicators() {
        return Array.from(this.indicators.values());
    }

    /**
     * Get indicators by type
     */
    getIndicatorsByType(type) {
        return Array.from(this.indicators.values()).filter(indicator => indicator.type === type);
    }

    /**
     * Clear all indicators
     */
    clearAllIndicators() {
        this.indicators.forEach((indicator, id) => {
            this.removeIndicator(id);
        });
    }
}

// Create global status indicator manager
const statusIndicatorManager = new StatusIndicatorManager();

export { StatusIndicatorManager, statusIndicatorManager };