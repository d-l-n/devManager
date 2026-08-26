export namespace main {
	
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

