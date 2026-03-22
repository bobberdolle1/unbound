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
	export class XrayNode {
	    id: string;
	    name: string;
	    address: string;
	    port: number;
	    uuid: string;
	    flow: string;
	    security: string;
	    sni: string;
	    fp: string;
	    pbk: string;
	    sid: string;
	    type: string;
	    rawUri: string;
	
	    static createFrom(source: any = {}) {
	        return new XrayNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.uuid = source["uuid"];
	        this.flow = source["flow"];
	        this.security = source["security"];
	        this.sni = source["sni"];
	        this.fp = source["fp"];
	        this.pbk = source["pbk"];
	        this.sid = source["sid"];
	        this.type = source["type"];
	        this.rawUri = source["rawUri"];
	    }
	}

}

