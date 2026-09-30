// i18n (Issue #69): traducción de strings de UI, cambio de idioma, RTL y
// formateo locale-aware de fechas/números. Sin dependencias: diccionarios
// planos + helper t(). Idiomas soportados: en, es, ar (RTL).
//
// Alcance MVP: chrome de la app (navegación, settings, ayuda, toasts, tabs y
// acciones comunes). Los textos dinámicos del backend siguen en inglés.

export const LANGUAGES = [
    { code: 'en', label: 'English', dir: 'ltr' },
    { code: 'es', label: 'Español', dir: 'ltr' },
    { code: 'ar', label: 'العربية', dir: 'rtl' },
];

const en = {
    'app.title': 'Local Dev Manager',
    'sidebar.search': 'Search projects...',
    'sidebar.add': 'Add project',
    'filter.all': 'All',
    'filter.running': 'Running',
    'filter.stopped': 'Stopped',
    'view.project': 'Project',
    'view.monitor': 'Monitor',
    'view.dashboard': 'Dashboard',
    'action.reload': 'Reload projects',
    'action.settings': 'Settings',
    'action.help': 'Help',
    'action.quit': 'Quit',
    'action.start': 'Start',
    'action.stop': 'Stop',
    'action.restart': 'Restart',
    'action.save': 'Save',
    'action.close': 'Close',
    'action.cancel': 'Cancel',
    'action.delete': 'Delete',
    'action.edit': 'Edit',
    'action.run': 'Run',
    'action.refresh': 'Refresh',
    'tab.logs': 'Logs',
    'tab.scripts': 'Scripts',
    'tab.git': 'Git',
    'tab.deps': 'Deps',
    'tab.playwright': 'Playwright',
    'tab.testing': 'Testing',
    'tab.evidence': 'Evidence',
    'tab.obscura': 'Obscura',
    'tab.backlog': 'Backlog',
    'tab.workflows': 'Workflows',
    'tab.tools': 'Tools',
    'settings.title': 'Settings',
    'settings.appearance': 'Appearance',
    'settings.style': 'Visual Style',
    'settings.display': 'Display',
    'settings.notifications': 'Notifications',
    'settings.monitoring': 'Monitoring',
    'settings.accent': 'Accent Color',
    'settings.about': 'About',
    'settings.language': 'Language',
    'settings.language.desc': 'Interface language and region formatting',
    'settings.touch': 'Touch mode',
    'settings.touch.desc': 'Larger touch targets (44px) and gesture navigation for tablets',
    'settings.theme': 'Theme',
    'empty.select': 'Select or add a project',
    'monitor.auto': 'Auto-refresh',
    'help.title': 'Help & documentation',
    'toast.saved': 'Saved',
    'toast.error': 'Error',
};

const es = {
    'app.title': 'Gestor de Desarrollo Local',
    'sidebar.search': 'Buscar proyectos...',
    'sidebar.add': 'Añadir proyecto',
    'filter.all': 'Todos',
    'filter.running': 'Corriendo',
    'filter.stopped': 'Parados',
    'view.project': 'Proyecto',
    'view.monitor': 'Monitor',
    'view.dashboard': 'Panel',
    'action.reload': 'Recargar proyectos',
    'action.settings': 'Ajustes',
    'action.help': 'Ayuda',
    'action.quit': 'Salir',
    'action.start': 'Iniciar',
    'action.stop': 'Parar',
    'action.restart': 'Reiniciar',
    'action.save': 'Guardar',
    'action.close': 'Cerrar',
    'action.cancel': 'Cancelar',
    'action.delete': 'Borrar',
    'action.edit': 'Editar',
    'action.run': 'Ejecutar',
    'action.refresh': 'Actualizar',
    'tab.logs': 'Logs',
    'tab.scripts': 'Scripts',
    'tab.git': 'Git',
    'tab.deps': 'Deps',
    'tab.playwright': 'Playwright',
    'tab.testing': 'Testing',
    'tab.evidence': 'Evidencias',
    'tab.obscura': 'Obscura',
    'tab.backlog': 'Backlog',
    'tab.workflows': 'Workflows',
    'tab.tools': 'Herramientas',
    'settings.title': 'Ajustes',
    'settings.appearance': 'Apariencia',
    'settings.style': 'Estilo visual',
    'settings.display': 'Pantalla',
    'settings.notifications': 'Notificaciones',
    'settings.monitoring': 'Monitoreo',
    'settings.accent': 'Color de acento',
    'settings.about': 'Acerca de',
    'settings.language': 'Idioma',
    'settings.language.desc': 'Idioma de la interfaz y formato regional',
    'settings.touch': 'Modo táctil',
    'settings.touch.desc': 'Botones más grandes (44px) y navegación por gestos para tablets',
    'settings.theme': 'Tema',
    'empty.select': 'Selecciona o añade un proyecto',
    'monitor.auto': 'Auto-refresco',
    'help.title': 'Ayuda y documentación',
    'toast.saved': 'Guardado',
    'toast.error': 'Error',
};

