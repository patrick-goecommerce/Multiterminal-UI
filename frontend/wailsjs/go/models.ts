export namespace backend {
	
	export class ClaudeDetectResult {
	    path: string;
	    source: string;
	    valid: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ClaudeDetectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.source = source["source"];
	        this.valid = source["valid"];
	    }
	}
	export class FileContent {
	    path: string;
	    name: string;
	    content: string;
	    size: number;
	    error: string;
	    binary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.size = source["size"];
	        this.error = source["error"];
	        this.binary = source["binary"];
	    }
	}
	export class FileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	    }
	}
	export class HealthInfo {
	    crash_detected: boolean;
	    logging_enabled: boolean;
	    logging_auto: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HealthInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.crash_detected = source["crash_detected"];
	        this.logging_enabled = source["logging_enabled"];
	        this.logging_auto = source["logging_auto"];
	    }
	}
	export class Issue {
	    number: number;
	    title: string;
	    state: string;
	    author: string;
	    labels: string[];
	    body: string;
	    createdAt: string;
	    updatedAt: string;
	    comments: number;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.author = source["author"];
	        this.labels = source["labels"];
	        this.body = source["body"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.comments = source["comments"];
	        this.url = source["url"];
	    }
	}
	export class IssueBranchInfo {
	    on_issue_branch: boolean;
	    branch_name: string;
	    issue_number: number;
	    is_same_issue: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IssueBranchInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.on_issue_branch = source["on_issue_branch"];
	        this.branch_name = source["branch_name"];
	        this.issue_number = source["issue_number"];
	        this.is_same_issue = source["is_same_issue"];
	    }
	}
	export class IssueComment {
	    author: string;
	    body: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new IssueComment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.author = source["author"];
	        this.body = source["body"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class IssueDetail {
	    number: number;
	    title: string;
	    state: string;
	    author: string;
	    labels: string[];
	    body: string;
	    createdAt: string;
	    updatedAt: string;
	    assignees: string[];
	    url: string;
	    comments: IssueComment[];
	
	    static createFrom(source: any = {}) {
	        return new IssueDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.author = source["author"];
	        this.labels = source["labels"];
	        this.body = source["body"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.assignees = source["assignees"];
	        this.url = source["url"];
	        this.comments = this.convertValues(source["comments"], IssueComment);
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
	export class IssueLabel {
	    name: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new IssueLabel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.color = source["color"];
	    }
	}
	export class MergeConflictInfo {
	    files: string[];
	    operation: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new MergeConflictInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = source["files"];
	        this.operation = source["operation"];
	        this.count = source["count"];
	    }
	}
	export class QueueItem {
	    id: number;
	    prompt: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new QueueItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.prompt = source["prompt"];
	        this.status = source["status"];
	    }
	}
	export class UpdateInfo {
	    currentVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	    downloadURL: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.downloadURL = source["downloadURL"];
	    }
	}
	export class WorktreeInfo {
	    path: string;
	    branch: string;
	    issue: number;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.branch = source["branch"];
	        this.issue = source["issue"];
	    }
	}

}

export namespace config {
	
	export class AudioSettings {
	    enabled?: boolean;
	    volume: number;
	    when_focused?: boolean;
	    done_sound: string;
	    input_sound: string;
	    error_sound: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.volume = source["volume"];
	        this.when_focused = source["when_focused"];
	        this.done_sound = source["done_sound"];
	        this.input_sound = source["input_sound"];
	        this.error_sound = source["error_sound"];
	    }
	}
	export class CommandEntry {
	    name: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new CommandEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.text = source["text"];
	    }
	}
	export class IssueTracking {
	    auto_comment_on_start: boolean;
	    auto_comment_on_done: boolean;
	    auto_comment_on_close: boolean;
	    auto_close_issue: boolean;
	    include_cost_in_report: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IssueTracking(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.auto_comment_on_start = source["auto_comment_on_start"];
	        this.auto_comment_on_done = source["auto_comment_on_done"];
	        this.auto_comment_on_close = source["auto_comment_on_close"];
	        this.auto_close_issue = source["auto_close_issue"];
	        this.include_cost_in_report = source["include_cost_in_report"];
	    }
	}
	export class ModelEntry {
	    label: string;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.id = source["id"];
	    }
	}
	export class Config {
	    default_shell: string;
	    default_dir: string;
	    theme: string;
	    terminal_color: string;
	    max_panes_per_tab: number;
	    sidebar_width: number;
	    claude_command: string;
	    claude_models: ModelEntry[];
	    commit_reminder_minutes: number;
	    restore_session?: boolean;
	    logging_enabled: boolean;
	    auto_branch_on_issue?: boolean;
	    use_worktrees?: boolean;
	    issue_tracking: IssueTracking;
	    commands: CommandEntry[];
	    audio: AudioSettings;
	    localhost_auto_open: string;
	    sidebar_pinned: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.default_shell = source["default_shell"];
	        this.default_dir = source["default_dir"];
	        this.theme = source["theme"];
	        this.terminal_color = source["terminal_color"];
	        this.max_panes_per_tab = source["max_panes_per_tab"];
	        this.sidebar_width = source["sidebar_width"];
	        this.claude_command = source["claude_command"];
	        this.claude_models = this.convertValues(source["claude_models"], ModelEntry);
	        this.commit_reminder_minutes = source["commit_reminder_minutes"];
	        this.restore_session = source["restore_session"];
	        this.logging_enabled = source["logging_enabled"];
	        this.auto_branch_on_issue = source["auto_branch_on_issue"];
	        this.use_worktrees = source["use_worktrees"];
	        this.issue_tracking = this.convertValues(source["issue_tracking"], IssueTracking);
	        this.commands = this.convertValues(source["commands"], CommandEntry);
	        this.audio = this.convertValues(source["audio"], AudioSettings);
	        this.localhost_auto_open = source["localhost_auto_open"];
	        this.sidebar_pinned = source["sidebar_pinned"];
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
	
	
	export class SavedPane {
	    name: string;
	    mode: number;
	    model: string;
	    issue_number?: number;
	    issue_branch?: string;
	
	    static createFrom(source: any = {}) {
	        return new SavedPane(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.model = source["model"];
	        this.issue_number = source["issue_number"];
	        this.issue_branch = source["issue_branch"];
	    }
	}
	export class SavedTab {
	    name: string;
	    dir: string;
	    focus_idx: number;
	    panes: SavedPane[];
	
	    static createFrom(source: any = {}) {
	        return new SavedTab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dir = source["dir"];
	        this.focus_idx = source["focus_idx"];
	        this.panes = this.convertValues(source["panes"], SavedPane);
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
	export class SessionState {
	    active_tab: number;
	    tabs: SavedTab[];
	
	    static createFrom(source: any = {}) {
	        return new SessionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active_tab = source["active_tab"];
	        this.tabs = this.convertValues(source["tabs"], SavedTab);
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

