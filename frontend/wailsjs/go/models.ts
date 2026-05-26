export namespace desktop {
	
	export class DashboardInfo {
	    totalGPUs: number;
	    freeGPUs: number;
	    runningJobs: number;
	    queuedJobs: number;
	    avgUtil: number;
	    totalVRAMMB: number;
	    usedVRAMMB: number;
	
	    static createFrom(source: any = {}) {
	        return new DashboardInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalGPUs = source["totalGPUs"];
	        this.freeGPUs = source["freeGPUs"];
	        this.runningJobs = source["runningJobs"];
	        this.queuedJobs = source["queuedJobs"];
	        this.avgUtil = source["avgUtil"];
	        this.totalVRAMMB = source["totalVRAMMB"];
	        this.usedVRAMMB = source["usedVRAMMB"];
	    }
	}
	export class DeviceInfo {
	    id: number;
	    vendor: string;
	    name: string;
	    vramTotalMB: number;
	    vramUsedMB: number;
	    utilizationPct: number;
	    temperatureC: number;
	    allocated: boolean;
	    allocatedTo: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.vendor = source["vendor"];
	        this.name = source["name"];
	        this.vramTotalMB = source["vramTotalMB"];
	        this.vramUsedMB = source["vramUsedMB"];
	        this.utilizationPct = source["utilizationPct"];
	        this.temperatureC = source["temperatureC"];
	        this.allocated = source["allocated"];
	        this.allocatedTo = source["allocatedTo"];
	    }
	}
	export class JobInfo {
	    id: string;
	    command: string;
	    numGPUs: number;
	    minVRAMMB: number;
	    priority: number;
	    status: string;
	    submittedAt: string;
	    startedAt: string;
	    gpuIDs: number[];
	
	    static createFrom(source: any = {}) {
	        return new JobInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.command = source["command"];
	        this.numGPUs = source["numGPUs"];
	        this.minVRAMMB = source["minVRAMMB"];
	        this.priority = source["priority"];
	        this.status = source["status"];
	        this.submittedAt = source["submittedAt"];
	        this.startedAt = source["startedAt"];
	        this.gpuIDs = source["gpuIDs"];
	    }
	}
	export class LinkInfo {
	    gpuA: number;
	    gpuB: number;
	    type: string;
	    bandwidthGBps: number;
	
	    static createFrom(source: any = {}) {
	        return new LinkInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gpuA = source["gpuA"];
	        this.gpuB = source["gpuB"];
	        this.type = source["type"];
	        this.bandwidthGBps = source["bandwidthGBps"];
	    }
	}
	export class SubmitRequest {
	    command: string;
	    numGPUs: number;
	    minVRAMMB: number;
	    priority: number;
	
	    static createFrom(source: any = {}) {
	        return new SubmitRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.numGPUs = source["numGPUs"];
	        this.minVRAMMB = source["minVRAMMB"];
	        this.priority = source["priority"];
	    }
	}
	export class TopologyInfo {
	    numGPUs: number;
	    bandwidth: number[][];
	    links: LinkInfo[];
	
	    static createFrom(source: any = {}) {
	        return new TopologyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.numGPUs = source["numGPUs"];
	        this.bandwidth = source["bandwidth"];
	        this.links = this.convertValues(source["links"], LinkInfo);
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

