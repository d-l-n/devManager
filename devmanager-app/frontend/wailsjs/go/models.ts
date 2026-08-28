export namespace config {
	
	export class Settings {
	    theme: string;
	    monitor_polling: boolean;
	    toasts_enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.monitor_polling = source["monitor_polling"];
	        this.toasts_enabled = source["toasts_enabled"];
	    }
	}

}

export namespace detection {
	
	export class ProjectConfig {
	    name: string;
	    server_command: string;
	    port: number;
	    url: string;
	    playwright_enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProjectConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.server_command = source["server_command"];
	        this.port = source["port"];
	        this.url = source["url"];
	        this.playwright_enabled = source["playwright_enabled"];
	    }
	}
	export class Script {
	    name: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new Script(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.command = source["command"];
	    }
	}

}

export namespace evidence {
	
	export class File {
	    path: string;
	    relPath: string;
	    kind: string;
	    testDir: string;
	    mtime: number;
	
	    static createFrom(source: any = {}) {
	        return new File(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.relPath = source["relPath"];
	        this.kind = source["kind"];
	        this.testDir = source["testDir"];
	        this.mtime = source["mtime"];
	    }
	}

}

export namespace git {
	
	export class LastCommit {
	    hash: string;
	    subject: string;
	    dateRel: string;
	
	    static createFrom(source: any = {}) {
	        return new LastCommit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.subject = source["subject"];
	        this.dateRel = source["dateRel"];
	    }
	}
	export class Status {
	    isRepo: boolean;
	    branch: string;
	    isDirty: boolean;
	    error?: string;
	    ahead: number;
	    behind: number;
	    hasUpstream: boolean;
	    lastCommit?: LastCommit;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isRepo = source["isRepo"];
	        this.branch = source["branch"];
	        this.isDirty = source["isDirty"];
	        this.error = source["error"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.hasUpstream = source["hasUpstream"];
	        this.lastCommit = this.convertValues(source["lastCommit"], LastCommit);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace logger {
	
	export class Entry {
	    ts: string;
	    text: string;
	    isError: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ts = source["ts"];
	        this.text = source["text"];
	        this.isError = source["isError"];
	    }
	}

}

export namespace main {
	
	export class ResRow {
	    name: string;
	    pid: number;
	    children: number;
	    cpu: number;
	    rss: number;
	
	    static createFrom(source: any = {}) {
	        return new ResRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pid = source["pid"];
	        this.children = source["children"];
	        this.cpu = source["cpu"];
	        this.rss = source["rss"];
	    }
	}
	export class PortRow {
	    index: number;
	    name: string;
	    port: number;
	    state: string;
	    ownerName: string;
	    ownerPID: number;
	
	    static createFrom(source: any = {}) {
	        return new PortRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.port = source["port"];
	        this.state = source["state"];
	        this.ownerName = source["ownerName"];
	        this.ownerPID = source["ownerPID"];
	    }
	}
	export class MonitorData {
	    portRows: PortRow[];
	    resRows: ResRow[];
	
	    static createFrom(source: any = {}) {
	        return new MonitorData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.portRows = this.convertValues(source["portRows"], PortRow);
	        this.resRows = this.convertValues(source["resRows"], ResRow);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NotifyResult {
	    ok: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new NotifyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	    }
	}
	export class PlaywrightStatus {
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new PlaywrightStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	    }
	}
	
	
	export class ScriptStatus {
	    running: boolean;
	    activeName: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.activeName = source["activeName"];
	    }
	}
	export class ServerStatus {
	    state: string;
	    activePort: number;
	    activeUrl: string;
	    uptimeSeconds: number;
	    failureReason: string;
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.activePort = source["activePort"];
	        this.activeUrl = source["activeUrl"];
	        this.uptimeSeconds = source["uptimeSeconds"];
	        this.failureReason = source["failureReason"];
	        this.running = source["running"];
	    }
	}

}

export namespace models {
	
	export class BacklogItem {
	    id: string;
	    title: string;
	    description: string;
	    status: string;
	    priority: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new BacklogItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.priority = source["priority"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlaywrightConfig {
	    enabled: boolean;
	    command: string;
	    ui_command: string;
	    debug_command: string;
	    report_command: string;
	
	    static createFrom(source: any = {}) {
	        return new PlaywrightConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.command = source["command"];
	        this.ui_command = source["ui_command"];
	        this.debug_command = source["debug_command"];
	        this.report_command = source["report_command"];
	    }
	}
	export class ServerConfig {
	    enabled: boolean;
	    command: string;
	    port: number;
	    url: string;
	    startup_timeout: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.command = source["command"];
	        this.port = source["port"];
	        this.url = source["url"];
	        this.startup_timeout = source["startup_timeout"];
	    }
	}
	export class Project {
	    name: string;
	    path: string;
	    server: ServerConfig;
	    playwright: PlaywrightConfig;
	    pinned: boolean;
	    backlog: BacklogItem[];
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.server = this.convertValues(source["server"], ServerConfig);
	        this.playwright = this.convertValues(source["playwright"], PlaywrightConfig);
	        this.pinned = source["pinned"];
	        this.backlog = this.convertValues(source["backlog"], BacklogItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

