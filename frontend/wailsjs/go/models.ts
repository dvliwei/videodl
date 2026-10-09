export namespace download {
	
	export class MediaSource {
	
	
	    static createFrom(source: any = {}) {
	        return new MediaSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace main {
	
	export class AnalyzeRequest {
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	    }
	}
	export class AnalyzeResponse {
	    analysisId: string;
	    phase: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.analysisId = source["analysisId"];
	        this.phase = source["phase"];
	    }
	}
	export class DownloadDirectoryResponse {
	    path: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DownloadDirectoryResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.isDefault = source["isDefault"];
	    }
	}

}

export namespace media {
	
	export class MediaVariant {
	    id: string;
	    label: string;
	    width?: number;
	    height?: number;
	    bandwidth?: number;
	    hasVideo: boolean;
	    hasAudio: boolean;
	    audioDescription: string;
	
	    static createFrom(source: any = {}) {
	        return new MediaVariant(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.bandwidth = source["bandwidth"];
	        this.hasVideo = source["hasVideo"];
	        this.hasAudio = source["hasAudio"];
	        this.audioDescription = source["audioDescription"];
	    }
	}
	export class MediaCandidate {
	    id: string;
	    title: string;
	    displayUrl?: string;
	    sourceType: string;
	    format?: string;
	    width?: number;
	    height?: number;
	    durationSeconds?: number;
	    sizeBytes?: number;
	    hasVideo: boolean;
	    hasAudio: boolean;
	    variants?: MediaVariant[];
	    unsupported?: string;
	
	    static createFrom(source: any = {}) {
	        return new MediaCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.displayUrl = source["displayUrl"];
	        this.sourceType = source["sourceType"];
	        this.format = source["format"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.durationSeconds = source["durationSeconds"];
	        this.sizeBytes = source["sizeBytes"];
	        this.hasVideo = source["hasVideo"];
	        this.hasAudio = source["hasAudio"];
	        this.variants = this.convertValues(source["variants"], MediaVariant);
	        this.unsupported = source["unsupported"];
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
	export class AnalysisResult {
	    id: string;
	    pageTitle: string;
	    candidates: MediaCandidate[];
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new AnalysisResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.pageTitle = source["pageTitle"];
	        this.candidates = this.convertValues(source["candidates"], MediaCandidate);
	        this.warnings = source["warnings"];
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
	export class DownloadRequest {
	    analysisId: string;
	    mediaId: string;
	    variantId?: string;
	    outputPath: string;
	    profile: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.analysisId = source["analysisId"];
	        this.mediaId = source["mediaId"];
	        this.variantId = source["variantId"];
	        this.outputPath = source["outputPath"];
	        this.profile = source["profile"];
	    }
	}
	export class DownloadTask {
	    id: string;
	    title: string;
	    analysisId?: string;
	    mediaId?: string;
	    variantId?: string;
	    profile: string;
	    state: string;
	    phase?: string;
	    attempt: number;
	    progress?: number;
	    speedBytesPerSecond?: number;
	    sizeBytes?: number;
	    outputPath?: string;
	    errorCode?: string;
	    errorMessage?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    startedAt?: any;
	    // Go type: time
	    completedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new DownloadTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.analysisId = source["analysisId"];
	        this.mediaId = source["mediaId"];
	        this.variantId = source["variantId"];
	        this.profile = source["profile"];
	        this.state = source["state"];
	        this.phase = source["phase"];
	        this.attempt = source["attempt"];
	        this.progress = source["progress"];
	        this.speedBytesPerSecond = source["speedBytesPerSecond"];
	        this.sizeBytes = source["sizeBytes"];
	        this.outputPath = source["outputPath"];
	        this.errorCode = source["errorCode"];
	        this.errorMessage = source["errorMessage"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.completedAt = this.convertValues(source["completedAt"], null);
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

