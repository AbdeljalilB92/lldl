export namespace app {
	
	export class AuthResponse {
	    success: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	    }
	}
	export class VideoResponse {
	    title: string;
	    slug: string;
	    duration: number;
	    downloadUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new VideoResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.slug = source["slug"];
	        this.duration = source["duration"];
	        this.downloadUrl = source["downloadUrl"];
	    }
	}
	export class ChapterResponse {
	    title: string;
	    slug: string;
	    videos: VideoResponse[];
	
	    static createFrom(source: any = {}) {
	        return new ChapterResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.slug = source["slug"];
	        this.videos = this.convertValues(source["videos"], VideoResponse);
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
	export class ConfigResponse {
	    token: string;
	    quality: string;
	    outputDir: string;
	    courseUrl: string;
	    found: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConfigResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.quality = source["quality"];
	        this.outputDir = source["outputDir"];
	        this.courseUrl = source["courseUrl"];
	        this.found = source["found"];
	    }
	}
	export class CourseResponse {
	    title: string;
	    chapterCount: number;
	    videoCount: number;
	    chapters: ChapterResponse[];
	    hasExercises: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CourseResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.chapterCount = source["chapterCount"];
	        this.videoCount = source["videoCount"];
	        this.chapters = this.convertValues(source["chapters"], ChapterResponse);
	        this.hasExercises = source["hasExercises"];
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
	export class SaveConfigRequest {
	    token: string;
	    quality: string;
	    outputDir: string;
	    courseUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveConfigRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.quality = source["quality"];
	        this.outputDir = source["outputDir"];
	        this.courseUrl = source["courseUrl"];
	    }
	}

}

