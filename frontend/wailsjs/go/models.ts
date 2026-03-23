export namespace engine {
	
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
	export class Profile {
	    Name: string;
	    Args: string[];
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Args = source["Args"];
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