const ar = {
    'app.title': 'مدير التطوير المحلي',
    'sidebar.search': 'ابحث في المشاريع...',
    'sidebar.add': 'إضافة مشروع',
    'filter.all': 'الكل',
    'filter.running': 'يعمل',
    'filter.stopped': 'متوقف',
    'view.project': 'المشروع',
    'view.monitor': 'المراقب',
    'view.dashboard': 'اللوحة',
    'action.reload': 'تحديث المشاريع',
    'action.settings': 'الإعدادات',
    'action.help': 'المساعدة',
    'action.quit': 'خروج',
    'action.start': 'تشغيل',
    'action.stop': 'إيقاف',
    'action.restart': 'إعادة تشغيل',
    'action.save': 'حفظ',
    'action.close': 'إغلاق',
    'action.cancel': 'إلغاء',
    'action.delete': 'حذف',
    'action.edit': 'تحرير',
    'action.run': 'تشغيل',
    'action.refresh': 'تحديث',
    'tab.logs': 'السجلات',
    'tab.scripts': 'السكربتات',
    'tab.git': 'غِت',
    'tab.deps': 'الحزم',
    'tab.playwright': 'بلاي رايت',
    'tab.testing': 'الاختبارات',
    'tab.evidence': 'الأدلة',
    'tab.obscura': 'أوبسكورا',
    'tab.backlog': 'المهام',
    'tab.workflows': 'سير العمل',
    'tab.tools': 'الأدوات',
    'settings.title': 'الإعدادات',
    'settings.appearance': 'المظهر',
    'settings.style': 'النمط البصري',
    'settings.display': 'العرض',
    'settings.notifications': 'الإشعارات',
    'settings.monitoring': 'المراقبة',
    'settings.accent': 'لون التمييز',
    'settings.about': 'حول',
    'settings.language': 'اللغة',
    'settings.language.desc': 'لغة الواجهة وتنسيق المنطقة',
    'settings.touch': 'وضع اللمس',
    'settings.touch.desc': 'أزرار أكبر (44px) وتنقل بالإيماءات للأجهزة اللوحية',
    'settings.theme': 'السمة',
    'empty.select': 'اختر مشروعاً أو أضف واحداً',
    'monitor.auto': 'تحديث تلقائي',
    'help.title': 'المساعدة والوثائق',
    'toast.saved': 'تم الحفظ',
    'toast.error': 'خطأ',
};

const DICTS = { en, es, ar };

let current = 'en';

// ValidLanguages devuelve los códigos soportados.
export function validLanguages() {
    return LANGUAGES.map((l) => l.code);
}

// isValidLanguage reporta si code está soportado.
export function isValidLanguage(code) {
    return code in DICTS;
}

// getLanguage devuelve el idioma activo.
export function getLanguage() {
    return current;
}

// setLanguage cambia el idioma y aplica dir/lang al documento. Devuelve el
// idioma efectivo (inválido → 'en' sin cambio de dir erróneo).
export function setLanguage(code) {
    if (!isValidLanguage(code)) return current;
    current = code;
    if (typeof document !== 'undefined') {
        document.documentElement.lang = code;
        document.documentElement.dir = dirFor(code);
        document.documentElement.setAttribute('data-lang', code);
    }
    return current;
}

// dirFor devuelve 'rtl' para idiomas right-to-left, 'ltr' en otro caso.
export function dirFor(code) {
    return code === 'ar' ? 'rtl' : 'ltr';
}

// isRTL reporta si el idioma activo es RTL.
export function isRTL(code) {
    return dirFor(code || current) === 'rtl';
}

// t traduce una clave con sustitución {var}. Clave ausente → fallback en;
// ausente también allí → la propia clave (paridad i18next).
export function t(key, vars) {
    let s = DICTS[current]?.[key];
    if (s === undefined) s = DICTS.en[key];
    if (s === undefined) return key;
    if (vars) {
        for (const [k, v] of Object.entries(vars)) {
            s = s.replaceAll(`{${k}}`, String(v));
        }
    }
    return s;
}

// tElement aplica t() a todos los nodos con data-i18n (textContent) y
// data-i18n-placeholder / data-i18n-title para atributos. Idempotente.
export function applyTranslations(root = document) {
    if (!root || typeof root.querySelectorAll !== 'function') return;
    root.querySelectorAll('[data-i18n]').forEach((el) => {
        el.textContent = t(el.getAttribute('data-i18n'));
    });
    root.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
        el.setAttribute('placeholder', t(el.getAttribute('data-i18n-placeholder')));
    });
    root.querySelectorAll('[data-i18n-title]').forEach((el) => {
        el.setAttribute('title', t(el.getAttribute('data-i18n-title')));
        const aria = el.getAttribute('aria-label');
        if (aria !== null) el.setAttribute('aria-label', t(el.getAttribute('data-i18n-title')));
    });
}

// ---- Formateo locale-aware ----

const LOCALES = { en: 'en-US', es: 'es-ES', ar: 'ar-EG' };

function localeFor(code) {
    return LOCALES[code || current] || 'en-US';
}

// formatDate formatea una fecha según el idioma (default: 13 dic 2025).
export function formatDate(value, opts) {
    const d = value instanceof Date ? value : new Date(value);
    if (Number.isNaN(d.getTime())) return String(value ?? '');
    return d.toLocaleDateString(localeFor(), opts || { dateStyle: 'medium' });
}

// formatTime formatea hora corta según el idioma.
export function formatTime(value) {
    const d = value instanceof Date ? value : new Date(value);
    if (Number.isNaN(d.getTime())) return String(value ?? '');
    return d.toLocaleTimeString(localeFor(), { hour: '2-digit', minute: '2-digit' });
}

// formatNumber formatea números con separadores del locale.
export function formatNumber(value) {
    const n = Number(value);
    if (!Number.isFinite(n)) return String(value ?? '');
    return n.toLocaleString(localeFor());
}

// formatBytes formatea tamaños con unidades localizadas.
export function formatBytes(value) {
    const n = Number(value);
    if (!Number.isFinite(n)) return String(value ?? '');
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let i = 0;
    let v = n;
    while (v >= 1024 && i < units.length - 1) {
        v /= 1024;
        i++;
    }
    return `${new Intl.NumberFormat(localeFor(), { maximumFractionDigits: 1 }).format(v)} ${units[i]}`;
}
