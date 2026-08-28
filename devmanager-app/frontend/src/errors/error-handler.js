/**
 * Enhanced Error Handling System for Frontend
 * Provides structured error handling, reporting, and user feedback
 */

class ErrorHandler {
    constructor() {
        this.errorCounts = new Map();
        this.errorHistory = [];
        this.maxHistorySize = 100;
        this.subscribers = [];
        
        // Error severity levels
        this.SEVERITY = {
            INFO: 'info',
            WARNING: 'warning', 
            ERROR: 'error',
            CRITICAL: 'critical'
        };

        // Error codes
        this.ERROR_CODES = {
            // Project errors
            PROJECT_NOT_FOUND: 'PROJECT_NOT_FOUND',
            PROJECT_ALREADY_EXISTS: 'PROJECT_ALREADY_EXISTS',
            PROJECT_INVALID_PATH: 'PROJECT_INVALID_PATH',
            PROJECT_LOAD_FAILED: 'PROJECT_LOAD_FAILED',

            // Server errors
            SERVER_START_FAILED: 'SERVER_START_FAILED',
            SERVER_STOP_FAILED: 'SERVER_STOP_FAILED',
            SERVER_NOT_RUNNING: 'SERVER_NOT_RUNNING',
            SERVER_ALREADY_RUNNING: 'SERVER_ALREADY_RUNNING',
            PORT_CONFLICT: 'PORT_CONFLICT',

            // Process errors
            PROCESS_NOT_FOUND: 'PROCESS_NOT_FOUND',
            PROCESS_START_FAILED: 'PROCESS_START_FAILED',
            PROCESS_KILL_FAILED: 'PROCESS_KILL_FAILED',

            // Configuration errors
            CONFIG_NOT_FOUND: 'CONFIG_NOT_FOUND',
            CONFIG_INVALID: 'CONFIG_INVALID',
            CONFIG_SAVE_FAILED: 'CONFIG_SAVE_FAILED',

            // File system errors
            FILE_NOT_FOUND: 'FILE_NOT_FOUND',
            FILE_ACCESS_DENIED: 'FILE_ACCESS_DENIED',
            FILE_CORRUPTED: 'FILE_CORRUPTED',

            // Network errors
            NETWORK_TIMEOUT: 'NETWORK_TIMEOUT',
            NETWORK_CONNECTION_FAILED: 'NETWORK_CONNECTION_FAILED',

            // Validation errors
            VALIDATION_FAILED: 'VALIDATION_FAILED',
            INVALID_INPUT: 'INVALID_INPUT',

            // Internal errors
            INTERNAL_ERROR: 'INTERNAL_ERROR',
            DATABASE_ERROR: 'DATABASE_ERROR',
            TIMEOUT_ERROR: 'TIMEOUT_ERROR'
        };
    }

    /**
     * Create a structured error object
     */
    createError(code, message, details = {}, userMessage = null) {
        const error = {
            code,
            message,
            userMessage: userMessage || this.getDefaultUserMessage(code),
            details,
            timestamp: new Date().toISOString(),
            severity: this.getSeverity(code),
            stack: this.getStackTrace(),
            id: this.generateErrorId()
        };

        return error;
    }

    /**
     * Handle an error
     */
    handle(error, context = {}) {
        const structuredError = this.structureError(error, context);
        
        // Count errors
        this.countError(structuredError);
        
        // Add to history
        this.addToHistory(structuredError);
        
        // Log error
        this.logError(structuredError);
        
        // Notify subscribers
        this.notifySubscribers(structuredError);
        
        // Show user feedback if appropriate
        if (structuredError.userMessage) {
            this.showUserFeedback(structuredError);
        }

        return structuredError;
    }

    /**
     * Structure any error into our format
     */
    structureError(error, context = {}) {
        // If it's already a structured error, enhance it
        if (error.code && error.message) {
            return {
                ...error,
                details: { ...error.details, ...context },
                timestamp: error.timestamp || new Date().toISOString(),
                id: error.id || this.generateErrorId()
            };
        }

        // If it's a standard Error object
        if (error instanceof Error) {
            return this.createError(
                this.ERROR_CODES.INTERNAL_ERROR,
                error.message,
                { 
                    name: error.name,
                    stack: error.stack,
                    ...context 
                }
            );
        }

        // If it's a string
        if (typeof error === 'string') {
            return this.createError(
                this.ERROR_CODES.INTERNAL_ERROR,
                error,
                context
            );
        }

        // Unknown error type
        return this.createError(
            this.ERROR_CODES.INTERNAL_ERROR,
            'Unknown error occurred',
            { originalError: error, ...context }
        );
    }

