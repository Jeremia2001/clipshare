export namespace config {
	
	export class Config {
	    api_url: string;
	    theme: string;
	    auto_start: boolean;
	    watch_folders: string[];
	    dev_mode: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.api_url = source["api_url"];
	        this.theme = source["theme"];
	        this.auto_start = source["auto_start"];
	        this.watch_folders = source["watch_folders"];
	        this.dev_mode = source["dev_mode"];
	    }
	}

}

export namespace main {
	
	export class ProbeResult {
	    duration: number;
	    width: number;
	    height: number;
	    fps: number;
	    codec: string;
	    bitrate_kbps: number;
	
	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.duration = source["duration"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.fps = source["fps"];
	        this.codec = source["codec"];
	        this.bitrate_kbps = source["bitrate_kbps"];
	    }
	}
	export class TrimRequest {
	    input_path: string;
	    start_time: number;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new TrimRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.start_time = source["start_time"];
	        this.duration = source["duration"];
	    }
	}
	export class UploadResult {
	    clip_id: string;
	    object_key: string;
	    file_size_bytes: number;
	    file_name: string;
	
	    static createFrom(source: any = {}) {
	        return new UploadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clip_id = source["clip_id"];
	        this.object_key = source["object_key"];
	        this.file_size_bytes = source["file_size_bytes"];
	        this.file_name = source["file_name"];
	    }
	}

}

