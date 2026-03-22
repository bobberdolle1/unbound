export namespace engine {
	
	export class AdvancedProfile {
	    Name: string;
	    Description: string;
	    Args: string[];
	    Category: string;
	    Techniques: string[];
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Description = source["Description"];
	        this.Args = source["Args"];
	        this.Category = source["Category"];
	        this.Techniques = source["Techniques"];
	    }
	}
	export class BlobPayload {
	    Type: string;
	    Data: number[];
	    Description: string;
	
	    static createFrom(source: any = {}) {
	        return new BlobPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Data = source["Data"];
	        this.Description = source["Description"];
	    }
	}
	export class DiagnosticResult {
	    Name: string;
	    Status: string;
	    Message: string;
	    Critical: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Status = source["Status"];
	        this.Message = source["Message"];
	        this.Critical = source["Critical"];
	    }
	}
	export class DiagnosticsReport {
	    Results: DiagnosticResult[];
	    Score: number;
	    Summary: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticsReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Results = this.convertValues(source["Results"], DiagnosticResult);
	        this.Score = source["Score"];
	        this.Summary = source["Summary"];
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
	export class Settings {
	    autoStart: boolean;
	    startMinimized: boolean;
	    defaultProfile: string;
	    startupProfileMode: string;
	    gameFilter: boolean;
	    autoUpdateEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoStart = source["autoStart"];
	        this.startMinimized = source["startMinimized"];
	        this.defaultProfile = source["defaultProfile"];
	        this.startupProfileMode = source["startupProfileMode"];
	        this.gameFilter = source["gameFilter"];
	        this.autoUpdateEnabled = source["autoUpdateEnabled"];
	    }
	}
	export class ProfileStats {
	    profile_name: string;
	    test_count: number;
	    success_count: number;
	    failure_count: number;
	    average_latency: number;
	    average_score: number;
	    // Go type: time
	    last_tested: any;
	    recommended_rank: number;
	
	    static createFrom(source: any = {}) {
	        return new ProfileStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile_name = source["profile_name"];
	        this.test_count = source["test_count"];
	        this.success_count = source["success_count"];
	        this.failure_count = source["failure_count"];
	        this.average_latency = source["average_latency"];
	        this.average_score = source["average_score"];
	        this.last_tested = this.convertValues(source["last_tested"], null);
	        this.recommended_rank = source["recommended_rank"];
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
	export class TestAnalytics {
	    total_sessions: number;
	    total_tests: number;
	    successful_tests: number;
	    failed_tests: number;
	    average_score: number;
	    profile_stats: {[key: string]: ProfileStats};
	    // Go type: time
	    last_updated: any;
	
	    static createFrom(source: any = {}) {
	        return new TestAnalytics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_sessions = source["total_sessions"];
	        this.total_tests = source["total_tests"];
	        this.successful_tests = source["successful_tests"];
	        this.failed_tests = source["failed_tests"];
	        this.average_score = source["average_score"];
	        this.profile_stats = source["profile_stats"];
	        this.last_updated = this.convertValues(source["last_updated"], null);
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
	export class TestResultPersistent {
	    url: string;
	    success: boolean;
	    latency: number;
	    error?: string;
	    status_code?: number;
	    tcp_freeze?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TestResultPersistent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.success = source["success"];
	        this.latency = source["latency"];
	        this.error = source["error"];
	        this.status_code = source["status_code"];
	        this.tcp_freeze = source["tcp_freeze"];
	    }
	}
	export class TestSession {
	    id: string;
	    // Go type: time
	    start_time: any;
	    // Go type: time
	    end_time: any;
	    duration: number;
	    profile_name: string;
	    test_mode: string;
	    results: TestResultPersistent[];
	    score: number;
	    success_rate: number;
	    best_profile?: string;
	
	    static createFrom(source: any = {}) {
	        return new TestSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.start_time = this.convertValues(source["start_time"], null);
	        this.end_time = this.convertValues(source["end_time"], null);
	        this.duration = source["duration"];
	        this.profile_name = source["profile_name"];
	        this.test_mode = source["test_mode"];
	        this.results = this.convertValues(source["results"], TestResultPersistent);
	        this.score = source["score"];
	        this.success_rate = source["success_rate"];
	        this.best_profile = source["best_profile"];
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
	export class UpdateInfo {
	    available: boolean;
	    version: string;
	    downloadUrl: string;
	    changelog: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.downloadUrl = source["downloadUrl"];
	        this.changelog = source["changelog"];
	    }
	}

}

