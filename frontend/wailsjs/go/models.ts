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
	export class Settings {
	    autoStart: boolean;
	    startMinimized: boolean;
	    defaultProfile: string;
	    startupProfileMode: string;
	    gameFilter: boolean;
	    autoUpdateEnabled: boolean;
	    showLogs: boolean;
	    enableTCPTimestamps: boolean;
	    discordCacheAutoClean: boolean;
	    secureDns: boolean;
	
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
	        this.showLogs = source["showLogs"];
	        this.enableTCPTimestamps = source["enableTCPTimestamps"];
	        this.discordCacheAutoClean = source["discordCacheAutoClean"];
	        this.secureDns = source["secureDns"];
	    }
	}

}