    /**
     * Get default user-friendly message for error code
     */
    getDefaultUserMessage(code) {
        const messages = {
            [this.ERROR_CODES.PROJECT_NOT_FOUND]: 'The requested project could not be found',
            [this.ERROR_CODES.PROJECT_ALREADY_EXISTS]: 'A project with this name already exists',
            [this.ERROR_CODES.PROJECT_INVALID_PATH]: 'The project path is invalid or inaccessible',
            [this.ERROR_CODES.SERVER_START_FAILED]: 'Could not start the development server',
            [this.ERROR_CODES.SERVER_STOP_FAILED]: 'Could not stop the development server',
            [this.ERROR_CODES.SERVER_NOT_RUNNING]: 'The server is not currently running',
            [this.ERROR_CODES.SERVER_ALREADY_RUNNING]: 'The server is already running',
            [this.ERROR_CODES.PORT_CONFLICT]: 'The specified port is already in use',
            [this.ERROR_CODES.PROCESS_NOT_FOUND]: 'The process could not be found',
            [this.ERROR_CODES.PROCESS_START_FAILED]: 'Failed to start the process',
            [this.ERROR_CODES.PROCESS_KILL_FAILED]: 'Failed to stop the process',
            [this.ERROR_CODES.CONFIG_NOT_FOUND]: 'Configuration file not found',
            [this.ERROR_CODES.CONFIG_INVALID]: 'The configuration is invalid',
            [this.ERROR_CODES.CONFIG_SAVE_FAILED]: 'Could not save the configuration',
            [this.ERROR_CODES.FILE_NOT_FOUND]: 'The requested file was not found',
            [this.ERROR_CODES.FILE_ACCESS_DENIED]: 'Access to the file was denied',
            [this.ERROR_CODES.FILE_CORRUPTED]: 'The file appears to be corrupted',
            [this.ERROR_CODES.NETWORK_TIMEOUT]: 'The operation timed out',
            [this.ERROR_CODES.NETWORK_CONNECTION_FAILED]: 'Could not establish a connection',
            [this.ERROR_CODES.VALIDATION_FAILED]: 'The input validation failed',
            [this.ERROR_CODES.INVALID_INPUT]: 'The provided input is invalid',
            [this.ERROR_CODES.INTERNAL_ERROR]: 'An unexpected error occurred. Please try again',
            [this.ERROR_CODES.DATABASE_ERROR]: 'A database error occurred',
            [this.ERROR_CODES.TIMEOUT_ERROR]: 'The operation timed out'
        };

        return messages[code] || 'An error occurred';
    }

    /**
     * Get severity level for error code
     */
    getSeverity(code) {
        const severityMap = {
            [this.ERROR_CODES.PROJECT_NOT_FOUND]: this.SEVERITY.WARNING,
            [this.ERROR_CODES.PROJECT_INVALID_PATH]: this.SEVERITY.WARNING,
            [this.ERROR_CODES.FILE_NOT_FOUND]: this.SEVERITY.WARNING,
            [this.ERROR_CODES.SERVER_START_FAILED]: this.SEVERITY.ERROR,
            [this.ERROR_CODES.SERVER_STOP_FAILED]: this.SEVERITY.ERROR,
            [this.ERROR_CODES.PROCESS_START_FAILED]: this.SEVERITY.ERROR,
            [this.ERROR_CODES.CONFIG_SAVE_FAILED]: this.SEVERITY.ERROR,
            [this.ERROR_CODES.INTERNAL_ERROR]: this.SEVERITY.CRITICAL,
            [this.ERROR_CODES.DATABASE_ERROR]: this.SEVERITY.CRITICAL
        };

        return severityMap[code] || this.SEVERITY.INFO;
    }

