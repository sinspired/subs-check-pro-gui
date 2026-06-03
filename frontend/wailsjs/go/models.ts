export namespace frontend {
	
	export class AppInfo {
	    apiKey: string;
	    listenPort: string;
	    subStorePort: string;
	    keyIsRandom: boolean;
	    isFirstRun: boolean;
	    configPath: string;
	    portConflictHTTP: boolean;
	    portConflictSubStore: boolean;
	    autostartEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.listenPort = source["listenPort"];
	        this.subStorePort = source["subStorePort"];
	        this.keyIsRandom = source["keyIsRandom"];
	        this.isFirstRun = source["isFirstRun"];
	        this.configPath = source["configPath"];
	        this.portConflictHTTP = source["portConflictHTTP"];
	        this.portConflictSubStore = source["portConflictSubStore"];
	        this.autostartEnabled = source["autostartEnabled"];
	    }
	}

}

