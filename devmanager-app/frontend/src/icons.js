import {
    Add,
    AlertTriangle,
    ArrowDown,
    ArrowUp,
    Bookmark,
    BookmarkCheck,
    CheckCircle,
    Code,
    Command,
    Edit,
    FileText,
    Folder,
    Globe,
    List,
    ListCheck,
    Moon,
    Monitor,
    Notification,
    Palette,
    Play,
    PowerOff,
    Refresh,
    Restart,
    Settings,
    Stop,
    Sun,
    Trash,
} from 'reicon';
import { default as logoSvg } from './icons/logo.svg?raw';

const ICONS = {
    add: Add,
    alert: AlertTriangle,
    browser: Globe,
    check: CheckCircle,
    code: Code,
    command: Command,
    edit: Edit,
    file: FileText,
    folder: Folder,
    list: List,
    pin: Bookmark,
    pinned: BookmarkCheck,
    play: Play,
    'power-off': PowerOff,
    refresh: Refresh,
    restart: Restart,
    settings: Settings,
    stop: Stop,
    sun: Sun,
    moon: Moon,
    tests: ListCheck,
    trash: Trash,
    up: ArrowUp,
    down: ArrowDown,
    monitor: Monitor,
    notification: Notification,
    palette: Palette,
    logo: logoSvg,
};

export function icon(name, { label = '', size = 16 } = {}) {
    const factory = ICONS[name];
    if (!factory) throw new Error(`Unknown Reicon icon: ${name}`);
    
    // Handle SVG imports (like logo) differently from Reicon functions
    if (name === 'logo') {
        const svg = document.createElement('div');
        svg.innerHTML = factory;
        svg.firstChild.setAttribute('width', size);
        svg.firstChild.setAttribute('height', size);
        svg.firstChild.setAttribute('class', 'reicon-icon');
        if (label) {
            svg.firstChild.setAttribute('aria-label', label);
            svg.firstChild.setAttribute('role', 'img');
        } else {
            svg.firstChild.setAttribute('aria-hidden', 'true');
            svg.firstChild.setAttribute('focusable', 'false');
        }
        return svg.firstChild;
    }
    
    const svg = factory({
        size,
        color: 'currentColor',
        weight: 'Outline',
        className: 'reicon-icon',
        attrs: label ? { 'aria-label': label, role: 'img' } : { 'aria-hidden': 'true', focusable: 'false' },
    });
    return svg;
}

export function setIcon(target, name, options) {
    target.replaceChildren(icon(name, options));
}

export function hydrateIcons(root = document) {
    root.querySelectorAll('[data-icon]').forEach((target) => {
        const label = target.getAttribute('aria-label') || '';
        setIcon(target, target.dataset.icon, { label });
    });
}
