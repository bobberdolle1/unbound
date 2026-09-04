export namespace engine {
	
	export class AssetVerificationResult {
	    totalFiles: number;
	    verified: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new AssetVerificationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalFiles = source["totalFiles"];
	        this.verified = source["verified"];
	        this.error = source["error"];
	    }
	}
	export class ProbeResult {
	    id: string;
	    service: string;
	    category: string;
	    name: string;
	    target: string;
	    transport: string;
	    status: string;
	    latency: number;
	    stage?: string;
	    class?: string;
	    error?: string;
	    details?: string;
	    resolvedIp?: string;
	    httpStatus?: number;
	    tlsVersion?: number;
	    certValid?: boolean;
	    certIssuer?: string;
	    attempts: number;
	    // Go type: time
	    timestamp: any;
	    isManualCheck: boolean;
	    url?: string;
	    success: boolean;
	    connectionRst?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.service = source["service"];
	        this.category = source["category"];
	        this.name = source["name"];
	        this.target = source["target"];
	        this.transport = source["transport"];
	        this.status = source["status"];
	        this.latency = source["latency"];
	        this.stage = source["stage"];
	        this.class = source["class"];
	        this.error = source["error"];
	        this.details = source["details"];
	        this.resolvedIp = source["resolvedIp"];
	        this.httpStatus = source["httpStatus"];
	        this.tlsVersion = source["tlsVersion"];
	        this.certValid = source["certValid"];
	        this.certIssuer = source["certIssuer"];
	        this.attempts = source["attempts"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.isManualCheck = source["isManualCheck"];
	        this.url = source["url"];
	        this.success = source["success"];
	        this.connectionRst = source["connectionRst"];
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
	export class ComparisonItem {
	    id: string;
	    service: string;
	    name: string;
	    target: string;
	    baseline: ProbeResult;
	    profile: ProbeResult;
	    verdict: string;
	    explanation: string;
	
	    static createFrom(source: any = {}) {
	        return new ComparisonItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.service = source["service"];
	        this.name = source["name"];
	        this.target = source["target"];
	        this.baseline = this.convertValues(source["baseline"], ProbeResult);
	        this.profile = this.convertValues(source["profile"], ProbeResult);
	        this.verdict = source["verdict"];
	        this.explanation = source["explanation"];
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
	export class BypassComparisonResult {
	    // Go type: time
	    timestamp: any;
	    profileName: string;
	    duration: number;
	    items: ComparisonItem[];
	    fixedCount: number;
	    directCount: number;
	    blockedCount: number;
	    brokenCount: number;
	    overallSummary: string;
	
	    static createFrom(source: any = {}) {
	        return new BypassComparisonResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.profileName = source["profileName"];
	        this.duration = source["duration"];
	        this.items = this.convertValues(source["items"], ComparisonItem);
	        this.fixedCount = source["fixedCount"];
	        this.directCount = source["directCount"];
	        this.blockedCount = source["blockedCount"];
	        this.brokenCount = source["brokenCount"];
	        this.overallSummary = source["overallSummary"];
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
	
	export class ComponentLocalState {
	    component: string;
	    name: string;
	    currentVersion: string;
	    status: string;
	    statusLabel: string;
	
	    static createFrom(source: any = {}) {
	        return new ComponentLocalState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.component = source["component"];
	        this.name = source["name"];
	        this.currentVersion = source["currentVersion"];
	        this.status = source["status"];
	        this.statusLabel = source["statusLabel"];
	    }
	}
	export class ComponentUpdateStatus {
	    component: string;
	    name: string;
	    currentVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	    // Go type: time
	    lastChecked: any;
	    status: string;
	    releaseUrl?: string;
	    changelog?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ComponentUpdateStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.component = source["component"];
	        this.name = source["name"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.lastChecked = this.convertValues(source["lastChecked"], null);
	        this.status = source["status"];
	        this.releaseUrl = source["releaseUrl"];
	        this.changelog = source["changelog"];
	        this.error = source["error"];
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
	export class DiagnosticGroup {
	    id: string;
	    name: string;
	    status: string;
	    probes: ProbeResult[];
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.probes = this.convertValues(source["probes"], ProbeResult);
	        this.summary = source["summary"];
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
	export class DiagnosticResult {
	    Component: string;
	    Status: string;
	    Details: string;
	    IsError: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Component = source["Component"];
	        this.Status = source["Status"];
	        this.Details = source["Details"];
	        this.IsError = source["IsError"];
	    }
	}
	export class DiscordCacheCleanupResult {
	    installationsFound: string[];
	    pathsScanned: string[];
	    pathsRemoved: string[];
	    filesRemoved: number;
	    bytesBefore: number;
	    bytesAfter: number;
	    bytesFreed: number;
	    failures: string[];
	    discordRunning: boolean;
	    runningProcesses?: string[];
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DiscordCacheCleanupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installationsFound = source["installationsFound"];
	        this.pathsScanned = source["pathsScanned"];
	        this.pathsRemoved = source["pathsRemoved"];
	        this.filesRemoved = source["filesRemoved"];
	        this.bytesBefore = source["bytesBefore"];
	        this.bytesAfter = source["bytesAfter"];
	        this.bytesFreed = source["bytesFreed"];
	        this.failures = source["failures"];
	        this.discordRunning = source["discordRunning"];
	        this.runningProcesses = source["runningProcesses"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class DoctorResult {
	    overallStatus: string;
	    mode: string;
	    // Go type: time
	    timestamp: any;
	    duration: number;
	    appVersion: string;
	    engineVersion: string;
	    os: string;
	    arch: string;
	    activeProfile: string;
	    groups: DiagnosticGroup[];
	    passCount: number;
	    failCount: number;
	    warnCount: number;
	    notVerCount: number;
	    infoCount: number;
	    manualItems?: string[];
	
	    static createFrom(source: any = {}) {
	        return new DoctorResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overallStatus = source["overallStatus"];
	        this.mode = source["mode"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.duration = source["duration"];
	        this.appVersion = source["appVersion"];
	        this.engineVersion = source["engineVersion"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.activeProfile = source["activeProfile"];
	        this.groups = this.convertValues(source["groups"], DiagnosticGroup);
	        this.passCount = source["passCount"];
	        this.failCount = source["failCount"];
	        this.warnCount = source["warnCount"];
	        this.notVerCount = source["notVerCount"];
	        this.infoCount = source["infoCount"];
	        this.manualItems = source["manualItems"];
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
	export class DoctorRunStart {
	    runId: string;
	    mode: string;
	    total: number;
	    // Go type: time
	    startedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new DoctorRunStart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.mode = source["mode"];
	        this.total = source["total"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
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
	export class DoctorRunState {
	    runId: string;
	    mode: string;
	    status: string;
	    completed: number;
	    total: number;
	    percent: number;
	    running: string[];
	    lastCompleted: string;
	    // Go type: time
	    startedAt: any;
	    elapsedMs: number;
	    result?: DoctorResult;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DoctorRunState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.mode = source["mode"];
	        this.status = source["status"];
	        this.completed = source["completed"];
	        this.total = source["total"];
	        this.percent = source["percent"];
	        this.running = source["running"];
	        this.lastCompleted = source["lastCompleted"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.elapsedMs = source["elapsedMs"];
	        this.result = this.convertValues(source["result"], DoctorResult);
	        this.error = source["error"];
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
	    autoStartProfile: boolean;
	    defaultProfile: string;
	    startupProfileMode: string;
	    gameFilter: boolean;
	    autoUpdateEnabled: boolean;
	    showLogs: boolean;
	    enableTCPTimestamps: boolean;
	    discordCacheAutoClean: boolean;
	    secureDns: boolean;
	    favoriteProfiles: string[];
	    autoReconnect: boolean;
	    autoTuneTargets: string[];
	    diagnosticsMode: string;
	    autoUpdatePolicy: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoStart = source["autoStart"];
	        this.startMinimized = source["startMinimized"];
	        this.autoStartProfile = source["autoStartProfile"];
	        this.defaultProfile = source["defaultProfile"];
	        this.startupProfileMode = source["startupProfileMode"];
	        this.gameFilter = source["gameFilter"];
	        this.autoUpdateEnabled = source["autoUpdateEnabled"];
	        this.showLogs = source["showLogs"];
	        this.enableTCPTimestamps = source["enableTCPTimestamps"];
	        this.discordCacheAutoClean = source["discordCacheAutoClean"];
	        this.secureDns = source["secureDns"];
	        this.favoriteProfiles = source["favoriteProfiles"];
	        this.autoReconnect = source["autoReconnect"];
	        this.autoTuneTargets = source["autoTuneTargets"];
	        this.diagnosticsMode = source["diagnosticsMode"];
	        this.autoUpdatePolicy = source["autoUpdatePolicy"];
	    }
	}
	export class SystemComponentState {
	    components: ComponentLocalState[];
	
	    static createFrom(source: any = {}) {
	        return new SystemComponentState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.components = this.convertValues(source["components"], ComponentLocalState);
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
	export class SystemUpdateOverview {
	    // Go type: time
	    lastChecked: any;
	    components: ComponentUpdateStatus[];
	
	    static createFrom(source: any = {}) {
	        return new SystemUpdateOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lastChecked = this.convertValues(source["lastChecked"], null);
	        this.components = this.convertValues(source["components"], ComponentUpdateStatus);
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
	export class TaskRegistrationInfo {
	    exists: boolean;
	    executable: string;
	    arguments: string;
	    rawCommand: string;
	    taskState: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskRegistrationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	        this.executable = source["executable"];
	        this.arguments = source["arguments"];
	        this.rawCommand = source["rawCommand"];
	        this.taskState = source["taskState"];
	    }
	}

}

export namespace main {
	
	export class PingRecord {
	    ts: number;
	    lat: number;
	    st: string;
	
	    static createFrom(source: any = {}) {
	        return new PingRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ts = source["ts"];
	        this.lat = source["lat"];
	        this.st = source["st"];
	    }
	}

}

