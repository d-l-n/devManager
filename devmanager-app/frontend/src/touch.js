// Touch support (Issue #69): estilos de targets táctiles 44px, scroll
// momentum y gestos de navegación (swipe entre tabs). Se activan con
// <html data-touch="on"> y el listener de swipe vive en installGestures().

const STYLE_ID = 'touch-styles';
const STYLES = `
/* Targets táctiles: todo botón/chip/tab/seg ≥ 44x44 en modo touch. */
:root[data-touch="on"] button,
:root[data-touch="on"] .tab,
:root[data-touch="on"] .filter-chip,
:root[data-touch="on"] .seg,
:root[data-touch="on"] input[type="checkbox"],
:root[data-touch="on"] input[type="radio"],
:root[data-touch="on"] select,
:root[data-touch="on"] .icon-btn {
    min-height: 44px;
    min-width: 44px;
}
:root[data-touch="on"] .text-input,
:root[data-touch="on"] input[type="text"],
:root[data-touch="on"] input[type="search"],
:root[data-touch="on"] textarea {
    min-height: 44px;
    font-size: 16px; /* evita zoom automático de iOS al foco */
}
/* Scroll con momentum y sin tap highlight fantasma. */
:root[data-touch="on"] * {
    -webkit-tap-highlight-color: transparent;
    touch-action: manipulation;
}
:root[data-touch="on"] #log-output,
:root[data-touch="on"] .panel,
:root[data-touch="on"] #project-list,
:root[data-touch="on"] .tools-list,
:root[data-touch="on"] .backlog-list {
    -webkit-overflow-scrolling: touch;
    overscroll-behavior: contain;
}
/* Cursor no aporta en touch, y el hover queda "pegado" tras un tap. */
:root[data-touch="on"] button:hover {
    filter: none;
}
`;

function ensureStyles() {
    if (typeof document === 'undefined' || document.getElementById(STYLE_ID)) return;
    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = STYLES;
    document.head.appendChild(style);
}

// setTouchMode activa/desactiva el modo táctil (data-touch="on" en <html>).
export function setTouchMode(on) {
    if (typeof document === 'undefined') return;
    ensureStyles();
    if (on) {
        document.documentElement.setAttribute('data-touch', 'on');
        installGestures();
    } else {
        document.documentElement.removeAttribute('data-touch');
    }
}

// isTouchDevice detecta pantalla táctil + pointer coarse (heurística estándar).
export function isTouchDevice() {
    if (typeof window === 'undefined' || !window.matchMedia) return false;
    return window.matchMedia('(pointer: coarse)').matches && 'ontouchstart' in window;
}

// swipeState comparte el estado del gesto entre installs (tests incluidos).
const swipeState = { startX: 0, startY: 0, tracking: false, installed: false };

// onSwipe y onSwipeEnd permiten a la app suscribirse a gestos horizontales.
let swipeHandlers = [];

export function onSwipe(handler) {
    swipeHandlers.push(handler);
    return () => {
        swipeHandlers = swipeHandlers.filter((h) => h !== handler);
    };
}

// installGestures añade (una vez) los listeners de swipe horizontal sobre el
// documento: umbral 64px horizontales y <45° para no pelear con el scroll.
export function installGestures(doc = document) {
    if (!doc || swipeState.installed) return;
    swipeState.installed = true;
    doc.addEventListener('touchstart', (e) => {
        if (e.touches.length !== 1) {
            swipeState.tracking = false;
            return;
        }
        swipeState.startX = e.touches[0].clientX;
        swipeState.startY = e.touches[0].clientY;
        swipeState.tracking = true;
    }, { passive: true });
    doc.addEventListener('touchend', (e) => {
        if (!swipeState.tracking) return;
        swipeState.tracking = false;
        const t = e.changedTouches[0];
        if (!t) return;
        const dx = t.clientX - swipeState.startX;
        const dy = t.clientY - swipeState.startY;
        if (Math.abs(dx) < 64 || Math.abs(dy) > Math.abs(dx) * 0.7) return;
        swipeHandlers.forEach((h) => h(dx > 0 ? 'right' : 'left'));
    }, { passive: true });
}

// resetGestures limpia el estado de instalación (solo tests).
export function resetGestures() {
    swipeState.installed = false;
    swipeHandlers = [];
}
