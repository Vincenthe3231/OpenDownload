export namespace capture {
	
	export class Pairing {
	    code: string;
	    // Go type: time
	    expiresAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Pairing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
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
	export class SessionSnapshot {
	    active: boolean;
	    // Go type: time
	    expiresAt?: any;
	    paired: boolean;
	    mode: string;
	    nativeStatus: string;
	    browser?: string;
	    tabId?: number;
	    diagnostic?: diagnostics.Diagnostic;
	
	    static createFrom(source: any = {}) {
	        return new SessionSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
	        this.paired = source["paired"];
	        this.mode = source["mode"];
	        this.nativeStatus = source["nativeStatus"];
	        this.browser = source["browser"];
	        this.tabId = source["tabId"];
	        this.diagnostic = this.convertValues(source["diagnostic"], diagnostics.Diagnostic);
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
	export class StreamSummary {
	    id: string;
	    host: string;
	    name: string;
	    type: string;
	    // Go type: time
	    capturedAt: any;
	    errorCode?: string;
	    diagnosticId?: string;
	
	    static createFrom(source: any = {}) {
	        return new StreamSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.host = source["host"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.capturedAt = this.convertValues(source["capturedAt"], null);
	        this.errorCode = source["errorCode"];
	        this.diagnosticId = source["diagnosticId"];
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

export namespace diagnostics {
	
	export class Diagnostic {
	    code: string;
	    stage: string;
	    retryable: boolean;
	    userMessage: string;
	    diagnosticId: string;
	    // Go type: time
	    occurredAt: any;
	    safeContext?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Diagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.stage = source["stage"];
	        this.retryable = source["retryable"];
	        this.userMessage = source["userMessage"];
	        this.diagnosticId = source["diagnosticId"];
	        this.occurredAt = this.convertValues(source["occurredAt"], null);
	        this.safeContext = source["safeContext"];
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

export namespace download {
	
	export class JobSnapshot {
	    id: string;
	    name: string;
	    outputPath: string;
	    status: string;
	    downloadedBytes: number;
	    totalBytes: number;
	    completedUnits: number;
	    totalUnits: number;
	    bytesPerSecond: number;
	    etaSeconds: number;
	    hasEta: boolean;
	    activeConnections: number;
	    message?: string;
	    version: number;
	
	    static createFrom(source: any = {}) {
	        return new JobSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.outputPath = source["outputPath"];
	        this.status = source["status"];
	        this.downloadedBytes = source["downloadedBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.completedUnits = source["completedUnits"];
	        this.totalUnits = source["totalUnits"];
	        this.bytesPerSecond = source["bytesPerSecond"];
	        this.etaSeconds = source["etaSeconds"];
	        this.hasEta = source["hasEta"];
	        this.activeConnections = source["activeConnections"];
	        this.message = source["message"];
	        this.version = source["version"];
	    }
	}

}

export namespace main {
	
	export class CapturedDownloadRequest {
	    id: string;
	    capturedStreamId: string;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new CapturedDownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.capturedStreamId = source["capturedStreamId"];
	        this.outputDir = source["outputDir"];
	    }
	}
	export class DownloadRequest {
	    id: string;
	    url: string;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.outputDir = source["outputDir"];
	    }
	}

}

