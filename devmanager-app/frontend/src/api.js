// Wrapper fino sobre bindings Wails. Un solo lugar que conoce window.go.
const app = () => window.go.main.App;

export const api = {
    getProjects: () => app().GetProjects(),
    addProject: (name, path) => app().AddProject(name, path),
    removeProject: (id) => app().RemoveProject(id),
    toggleProject: (id) => app().ToggleProject(id),
    browseForFolder: () => app().BrowseForFolder(),
    importProjects: (folderPath) => app().ImportProjects(folderPath),
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
    // Settings
    getSettings: () => app().GetSettings(),
    setSetting: (key, value) => app().SetSetting(key, value),
    getEffectiveTheme: () => app().GetEffectiveTheme(),
    // Backlog
    getBacklog: (i) => app().GetBacklog(i),
    addBacklogItem: (i, title, description, status, priority) => app().AddBacklogItem(i, title, description, status, priority),
    updateBacklogItem: (i, itemId, title, description, status, priority) => app().UpdateBacklogItem(i, itemId, title, description, status, priority),
    deleteBacklogItem: (i, itemId) => app().DeleteBacklogItem(i, itemId),
    moveBacklogItem: (i, itemId, newIndex) => app().MoveBacklogItem(i, itemId, newIndex),
    // App
    restartApp: () => app().RestartApp(),
    quit: () => app().Quit(),
};

export const events = () => window.runtime;
