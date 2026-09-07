// Mock del bridge Wails para E2E (Issue #46): inyecta window.go.main.App y
// window.runtime ANTES de que main.js cargue (boot() corre al importar el
// módulo, así que el mock debe existir primero vía addInitScript).
//
// Mantiene estado mutable (proyectos + estados de servidor) para simular al
// backend Go: startServer/stopServer mutan y emiten "server:state", igual que
// el backend real. Los métodos que los specs no necesitan devuelven defaults
// seguros; métodos no existentes exponen un stub que lanza para detectar
// llamadas no mockeadas en vez de fallar con TypeError críptico.
//
// Uso desde un spec:
//   import { install, seedProjects } from './helpers/wails-mock.js';
//   test.beforeEach(async ({ page }) => {
//       await install(page, { projects: seedProjects() });
//       await page.goto('/');
//   });

const SEED_PROJECT = (name, port, enabled = true) => ({
    name,
    path: `C:/dev/${name.toLowerCase()}`,
    server: {
        enabled,
        command: 'npm run dev',
        port,
        url: `http://localhost:${port}`,
        startup_timeout: 15000,
    },
    playwright: { enabled: true, command: 'npx playwright test', ui_command: 'npx playwright test --ui', debug_command: 'npx playwright test --debug', report_command: 'npx playwright show-report' },
    user: { enabled: false, command: '' },
    tabs: { hidden: [], order: [] },
    pinned: false,
    backlog: [],
});

// Proyectos semilla: dos stopped, uno "running" para probar filtros/badges.
export function seedProjects() {
    return [
        SEED_PROJECT('Alpha', 3000),
        SEED_PROJECT('Beta', 3001),
        SEED_PROJECT('Gamma', 3002),
    ];
}