    /**
     * Generate unique error ID
     */
    generateErrorId() {
        return `err_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    }

    /**
     * Get current stack trace
     */
    getStackTrace() {
        const stack = new Error().stack;
        return stack ? stack.split('\n').slice(2) : [];
    }

    /**
     * Count error occurrences
     */
    countError(error) {
        const count = this.errorCounts.get(error.code) || 0;
        this.errorCounts.set(error.code, count + 1);
    }

    /**
     * Add error to history
     */
    addToHistory(error) {
        this.errorHistory.unshift(error);
        if (this.errorHistory.length > this.maxHistorySize) {
            this.errorHistory = this.errorHistory.slice(0, this.maxHistorySize);
        }
    }

    /**
     * Log error to console
     */
    logError(error) {
        const logMethod = this.getLogMethod(error.severity);
        logMethod.call(console, `[${error.code}] ${error.message}`, error);
        
        if (error.details && Object.keys(error.details).length > 0) {
            console.log('Error details:', error.details);
        }
    }

    /**
     * Get appropriate console log method
     */
    getLogMethod(severity) {
        switch (severity) {
            case this.SEVERITY.CRITICAL:
                return console.error;
            case this.SEVERITY.ERROR:
                return console.error;
            case this.SEVERITY.WARNING:
                return console.warn;
            default:
                return console.log;
        }
    }

    /**
     * Show user feedback (toast, notification, etc.)
     */
    showUserFeedback(error) {
        // Import showToast dynamically to avoid circular dependencies
        import('../widgets/toast.js').then(({ showToast }) => {
            const toastType = this.getToastType(error.severity);
            showToast(error.userMessage, toastType);
        }).catch(() => {
            // Fallback to alert if toast module not available
            console.warn('Toast module not available, using fallback');
            this.fallbackUserMessage(error);
        });
    }

    /**
     * Get toast type based on severity
     */
    getToastType(severity) {
        switch (severity) {
            case this.SEVERITY.CRITICAL:
                return 'error';
            case this.SEVERITY.ERROR:
                return 'error';
            case this.SEVERITY.WARNING:
                return 'warning';
            default:
                return 'info';
        }
    }

    /**
     * Fallback user message method
     */
    fallbackUserMessage(error) {
        // Create a simple notification if toast is not available
        const notification = document.createElement('div');
        notification.className = `error-notification ${error.severity}`;
        notification.textContent = error.userMessage;
        notification.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 12px 20px;
            border-radius: 4px;
            color: white;
            font-weight: bold;
            z-index: 10000;
            max-width: 300px;
            word-wrap: break-word;
        `;

        // Set background color based on severity
        const colors = {
            [this.SEVERITY.CRITICAL]: '#dc3545',
            [this.SEVERITY.ERROR]: '#dc3545',
            [this.SEVERITY.WARNING]: '#ffc107',
            [this.SEVERITY.INFO]: '#17a2b8'
        };
        notification.style.backgroundColor = colors[error.severity] || '#17a2b8';

        document.body.appendChild(notification);

        // Remove after 5 seconds
        setTimeout(() => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        }, 5000);
    }

    /**
     * Subscribe to error events
     */
    subscribe(callback) {
        this.subscribers.push(callback);
        
        // Return unsubscribe function
        return () => {
            const index = this.subscribers.indexOf(callback);
            if (index > -1) {
                this.subscribers.splice(index, 1);
            }
        };
    }

    /**
     * Notify all subscribers
     */
    notifySubscribers(error) {
        this.subscribers.forEach(callback => {
            try {
                callback(error);
            } catch (err) {
                console.error('Error in subscriber callback:', err);
            }
        });
    }

    /**
     * Get error statistics
     */
    getErrorStats() {
        const stats = {};
        this.errorCounts.forEach((count, code) => {
            stats[code] = count;
        });
        return stats;
    }

    /**
     * Get recent errors
     */
    getRecentErrors(limit = 10) {
        return this.errorHistory.slice(0, limit);
    }

    /**
     * Clear error history
     */
    clearHistory() {
        this.errorHistory = [];
        this.errorCounts.clear();
    }

    /**
     * Safe execute function with error handling
     */
    safeExecute(fn, context = {}) {
        try {
            return fn();
        } catch (error) {
            this.handle(error, context);
            throw error;
        }
    }

    /**
     * Safe async execute function with error handling
     */
    async safeExecuteAsync(fn, context = {}) {
        try {
            return await fn();
        } catch (error) {
            this.handle(error, context);
            throw error;
        }
    }
}

// Create global error handler instance
const globalErrorHandler = new ErrorHandler();

// Set up global error handlers
window.addEventListener('error', (event) => {
    globalErrorHandler.handle(event.error, {
        filename: event.filename,
        lineno: event.lineno,
        colno: event.colno,
        type: 'javascript_error'
    });
});

window.addEventListener('unhandledrejection', (event) => {
    globalErrorHandler.handle(event.reason, {
        type: 'unhandled_promise_rejection'
    });
});

// Convenience functions for common error types
const createProjectError = (code, projectData, details = {}) => {
    return globalErrorHandler.createError(code, '', { 
        project: projectData, 
        ...details 
    });
};

const createServerError = (code, serverData, details = {}) => {
    return globalErrorHandler.createError(code, '', { 
        server: serverData, 
        ...details 
    });
};

const createValidationError = (field, value, rule) => {
    return globalErrorHandler.createError(
        globalErrorHandler.ERROR_CODES.VALIDATION_FAILED,
        '',
        { field, value, rule }
    );
};

export {
    ErrorHandler,
    globalErrorHandler,
    createProjectError,
    createServerError,
    createValidationError
};