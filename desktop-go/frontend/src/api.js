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
};

export const events = () => window.runtime;
