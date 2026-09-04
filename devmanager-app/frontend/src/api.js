// Wrapper fino sobre bindings Wails. Un solo lugar que conoce window.go.
const app = () => window.go.main.App;

export const api = {
    getProjects: () => app().GetProjects(),
    addProject: (p) => app().AddProject(p),
    updateProject: (i, p) => app().UpdateProject(i, p),
    removeProject: (i) => app().RemoveProject(i),
    togglePin: (i) => app().TogglePin(i),
    startServer: (i) => app().StartServer(i),
    stopServer: (i) => app().StopServer(i),
    restartServer: (i) => app().RestartServer(i),
    getServerStatus: (i) => app().GetServerStatus(i),
    // Playwright
    runTests: (i) => app().RunTests(i),
    runUI: (i) => app().RunUI(i),
    runDebug: (i) => app().RunDebug(i),
    showReport: (i) => app().ShowReport(i),
    stopPlaywright: (i) => app().StopPlaywright(i),
    getPlaywrightStatus: (i) => app().GetPlaywrightStatus(i),
    // Scripts
    getScripts: (i) => app().GetScripts(i),
    runScript: (i, name, cmd) => app().RunScript(i, name, cmd),
    stopScript: (i) => app().StopScript(i),
    getScriptStatus: (i) => app().GetScriptStatus(i),
    // Git
    getGitStatus: (i) => app().GetGitStatus(i),
    gitAction: (i, action) => app().GitAction(i, action),
    // Git (Issue #63: diff, branches, tags)
    getGitDiff: (i) => app().GetGitDiff(i),
    gitBranches: (i) => app().GitBranches(i),
    gitCreateBranch: (i, name) => app().GitCreateBranch(i, name),
    gitRenameBranch: (i, oldName, newName) => app().GitRenameBranch(i, oldName, newName),
    gitDeleteBranch: (i, name) => app().GitDeleteBranch(i, name),
    gitCheckout: (i, name) => app().GitCheckout(i, name),
    gitTags: (i) => app().GitTags(i),
    gitCreateTag: (i, name) => app().GitCreateTag(i, name),
    gitDeleteTag: (i, name) => app().GitDeleteTag(i, name),
    gitPushTag: (i, name) => app().GitPushTag(i, name),
    // Monitor
    getMonitorData: () => app().GetMonitorData(),
    killTree: (pid) => app().KillTree(pid),
    // Evidence / externals
    getEvidence: (i) => app().GetEvidence(i),
    getEvidenceThumbnail: (path) => app().GetEvidenceThumbnail(path),
    openTraceViewer: (i, path) => app().OpenTraceViewer(i, path),
    openHTMLReport: (i) => app().OpenHTMLReport(i),
    openExternally: (path) => app().OpenExternally(path),
    openContainingFolder: (path) => app().OpenContainingFolder(path),
    openInExplorer: (i) => app().OpenInExplorer(i),
    openTerminal: (i) => app().OpenTerminal(i),
    openVSCode: (i) => app().OpenVSCode(i),
    openOpenCode: (i) => app().OpenOpenCode(i),
    // Detección de config de proyecto (Issue #11) + diálogo nativo
    detectProjectConfig: (path) => app().DetectProjectConfig(path),
    browseFolder: () => app().BrowseFolder(),
    browseWorkspaceFolder: () => app().BrowseWorkspaceFolder(),
    discoverProjects: (root) => app().DiscoverProjects(root),
    // App Log global (Issue #14)
    getAppLog: () => app().GetAppLog(),
    clearAppLog: () => app().ClearAppLog(),
    // Settings
    getSettings: () => app().GetSettings(),
    setSetting: (key, value) => app().SetSetting(key, value),
    // App
    reloadProjects: () => app().ReloadProjects(),
    autoAssignPorts: () => app().AutoAssignPorts(),
    saveDetectedPort: (i, port) => app().SaveDetectedPort(i, port),
    openURL: (url) => app().OpenURL(url),
    restartApp: () => app().RestartApp(),
    quit: () => app().Quit(),
    // Backlog (feature)


    getBacklog: (i) => app().GetBacklog(i),
    addBacklogItem: (i, title, description, status, priority) => app().AddBacklogItem(i, title, description, status, priority),
    updateBacklogItem: (i, itemId, title, description, status, priority) => app().UpdateBacklogItem(i, itemId, title, description, status, priority),
    deleteBacklogItem: (i, itemId) => app().DeleteBacklogItem(i, itemId),
    moveBacklogItem: (i, itemId, newIndex) => app().MoveBacklogItem(i, itemId, newIndex),
    // Obscura (herramienta auxiliar: screenshots, dump, eval, fetch libre)
    getObscuraStatus: (i) => app().GetObscuraStatus(i),
    obscuraScreenshot: (i, url) => app().ObscuraScreenshot(i, url),
    obscuraDump: (i, url, format) => app().ObscuraDump(i, url, format),
    obscuraEval: (i, url, js) => app().ObscuraEval(i, url, js),
    obscuraFetch: (i, command) => app().ObscuraFetch(i, command),
    stopObscura: (i) => app().StopObscura(i),
    // Updater (Issue #58)
    checkForUpdate: () => app().CheckForUpdate(),
    getVersion: () => app().GetVersion(),
};

export const events = () => window.runtime;
