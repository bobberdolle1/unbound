export namespace engine {
	
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

