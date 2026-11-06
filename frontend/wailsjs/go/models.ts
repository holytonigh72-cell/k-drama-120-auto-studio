export namespace pipeline {
	
	export class AudioClip {
	    path: string;
	    start_ms: number;
	    gain_db?: number;
	
	    static createFrom(source: any = {}) {
	        return new AudioClip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.start_ms = source["start_ms"];
	        this.gain_db = source["gain_db"];
	    }
	}
	export class ConnectionStatus {
	    state: string;
	    server?: string;
	    authenticated: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.server = source["server"];
	        this.authenticated = source["authenticated"];
	        this.message = source["message"];
	    }
	}
	export class Estimate {
	    currency: string;
	    total: number;
	    balance: number;
	    items: Record<string, number>;
	    verified: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Estimate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currency = source["currency"];
	        this.total = source["total"];
	        this.balance = source["balance"];
	        this.items = source["items"];
	        this.verified = source["verified"];
	    }
	}
	export class ImportResult {
	    path: string;
	    imported_path?: string;
	    kind: string;
	    reused: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.imported_path = source["imported_path"];
	        this.kind = source["kind"];
	        this.reused = source["reused"];
	    }
	}
	export class Stage {
	    name: string;
	    status: string;
	    attempts: number;
	    error?: string;
	    output?: string[];
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Stage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.attempts = source["attempts"];
	        this.error = source["error"];
	        this.output = source["output"];
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
	export class Project {
	    id: string;
	    title: string;
	    premise: string;
	    workspace: string;
	    output: string;
	    target_seconds: number;
	    resolution: string;
	    provider: string;
	    approved_cap: number;
	    estimate?: Estimate;
	    stages: Record<string, Stage>;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.premise = source["premise"];
	        this.workspace = source["workspace"];
	        this.output = source["output"];
	        this.target_seconds = source["target_seconds"];
	        this.resolution = source["resolution"];
	        this.provider = source["provider"];
	        this.approved_cap = source["approved_cap"];
	        this.estimate = this.convertValues(source["estimate"], Estimate);
	        this.stages = this.convertValues(source["stages"], Stage, true);
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
	export class RecoveredAsset {
	    job_key: string;
	    kind: string;
	    model: string;
	    charged_credits: number;
	    url: string;
	    sources?: string[];
	    local_path?: string;
	    status: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new RecoveredAsset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_key = source["job_key"];
	        this.kind = source["kind"];
	        this.model = source["model"];
	        this.charged_credits = source["charged_credits"];
	        this.url = source["url"];
	        this.sources = source["sources"];
	        this.local_path = source["local_path"];
	        this.status = source["status"];
	        this.error = source["error"];
	    }
	}

}


