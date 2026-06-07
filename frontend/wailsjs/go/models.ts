export namespace agentserver {
	
	export class ContainerInfo {
	    id: string;
	    name: string;
	    image: string;
	    status: string;
	    cpuPercent: number;
	    memUsedMB: number;
	    memLimitMB: number;
	
	    static createFrom(source: any = {}) {
	        return new ContainerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.image = source["image"];
	        this.status = source["status"];
	        this.cpuPercent = source["cpuPercent"];
	        this.memUsedMB = source["memUsedMB"];
	        this.memLimitMB = source["memLimitMB"];
	    }
	}
	export class ContainerReport {
	    available: boolean;
	    runtime: string;
	    error?: string;
	    containers: ContainerInfo[];
	
	    static createFrom(source: any = {}) {
	        return new ContainerReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.runtime = source["runtime"];
	        this.error = source["error"];
	        this.containers = this.convertValues(source["containers"], ContainerInfo);
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
	export class GPUProcInfo {
	    pid: number;
	    name: string;
	    memMB: number;
	
	    static createFrom(source: any = {}) {
	        return new GPUProcInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.name = source["name"];
	        this.memMB = source["memMB"];
	    }
	}
	export class HostStats {
	    hostname: string;
	    osName: string;
	    kernel: string;
	    arch: string;
	    cpuModel: string;
	    uptimeSeconds: number;
	    cpuPercent: number;
	    cpuCores: number;
	    memTotalMB: number;
	    memUsedMB: number;
	    loadAvg: number[];
	    perCoreCPU: number[];
	
	    static createFrom(source: any = {}) {
	        return new HostStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostname = source["hostname"];
	        this.osName = source["osName"];
	        this.kernel = source["kernel"];
	        this.arch = source["arch"];
	        this.cpuModel = source["cpuModel"];
	        this.uptimeSeconds = source["uptimeSeconds"];
	        this.cpuPercent = source["cpuPercent"];
	        this.cpuCores = source["cpuCores"];
	        this.memTotalMB = source["memTotalMB"];
	        this.memUsedMB = source["memUsedMB"];
	        this.loadAvg = source["loadAvg"];
	        this.perCoreCPU = source["perCoreCPU"];
	    }
	}
	export class ProcInfo {
	    pid: number;
	    command: string;
	    cpuPercent: number;
	    memMB: number;
	
	    static createFrom(source: any = {}) {
	        return new ProcInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.command = source["command"];
	        this.cpuPercent = source["cpuPercent"];
	        this.memMB = source["memMB"];
	    }
	}

}

export namespace desktop {
	
	export class AppLogEntry {
	    time: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AppLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.message = source["message"];
	    }
	}
	export class ClusterDeviceInfo {
	    id: number;
	    vendor: string;
	    name: string;
	    vramTotalMB: number;
	    vramUsedMB: number;
	    utilizationPct: number;
	    temperatureC: number;
	    allocated: boolean;
	    allocatedTo: string;
	    nodeID: string;
	    nodeName: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterDeviceInfo(source);
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
	        this.nodeID = source["nodeID"];
	        this.nodeName = source["nodeName"];
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
	export class NodeTopologyInfo {
	    nodeID: string;
	    nodeName: string;
	    numGPUs: number;
	    bandwidth: number[][];
	    links: LinkInfo[];
	
	    static createFrom(source: any = {}) {
	        return new NodeTopologyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeID = source["nodeID"];
	        this.nodeName = source["nodeName"];
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
	export class ClusterTopologyInfo {
	    nodes: NodeTopologyInfo[];
	
	    static createFrom(source: any = {}) {
	        return new ClusterTopologyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], NodeTopologyInfo);
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
	    mode: string;
	    numGPUs: number;
	    minVRAMMB: number;
	    priority: number;
	    status: string;
	    submittedAt: string;
	    startedAt: string;
	    gpuIDs: number[];
	    nodeID: string;
	    nodeName: string;
	
	    static createFrom(source: any = {}) {
	        return new JobInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.command = source["command"];
	        this.mode = source["mode"];
	        this.numGPUs = source["numGPUs"];
	        this.minVRAMMB = source["minVRAMMB"];
	        this.priority = source["priority"];
	        this.status = source["status"];
	        this.submittedAt = source["submittedAt"];
	        this.startedAt = source["startedAt"];
	        this.gpuIDs = source["gpuIDs"];
	        this.nodeID = source["nodeID"];
	        this.nodeName = source["nodeName"];
	    }
	}
	
	export class LogData {
	    data: string;
	    offset: number;
	    eof: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LogData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data = source["data"];
	        this.offset = source["offset"];
	        this.eof = source["eof"];
	    }
	}
	export class NodeInfo {
	    id: string;
	    name: string;
	    status: string;
	    numGPUs: number;
	    freeGPUs: number;
	    localDir: string;
	    remoteDir: string;
	    gpuVendor: string;
	    gpuName: string;
	    arch: string;
	    os: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.numGPUs = source["numGPUs"];
	        this.freeGPUs = source["freeGPUs"];
	        this.localDir = source["localDir"];
	        this.remoteDir = source["remoteDir"];
	        this.gpuVendor = source["gpuVendor"];
	        this.gpuName = source["gpuName"];
	        this.arch = source["arch"];
	        this.os = source["os"];
	    }
	}
	export class NodeMonitorInfo {
	    nodeID: string;
	    nodeName: string;
	    reachable: boolean;
	    error?: string;
	    host: agentserver.HostStats;
	    gpus: DeviceInfo[];
	    containers: agentserver.ContainerReport;
	    processes: agentserver.ProcInfo[];
	    gpuProcesses: agentserver.GPUProcInfo[];
	    collectedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeMonitorInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeID = source["nodeID"];
	        this.nodeName = source["nodeName"];
	        this.reachable = source["reachable"];
	        this.error = source["error"];
	        this.host = this.convertValues(source["host"], agentserver.HostStats);
	        this.gpus = this.convertValues(source["gpus"], DeviceInfo);
	        this.containers = this.convertValues(source["containers"], agentserver.ContainerReport);
	        this.processes = this.convertValues(source["processes"], agentserver.ProcInfo);
	        this.gpuProcesses = this.convertValues(source["gpuProcesses"], agentserver.GPUProcInfo);
	        this.collectedAt = source["collectedAt"];
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
	
	export class SavedNodeInfo {
	    id: string;
	    sshCommand: string;
	    mockMode: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SavedNodeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sshCommand = source["sshCommand"];
	        this.mockMode = source["mockMode"];
	    }
	}
	export class SubmitRequest {
	    command: string;
	    pathVariable: string;
	    mode: string;
	    numGPUs: number;
	    minVRAMMB: number;
	    priority: number;
	
	    static createFrom(source: any = {}) {
	        return new SubmitRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.pathVariable = source["pathVariable"];
	        this.mode = source["mode"];
	        this.numGPUs = source["numGPUs"];
	        this.minVRAMMB = source["minVRAMMB"];
	        this.priority = source["priority"];
	    }
	}
	export class TerminalResult {
	    output: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new TerminalResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.output = source["output"];
	        this.error = source["error"];
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
	export class UninstallResult {
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new UninstallResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class UpdateInfo {
	    current: string;
	    latest: string;
	    available: boolean;
	    notes: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.notes = source["notes"];
	    }
	}

}