export async function install(page, { projects = seedProjects() } = {}) {
    await page.addInitScript(({ seedProjects }) => {
        // ---- Estado mutable del "backend" ----
        const state = {
            projects: JSON.parse(JSON.stringify(seedProjects)),
            serverStates: new Map(), // index -> 'stopped' | 'running' | 'starting'
            settings: {
                theme: 'dark', style: 'standard', monitor_polling: true,
                toasts_enabled: true, accent_overrides: {}, accent_global: false,
                accent_global_color: '', backup_frequency: 'off', backup_retention: 20,
            },
            nextId: 1,
        };
        // Registro de llamadas a la API para asserts de los specs.
        const calls = [];
        window.__mockCalls = calls;
        const record = (method, args) => calls.push([method, ...args]);
        // Gamma arranca "running" para el spec de filtros/badges.
        state.serverStates.set(2, 'running');

        const clone = (v) => JSON.parse(JSON.stringify(v));
        const okIndex = (i) => i >= 0 && i < state.projects.length;

        // ---- window.runtime: EventsOn captura callbacks para dispararlos ----
        const listeners = new Map(); // event -> Set<callback>
        window.runtime = {
            EventsOnMultiple: (event, cb, _max) => {
                if (!listeners.has(event)) listeners.set(event, new Set());
                listeners.get(event).add(cb);
                // Wails devuelve un cancelador; la app no lo usa.
                return () => listeners.get(event)?.delete(cb);
            },
            EventsOn: (event, cb) => window.runtime.EventsOnMultiple(event, cb, -1),
            EventsOnce: (event, cb) => window.runtime.EventsOnMultiple(event, cb, 1),
            EventsOff: (event) => listeners.delete(event),
            WindowShow: () => {}, WindowHide: () => {}, WindowUnminimise: () => {},
        };
        // Helper global para specs: emitir eventos "desde Go".
        window.__emitGo = (event, ...args) => {
            (listeners.get(event) || []).forEach((cb) => { try { cb(...args); } catch (e) { console.error('[mock emit]', event, e); } });
        };
        // Acceso de specs al estado interno (debug/asserts).
        window.__mockState = state;

        const emit = (event, payload) => window.__emitGo(event, payload);

        const baseStatus = (i) => ({
            state: state.serverStates.get(i) || 'stopped',
            activePort: state.projects[i]?.server?.port || 0,
            activeUrl: state.projects[i]?.server?.url || '',
            uptimeSeconds: (state.serverStates.get(i) === 'running') ? 120 : 0,
            failureReason: '',
            running: state.serverStates.get(i) === 'running',
        });

        // ---- window.go.main.App ----
        const app = {
            // Proyectos
            GetProjects: () => clone(state.projects),
            AddProject: (p) => {
                state.projects.push(clone(p));
                emit('projects:changed', {});
                return [];
            },
            UpdateProject: (i, p) => {
                if (!okIndex(i)) return ['index out of range'];
                state.projects[i] = clone(p);
                emit('projects:changed', {});
                return [];
            },
            RemoveProject: (i) => {
                if (!okIndex(i)) return ['index out of range'];
                state.projects.splice(i, 1);
                emit('projects:changed', {});
                return [];
            },
            TogglePin: (i) => {
                if (!okIndex(i)) return ['index out of range'];
                state.projects[i].pinned = !state.projects[i].pinned;
                emit('projects:changed', {});
                return [];
            },
            ReloadProjects: () => emit('projects:changed', {}),
            // Servidores
            StartServer: (i) => {
                state.serverStates.set(i, 'running');
                emit('server:state', { index: i });
            },
            StopServer: (i) => {
                state.serverStates.set(i, 'stopped');
                emit('server:state', { index: i });
            },
            RestartServer: (i) => {
                state.serverStates.set(i, 'running');
                emit('server:state', { index: i });
            },
            GetServerStatus: (i) => (okIndex(i) ? baseStatus(i) : baseStatus(0)),
            // Playwright / scripts / git / deps / monitor / evidence: defaults inertes
            RunTests: () => {}, RunUI: () => {}, RunDebug: () => {}, ShowReport: () => {},
            StopPlaywright: () => {},
            GetPlaywrightStatus: () => ({ state: 'off' }),
            GetScripts: () => [],
            RunScript: () => {}, StopScript: () => {},
            GetScriptStatus: () => ({ running: false, activeName: '' }),
            GetGitStatus: () => ({ isRepo: true, branch: 'main', isDirty: false, error: '', ahead: 0, behind: 0, hasUpstream: false }),
            GitAction: () => {}, GetGitDiff: () => [], GitBranches: () => [],
            GitCreateBranch: () => {}, GitRenameBranch: () => {}, GitDeleteBranch: () => {},
            GitCheckout: () => {}, GitTags: () => [], GitCreateTag: () => {},
            GitDeleteTag: () => {}, GitPushTag: () => {},
            GetDeps: () => ({ manager: '', deps: [] }),
            GetDepsAudit: () => ({ manager: '', vulns: [] }),
            GetMonitorData: () => ({
                portRows: [],
                resRows: [{ name: 'Gamma', pid: 111, children: 2, cpu: 12.5, rss: 45 }],
            }),
            // Historial 24h: Alpha corriendo 2h, Gamma 1h, Beta sin datos.
            GetDashboardHistory: () => {
                const nowSec = Math.floor(Date.now() / 1000);
                const series = (fromSec, running) => {
                    const arr = [];
                    for (let t = nowSec - fromSec; t <= nowSec; t += 60) {
                        arr.push({ ts: t, running, uptime_sec: running ? nowSec - t : 0 });
                    }
                    return arr;
                };
                return [
                    { name: 'Alpha', samples: series(2 * 3600, true) },
                    { name: 'Beta', samples: [] },
                    { name: 'Gamma', samples: series(3600, true) },
                ];
            },
            KillTree: () => {},
            GetEvidence: () => [],
            GetEvidenceThumbnail: () => '',
            OpenTraceViewer: () => {}, OpenHTMLReport: () => {},
            OpenExternally: () => {}, OpenContainingFolder: () => {},
            OpenInExplorer: () => {}, OpenTerminal: () => {}, OpenVSCode: () => {}, OpenOpenCode: () => {},
            DetectProjectConfig: () => null,
            DetectUserCommand: () => '',
            BrowseFolder: () => '', BrowseWorkspaceFolder: () => '',
            DiscoverProjects: () => [],
            GetAppLog: () => [], ClearAppLog: () => {},
            // Settings (SetSetting valida como el Go real para reusar el flujo)
            GetSettings: () => clone(state.settings),
            SetSetting: (key, value) => {
                const s = state.settings;
                switch (key) {
                    case 'style':
                        if (!['standard', 'brutalist', 'glassmorphism', 'retro', 'dracula'].includes(value)) {
                            return ['Invalid style value (expected standard, brutalist, glassmorphism, retro or dracula)'];
                        }
                        s.style = value; break;
                    case 'theme':
                        if (!['light', 'dark', 'oled', 'system'].includes(value)) {
                            return ['Invalid theme value (expected light, dark, oled or system)'];
                        }
                        s.theme = value; break;
                    case 'monitor_polling':
                    case 'toasts_enabled':
                    case 'accent_global':
                        s[key] = value === 'true'; break;
                    case 'accent_global_color':
                        s[key] = value; break;
                    case 'backup_frequency':
                        if (!['off', 'hourly', '6h', 'daily', 'weekly'].includes(value)) {
                            return ['Invalid backup_frequency value (expected off, hourly, 6h, daily or weekly)'];
                        }
                        s[key] = value; break;
                    default:
                        if (key.startsWith('accent_override.')) {
                            const style = key.slice('accent_override.'.length);
                            if (value === '' || value === 'default') delete s.accent_overrides[style];
                            else s.accent_overrides[style] = value;
                        } else {
                            return ['Unknown setting key: ' + key];
                        }
                }
                emit('settings:changed', { key, value });
                return [];
            },
            // Backups (Issue #71): catálogo vacío en tests
            CreateBackup: () => ({ ok: true, filename: 'backup-test.dmbak', message: 'Backup created: backup-test.dmbak' }),
            ListBackups: () => [],
            ValidateBackup: () => ({ ok: true, message: 'Backup is valid' }),
            RestoreBackup: () => [],
            OpenBackupsFolder: () => {},
            // Misc
            AutoAssignPorts: () => 0,
            SaveDetectedPort: () => [],
            OpenURL: () => {},
            RestartApp: () => {},
            Quit: () => {},
            GetVersion: () => 'v2.1.0',
            CheckForUpdate: () => ({
                currentVersion: 'v2.1.0', latestVersion: 'v2.1.0',
                updateUrl: '', downloadUrl: '', releaseNotes: '', isUpToDate: true,
            }),
            GetProjectFeatures: () => ({ hasPackageManager: true, hasEvidenceFiles: true }),
            GetBacklog: () => [],
            AddBacklogItem: () => {}, UpdateBacklogItem: () => {},
            DeleteBacklogItem: () => {}, MoveBacklogItem: () => {},
            GetObscuraStatus: () => ({ state: 'off', binaryExists: false, binaryPath: '' }),
            ObscuraScreenshot: () => {}, ObscuraDump: () => {}, ObscuraEval: () => {},
            ObscuraFetch: () => {}, StopObscura: () => {},
            CreateUser: () => [],
        };

        // Stub que lanza para cualquier método olvidado: hace visible una
        // llamada no mockeada en vez de un TypeError críptico.
        window.go = {
            main: {
                App: new Proxy(app, {
                    get(target, prop) {
                        if (prop in target) {
                            const fn = target[prop];
                            return (...args) => {
                                record(String(prop), args);
                                // Wails SIEMPRE devuelve Promises: envolver el
                                // resultado para que .then/await funcionen igual.
                                const result = fn(...args);
                                return (result instanceof Promise) ? result : Promise.resolve(result);
                            };
                        }
                        return (...args) => {
                            throw new Error(`wails-mock: método no mockeado: ${String(prop)}(${JSON.stringify(args)})`);
                        };
                    },
                }),
            },
        };
    }, { seedProjects: projects });
}
