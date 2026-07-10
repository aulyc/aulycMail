export namespace account {

	export class Account {
	    id: string;
	    name: string;
	    email: string;
	    sharedMailboxParentId?: string;
	    imapHost: string;
	    imapPort: number;
	    imapSecurity: string;
	    smtpHost: string;
	    smtpPort: number;
	    smtpSecurity: string;
	    noOutgoingServer: boolean;
	    smtpUsername: string;
	    replyForwardIdentityId: string;
	    authType: string;
	    username: string;
	    enabled: boolean;
	    orderIndex: number;
	    color: string;
	    syncPeriodDays: number;
	    localRetentionDays: number;
	    syncStrategy: string;
	    fullCheckIntervalDays: number;
	    bodyDownloadPolicy: string;
	    bodyDownloadDays: number;
	    syncInterval: number;
	    syncAllFolders: boolean;
	    syncFoldersEnabled: boolean;
	    readReceiptRequestPolicy: string;
	    sentFolderPath?: string;
	    draftsFolderPath?: string;
	    trashFolderPath?: string;
	    spamFolderPath?: string;
	    archiveFolderPath?: string;
	    allMailFolderPath?: string;
	    starredFolderPath?: string;
	    createdAt: string;
	    updatedAt: string;

	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.email = source["email"];
	        this.sharedMailboxParentId = source["sharedMailboxParentId"];
	        this.imapHost = source["imapHost"];
	        this.imapPort = source["imapPort"];
	        this.imapSecurity = source["imapSecurity"];
	        this.smtpHost = source["smtpHost"];
	        this.smtpPort = source["smtpPort"];
	        this.smtpSecurity = source["smtpSecurity"];
	        this.noOutgoingServer = source["noOutgoingServer"];
	        this.smtpUsername = source["smtpUsername"];
	        this.replyForwardIdentityId = source["replyForwardIdentityId"];
	        this.authType = source["authType"];
	        this.username = source["username"];
	        this.enabled = source["enabled"];
	        this.orderIndex = source["orderIndex"];
	        this.color = source["color"];
	        this.syncPeriodDays = source["syncPeriodDays"];
	        this.localRetentionDays = source["localRetentionDays"];
	        this.syncStrategy = source["syncStrategy"];
	        this.fullCheckIntervalDays = source["fullCheckIntervalDays"];
	        this.bodyDownloadPolicy = source["bodyDownloadPolicy"];
	        this.bodyDownloadDays = source["bodyDownloadDays"];
	        this.syncInterval = source["syncInterval"];
	        this.syncAllFolders = source["syncAllFolders"];
	        this.syncFoldersEnabled = source["syncFoldersEnabled"];
	        this.readReceiptRequestPolicy = source["readReceiptRequestPolicy"];
	        this.sentFolderPath = source["sentFolderPath"];
	        this.draftsFolderPath = source["draftsFolderPath"];
	        this.trashFolderPath = source["trashFolderPath"];
	        this.spamFolderPath = source["spamFolderPath"];
	        this.archiveFolderPath = source["archiveFolderPath"];
	        this.allMailFolderPath = source["allMailFolderPath"];
	        this.starredFolderPath = source["starredFolderPath"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class AccountConfig {
	    name: string;
	    displayName: string;
	    email: string;
	    sharedMailboxParentId?: string;
	    imapHost: string;
	    imapPort: number;
	    imapSecurity: string;
	    smtpHost: string;
	    smtpPort: number;
	    smtpSecurity: string;
	    noOutgoingServer: boolean;
	    smtpUsername: string;
	    smtpPassword: string;
	    replyForwardIdentityId: string;
	    authType: string;
	    username: string;
	    password: string;
	    color: string;
	    syncPeriodDays: number;
	    localRetentionDays: number;
	    syncStrategy: string;
	    fullCheckIntervalDays: number;
	    bodyDownloadPolicy: string;
	    bodyDownloadDays: number;
	    syncInterval: number;
	    syncAllFolders: boolean;
	    syncFoldersEnabled: boolean;
	    readReceiptRequestPolicy: string;
	    sentFolderPath?: string;
	    draftsFolderPath?: string;
	    trashFolderPath?: string;
	    spamFolderPath?: string;
	    archiveFolderPath?: string;
	    allMailFolderPath?: string;
	    starredFolderPath?: string;

	    static createFrom(source: any = {}) {
	        return new AccountConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.email = source["email"];
	        this.sharedMailboxParentId = source["sharedMailboxParentId"];
	        this.imapHost = source["imapHost"];
	        this.imapPort = source["imapPort"];
	        this.imapSecurity = source["imapSecurity"];
	        this.smtpHost = source["smtpHost"];
	        this.smtpPort = source["smtpPort"];
	        this.smtpSecurity = source["smtpSecurity"];
	        this.noOutgoingServer = source["noOutgoingServer"];
	        this.smtpUsername = source["smtpUsername"];
	        this.smtpPassword = source["smtpPassword"];
	        this.replyForwardIdentityId = source["replyForwardIdentityId"];
	        this.authType = source["authType"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.color = source["color"];
	        this.syncPeriodDays = source["syncPeriodDays"];
	        this.localRetentionDays = source["localRetentionDays"];
	        this.syncStrategy = source["syncStrategy"];
	        this.fullCheckIntervalDays = source["fullCheckIntervalDays"];
	        this.bodyDownloadPolicy = source["bodyDownloadPolicy"];
	        this.bodyDownloadDays = source["bodyDownloadDays"];
	        this.syncInterval = source["syncInterval"];
	        this.syncAllFolders = source["syncAllFolders"];
	        this.syncFoldersEnabled = source["syncFoldersEnabled"];
	        this.readReceiptRequestPolicy = source["readReceiptRequestPolicy"];
	        this.sentFolderPath = source["sentFolderPath"];
	        this.draftsFolderPath = source["draftsFolderPath"];
	        this.trashFolderPath = source["trashFolderPath"];
	        this.spamFolderPath = source["spamFolderPath"];
	        this.archiveFolderPath = source["archiveFolderPath"];
	        this.allMailFolderPath = source["allMailFolderPath"];
	        this.starredFolderPath = source["starredFolderPath"];
	    }
	}
	export class Identity {
	    id: string;
	    accountId: string;
	    email: string;
	    name: string;
	    isDefault: boolean;
	    signatureHtml?: string;
	    signatureText?: string;
	    signatureEnabled: boolean;
	    signatureForNew: boolean;
	    signatureForReply: boolean;
	    signatureForForward: boolean;
	    signaturePlacement: string;
	    signatureSeparator: boolean;
	    signatureSeparatorStyle: string;
	    orderIndex: number;
	    createdAt: string;
	    updatedAt: string;

	    static createFrom(source: any = {}) {
	        return new Identity(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.email = source["email"];
	        this.name = source["name"];
	        this.isDefault = source["isDefault"];
	        this.signatureHtml = source["signatureHtml"];
	        this.signatureText = source["signatureText"];
	        this.signatureEnabled = source["signatureEnabled"];
	        this.signatureForNew = source["signatureForNew"];
	        this.signatureForReply = source["signatureForReply"];
	        this.signatureForForward = source["signatureForForward"];
	        this.signaturePlacement = source["signaturePlacement"];
	        this.signatureSeparator = source["signatureSeparator"];
	        this.signatureSeparatorStyle = source["signatureSeparatorStyle"];
	        this.orderIndex = source["orderIndex"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class IdentityConfig {
	    email: string;
	    name: string;
	    signatureHtml?: string;
	    signatureText?: string;
	    signatureEnabled: boolean;
	    signatureForNew: boolean;
	    signatureForReply: boolean;
	    signatureForForward: boolean;
	    signaturePlacement: string;
	    signatureSeparator: boolean;
	    signatureSeparatorStyle: string;

	    static createFrom(source: any = {}) {
	        return new IdentityConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.name = source["name"];
	        this.signatureHtml = source["signatureHtml"];
	        this.signatureText = source["signatureText"];
	        this.signatureEnabled = source["signatureEnabled"];
	        this.signatureForNew = source["signatureForNew"];
	        this.signatureForReply = source["signatureForReply"];
	        this.signatureForForward = source["signatureForForward"];
	        this.signaturePlacement = source["signaturePlacement"];
	        this.signatureSeparator = source["signatureSeparator"];
	        this.signatureSeparatorStyle = source["signatureSeparatorStyle"];
	    }
	}

}
export namespace app {

	export class AccountIdentityGroup {
	    account?: account.Account;
	    identities: account.Identity[];

	    static createFrom(source: any = {}) {
	        return new AccountIdentityGroup(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.account = this.convertValues(source["account"], account.Account);
	        this.identities = this.convertValues(source["identities"], account.Identity);
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
	export class AppInfo {
	    name: string;
	    version: string;
	    description: string;
	    website: string;
	    license: string;

	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.website = source["website"];
	        this.license = source["license"];
	    }
	}
	export class BackupProgress {
	    phase: string;
	    accountEmail?: string;
	    folderPath?: string;
	    current: number;
	    total: number;
	    exported: number;
	    skipped: number;
	    missing: number;
	    failed: number;
	    message?: string;

	    static createFrom(source: any = {}) {
	        return new BackupProgress(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.accountEmail = source["accountEmail"];
	        this.folderPath = source["folderPath"];
	        this.current = source["current"];
	        this.total = source["total"];
	        this.exported = source["exported"];
	        this.skipped = source["skipped"];
	        this.missing = source["missing"];
	        this.failed = source["failed"];
	        this.message = source["message"];
	    }
	}
	export class BackupRunOptions {
	    directory: string;
	    scope: string;
	    selectedAccountIds: string[];

	    static createFrom(source: any = {}) {
	        return new BackupRunOptions(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.scope = source["scope"];
	        this.selectedAccountIds = source["selectedAccountIds"];
	    }
	}
	export class BackupRunResult {
	    directory: string;
	    mode: string;
	    total: number;
	    exported: number;
	    skipped: number;
	    missing: number;
	    failed: number;
	    reportPath?: string;

	    static createFrom(source: any = {}) {
	        return new BackupRunResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.mode = source["mode"];
	        this.total = source["total"];
	        this.exported = source["exported"];
	        this.skipped = source["skipped"];
	        this.missing = source["missing"];
	        this.failed = source["failed"];
	        this.reportPath = source["reportPath"];
	    }
	}
	export class BackupRunState {
	    running: boolean;
	    startedAt?: string;
	    progress?: BackupProgress;

	    static createFrom(source: any = {}) {
	        return new BackupRunState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.startedAt = source["startedAt"];
	        this.progress = this.convertValues(source["progress"], BackupProgress);
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
	export class BackupSettings {
	    directory: string;
	    scope: string;
	    selectedAccountIds: string[];

	    static createFrom(source: any = {}) {
	        return new BackupSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.scope = source["scope"];
	        this.selectedAccountIds = source["selectedAccountIds"];
	    }
	}
	export class BackupStatus {
	    directory: string;
	    mode: string;
	    hasIndex: boolean;
	    messageCount: number;
	    lastRunAt?: string;
	    lastRunMode?: string;
	    lastRunResult?: string;

	    static createFrom(source: any = {}) {
	        return new BackupStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.mode = source["mode"];
	        this.hasIndex = source["hasIndex"];
	        this.messageCount = source["messageCount"];
	        this.lastRunAt = source["lastRunAt"];
	        this.lastRunMode = source["lastRunMode"];
	        this.lastRunResult = source["lastRunResult"];
	    }
	}
	export class BackupViewerAccount {
	    accountEmail: string;
	    messageCount: number;

	    static createFrom(source: any = {}) {
	        return new BackupViewerAccount(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountEmail = source["accountEmail"];
	        this.messageCount = source["messageCount"];
	    }
	}
	export class BackupViewerAttachment {
	    index: number;
	    filename: string;
	    contentType: string;
	    size: number;
	    inline: boolean;

	    static createFrom(source: any = {}) {
	        return new BackupViewerAttachment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.filename = source["filename"];
	        this.contentType = source["contentType"];
	        this.size = source["size"];
	        this.inline = source["inline"];
	    }
	}
	export class BackupViewerMessageSummary {
	    key: string;
	    accountEmail: string;
	    folderPath: string;
	    subject: string;
	    date: string;
	    size: number;
	    attachmentCount: number;
	    snippet?: string;

	    static createFrom(source: any = {}) {
	        return new BackupViewerMessageSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.accountEmail = source["accountEmail"];
	        this.folderPath = source["folderPath"];
	        this.subject = source["subject"];
	        this.date = source["date"];
	        this.size = source["size"];
	        this.attachmentCount = source["attachmentCount"];
	        this.snippet = source["snippet"];
	    }
	}
	export class BackupViewerCatalog {
	    directory: string;
	    accounts: BackupViewerAccount[];
	    messages: BackupViewerMessageSummary[];
	    messageCount: number;
	    indexReady: boolean;
	    needsIndex: boolean;

	    static createFrom(source: any = {}) {
	        return new BackupViewerCatalog(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.accounts = this.convertValues(source["accounts"], BackupViewerAccount);
	        this.messages = this.convertValues(source["messages"], BackupViewerMessageSummary);
	        this.messageCount = source["messageCount"];
	        this.indexReady = source["indexReady"];
	        this.needsIndex = source["needsIndex"];
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
	export class BackupViewerMessageDetail {
	    key: string;
	    accountEmail: string;
	    folderPath: string;
	    subject: string;
	    date: string;
	    from: string[];
	    to: string[];
	    cc: string[];
	    bcc: string[];
	    bodyHTML: string;
	    bodyText: string;
	    hasHTML: boolean;
	    size: number;
	    attachments: BackupViewerAttachment[];

	    static createFrom(source: any = {}) {
	        return new BackupViewerMessageDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.accountEmail = source["accountEmail"];
	        this.folderPath = source["folderPath"];
	        this.subject = source["subject"];
	        this.date = source["date"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.cc = source["cc"];
	        this.bcc = source["bcc"];
	        this.bodyHTML = source["bodyHTML"];
	        this.bodyText = source["bodyText"];
	        this.hasHTML = source["hasHTML"];
	        this.size = source["size"];
	        this.attachments = this.convertValues(source["attachments"], BackupViewerAttachment);
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
	export class BackupViewerMessagePage {
	    messages: BackupViewerMessageSummary[];
	    total: number;
	    hasMore: boolean;

	    static createFrom(source: any = {}) {
	        return new BackupViewerMessagePage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], BackupViewerMessageSummary);
	        this.total = source["total"];
	        this.hasMore = source["hasMore"];
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

	export class ComposerAttachment {
	    filename: string;
	    contentType: string;
	    size: number;
	    data: string;

	    static createFrom(source: any = {}) {
	        return new ComposerAttachment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.contentType = source["contentType"];
	        this.size = source["size"];
	        this.data = source["data"];
	    }
	}
	export class ConnectionTestResult {
	    success: boolean;
	    error?: string;
	    certificateRequired: boolean;
	    certificate?: certificate.CertificateInfo;

	    static createFrom(source: any = {}) {
	        return new ConnectionTestResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.certificateRequired = source["certificateRequired"];
	        this.certificate = this.convertValues(source["certificate"], certificate.CertificateInfo);
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
	export class DraftResult {
	    draft?: draft.Draft;

	    static createFrom(source: any = {}) {
	        return new DraftResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.draft = this.convertValues(source["draft"], draft.Draft);
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
	export class MailtoData {
	    to: string[];
	    cc: string[];
	    bcc: string[];
	    subject: string;
	    body: string;

	    static createFrom(source: any = {}) {
	        return new MailtoData(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.to = source["to"];
	        this.cc = source["cc"];
	        this.bcc = source["bcc"];
	        this.subject = source["subject"];
	        this.body = source["body"];
	    }
	}
	export class MessageSourceResult {
	    content?: string;
	    filePath?: string;
	    size: number;
	    tooLarge: boolean;

	    static createFrom(source: any = {}) {
	        return new MessageSourceResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.filePath = source["filePath"];
	        this.size = source["size"];
	        this.tooLarge = source["tooLarge"];
	    }
	}
	export class OfflineBodyCacheClearResult {
	    folders: number;
	    bodiesCleared: number;
	    attachmentsDeleted: number;

	    static createFrom(source: any = {}) {
	        return new OfflineBodyCacheClearResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folders = source["folders"];
	        this.bodiesCleared = source["bodiesCleared"];
	        this.attachmentsDeleted = source["attachmentsDeleted"];
	    }
	}

}

export namespace appstate {

	export class UIState {
	    selectedAccountId: string;
	    selectedFolderId: string;
	    selectedFolderName: string;
	    selectedFolderType: string;
	    selectedThreadId: string;
	    selectedConversationAccountId: string;
	    selectedConversationFolderId: string;
	    sidebarWidth: number;
	    listWidth: number;
	    expandedAccounts: Record<string, boolean>;
	    unifiedInboxExpanded: boolean;
	    collapsedFolders: Record<string, boolean>;
	    activeExtension?: string;

	    static createFrom(source: any = {}) {
	        return new UIState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selectedAccountId = source["selectedAccountId"];
	        this.selectedFolderId = source["selectedFolderId"];
	        this.selectedFolderName = source["selectedFolderName"];
	        this.selectedFolderType = source["selectedFolderType"];
	        this.selectedThreadId = source["selectedThreadId"];
	        this.selectedConversationAccountId = source["selectedConversationAccountId"];
	        this.selectedConversationFolderId = source["selectedConversationFolderId"];
	        this.sidebarWidth = source["sidebarWidth"];
	        this.listWidth = source["listWidth"];
	        this.expandedAccounts = source["expandedAccounts"];
	        this.unifiedInboxExpanded = source["unifiedInboxExpanded"];
	        this.collapsedFolders = source["collapsedFolders"];
	        this.activeExtension = source["activeExtension"];
	    }
	}

}

export namespace certificate {

	export class CertificateInfo {
	    host?: string;
	    subject: string;
	    issuer: string;
	    fingerprint: string;
	    notBefore: string;
	    notAfter: string;
	    dnsNames: string[];
	    isExpired: boolean;
	    errorReason: string;

	    static createFrom(source: any = {}) {
	        return new CertificateInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.subject = source["subject"];
	        this.issuer = source["issuer"];
	        this.fingerprint = source["fingerprint"];
	        this.notBefore = source["notBefore"];
	        this.notAfter = source["notAfter"];
	        this.dnsNames = source["dnsNames"];
	        this.isExpired = source["isExpired"];
	        this.errorReason = source["errorReason"];
	    }
	}

}

export namespace contact {

	export class Contact {
	    email: string;
	    display_name: string;
	    source: string;
	    kind?: string;
	    avatar_url?: string;
	    send_count: number;
	    last_used: string;
	    created_at: string;

	    static createFrom(source: any = {}) {
	        return new Contact(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.display_name = source["display_name"];
	        this.source = source["source"];
	        this.kind = source["kind"];
	        this.avatar_url = source["avatar_url"];
	        this.send_count = source["send_count"];
	        this.last_used = source["last_used"];
	        this.created_at = source["created_at"];
	    }
	}

}

export namespace contactdto {

	export class ContactIMPP {
	    handle: string;
	    type?: string;

	    static createFrom(source: any = {}) {
	        return new ContactIMPP(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.handle = source["handle"];
	        this.type = source["type"];
	    }
	}
	export class ContactURL {
	    url: string;
	    type?: string;

	    static createFrom(source: any = {}) {
	        return new ContactURL(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.type = source["type"];
	    }
	}
	export class ContactAddress {
	    type?: string;
	    street?: string;
	    city?: string;
	    region?: string;
	    postcode?: string;
	    country?: string;

	    static createFrom(source: any = {}) {
	        return new ContactAddress(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.street = source["street"];
	        this.city = source["city"];
	        this.region = source["region"];
	        this.postcode = source["postcode"];
	        this.country = source["country"];
	    }
	}
	export class ContactPhone {
	    number: string;
	    type?: string;
	    isPrimary?: boolean;

	    static createFrom(source: any = {}) {
	        return new ContactPhone(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.type = source["type"];
	        this.isPrimary = source["isPrimary"];
	    }
	}
	export class ContactAssociatedAccount {
	    accountId: string;
	    name?: string;
	    email: string;

	    static createFrom(source: any = {}) {
	        return new ContactAssociatedAccount(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.name = source["name"];
	        this.email = source["email"];
	    }
	}
	export class ContactEmail {
	    email: string;
	    type?: string;
	    isPrimary?: boolean;

	    static createFrom(source: any = {}) {
	        return new ContactEmail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.type = source["type"];
	        this.isPrimary = source["isPrimary"];
	    }
	}
	export class Contact {
	    id: string;
	    name: string;
	    emails: string[];
	    emailItems?: ContactEmail[];
	    associatedAccounts?: ContactAssociatedAccount[];
	    phones?: ContactPhone[];
	    addresses?: ContactAddress[];
	    urls?: ContactURL[];
	    impps?: ContactIMPP[];
	    org?: string;
	    title?: string;
	    note?: string;
	    bday?: string;
	    nickname?: string;
	    categories?: string[];
	    photoData?: string;
	    photoMediaType?: string;
	    photoUrl?: string;
	    sourceId?: string;
	    updatedAt: string;

	    static createFrom(source: any = {}) {
	        return new Contact(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.emails = source["emails"];
	        this.emailItems = this.convertValues(source["emailItems"], ContactEmail);
	        this.associatedAccounts = this.convertValues(source["associatedAccounts"], ContactAssociatedAccount);
	        this.phones = this.convertValues(source["phones"], ContactPhone);
	        this.addresses = this.convertValues(source["addresses"], ContactAddress);
	        this.urls = this.convertValues(source["urls"], ContactURL);
	        this.impps = this.convertValues(source["impps"], ContactIMPP);
	        this.org = source["org"];
	        this.title = source["title"];
	        this.note = source["note"];
	        this.bday = source["bday"];
	        this.nickname = source["nickname"];
	        this.categories = source["categories"];
	        this.photoData = source["photoData"];
	        this.photoMediaType = source["photoMediaType"];
	        this.photoUrl = source["photoUrl"];
	        this.sourceId = source["sourceId"];
	        this.updatedAt = source["updatedAt"];
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
	export class ContactAccountGroup {
	    accountId: string;
	    name?: string;
	    email: string;
	    count: number;
	    senderCount: number;
	    recipientCount: number;
	    ccCount: number;
	    bccCount: number;

	    static createFrom(source: any = {}) {
	        return new ContactAccountGroup(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.name = source["name"];
	        this.email = source["email"];
	        this.count = source["count"];
	        this.senderCount = source["senderCount"];
	        this.recipientCount = source["recipientCount"];
	        this.ccCount = source["ccCount"];
	        this.bccCount = source["bccCount"];
	    }
	}


	export class ContactBrowseResult {
	    items: Contact[];
	    total: number;

	    static createFrom(source: any = {}) {
	        return new ContactBrowseResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Contact);
	        this.total = source["total"];
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
	export class ContactPhoto {
	    data?: string;
	    mediaType?: string;
	    url?: string;

	    static createFrom(source: any = {}) {
	        return new ContactPhoto(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data = source["data"];
	        this.mediaType = source["mediaType"];
	        this.url = source["url"];
	    }
	}
	export class ContactCreateInput {
	    sourceId?: string;
	    email: string;
	    name?: string;
	    nickname?: string;
	    org?: string;
	    title?: string;
	    note?: string;
	    bday?: string;
	    categories?: string[];
	    emails?: ContactEmail[];
	    phones?: ContactPhone[];
	    addresses?: ContactAddress[];
	    urls?: ContactURL[];
	    impps?: ContactIMPP[];
	    photo?: ContactPhoto;

	    static createFrom(source: any = {}) {
	        return new ContactCreateInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.email = source["email"];
	        this.name = source["name"];
	        this.nickname = source["nickname"];
	        this.org = source["org"];
	        this.title = source["title"];
	        this.note = source["note"];
	        this.bday = source["bday"];
	        this.categories = source["categories"];
	        this.emails = this.convertValues(source["emails"], ContactEmail);
	        this.phones = this.convertValues(source["phones"], ContactPhone);
	        this.addresses = this.convertValues(source["addresses"], ContactAddress);
	        this.urls = this.convertValues(source["urls"], ContactURL);
	        this.impps = this.convertValues(source["impps"], ContactIMPP);
	        this.photo = this.convertValues(source["photo"], ContactPhoto);
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


	export class ContactPatch {
	    name?: string;
	    nickname?: string;
	    org?: string;
	    title?: string;
	    note?: string;
	    bday?: string;
	    emails?: ContactEmail[];
	    phones?: ContactPhone[];
	    addresses?: ContactAddress[];
	    urls?: ContactURL[];
	    impps?: ContactIMPP[];
	    categories?: string[];
	    photo?: ContactPhoto;

	    static createFrom(source: any = {}) {
	        return new ContactPatch(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.nickname = source["nickname"];
	        this.org = source["org"];
	        this.title = source["title"];
	        this.note = source["note"];
	        this.bday = source["bday"];
	        this.emails = this.convertValues(source["emails"], ContactEmail);
	        this.phones = this.convertValues(source["phones"], ContactPhone);
	        this.addresses = this.convertValues(source["addresses"], ContactAddress);
	        this.urls = this.convertValues(source["urls"], ContactURL);
	        this.impps = this.convertValues(source["impps"], ContactIMPP);
	        this.categories = source["categories"];
	        this.photo = this.convertValues(source["photo"], ContactPhoto);
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

export namespace draft {

	export class Draft {
	    id: string;
	    accountId: string;
	    toList: string;
	    ccList: string;
	    bccList: string;
	    subject: string;
	    bodyHtml: string;
	    bodyText: string;
	    inReplyToId?: string;
	    sourceMessageId?: string;
	    replyType?: string;
	    referencesList?: string;
	    identityId?: string;
	    syncStatus: string;
	    imapUid?: number;
	    folderId?: string;
	    lastSyncAttempt?: string;
	    syncError?: string;
	    createdAt: string;
	    updatedAt: string;

	    static createFrom(source: any = {}) {
	        return new Draft(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.toList = source["toList"];
	        this.ccList = source["ccList"];
	        this.bccList = source["bccList"];
	        this.subject = source["subject"];
	        this.bodyHtml = source["bodyHtml"];
	        this.bodyText = source["bodyText"];
	        this.inReplyToId = source["inReplyToId"];
	        this.sourceMessageId = source["sourceMessageId"];
	        this.replyType = source["replyType"];
	        this.referencesList = source["referencesList"];
	        this.identityId = source["identityId"];
	        this.syncStatus = source["syncStatus"];
	        this.imapUid = source["imapUid"];
	        this.folderId = source["folderId"];
	        this.lastSyncAttempt = source["lastSyncAttempt"];
	        this.syncError = source["syncError"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

export namespace folder {

	export class Folder {
	    id: string;
	    accountId: string;
	    name: string;
	    path: string;
	    type: string;
	    parentId?: string;
	    uidValidity: number;
	    uidNext: number;
	    highestModSeq: number;
	    totalCount: number;
	    unreadCount: number;
	    lastSync?: string;
	    lastFullSync?: string;
	    subscribed: boolean;

	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.type = source["type"];
	        this.parentId = source["parentId"];
	        this.uidValidity = source["uidValidity"];
	        this.uidNext = source["uidNext"];
	        this.highestModSeq = source["highestModSeq"];
	        this.totalCount = source["totalCount"];
	        this.unreadCount = source["unreadCount"];
	        this.lastSync = source["lastSync"];
	        this.lastFullSync = source["lastFullSync"];
	        this.subscribed = source["subscribed"];
	    }
	}
	export class FolderTree {
	    folder?: Folder;
	    children?: FolderTree[];

	    static createFrom(source: any = {}) {
	        return new FolderTree(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = this.convertValues(source["folder"], Folder);
	        this.children = this.convertValues(source["children"], FolderTree);
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

export namespace message {

	export class Address {
	    name: string;
	    email: string;

	    static createFrom(source: any = {}) {
	        return new Address(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	    }
	}
	export class Attachment {
	    id: string;
	    messageId: string;
	    filename: string;
	    contentType: string;
	    size: number;
	    contentId?: string;
	    isInline: boolean;
	    localPath?: string;

	    static createFrom(source: any = {}) {
	        return new Attachment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.messageId = source["messageId"];
	        this.filename = source["filename"];
	        this.contentType = source["contentType"];
	        this.size = source["size"];
	        this.contentId = source["contentId"];
	        this.isInline = source["isInline"];
	        this.localPath = source["localPath"];
	    }
	}
	export class ContactMessage {
	    id: string;
	    threadId: string;
	    accountId: string;
	    accountName?: string;
	    accountEmail?: string;
	    folderId: string;
	    subject: string;
	    fromName: string;
	    fromEmail: string;
	    date: string;
	    isRead: boolean;
	    incoming: boolean;
	    snippet: string;

	    static createFrom(source: any = {}) {
	        return new ContactMessage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.threadId = source["threadId"];
	        this.accountId = source["accountId"];
	        this.accountName = source["accountName"];
	        this.accountEmail = source["accountEmail"];
	        this.folderId = source["folderId"];
	        this.subject = source["subject"];
	        this.fromName = source["fromName"];
	        this.fromEmail = source["fromEmail"];
	        this.date = source["date"];
	        this.isRead = source["isRead"];
	        this.incoming = source["incoming"];
	        this.snippet = source["snippet"];
	    }
	}
	export class Message {
	    id: string;
	    accountId: string;
	    folderId: string;
	    uid: number;
	    messageId?: string;
	    inReplyTo?: string;
	    references?: string;
	    threadId?: string;
	    subject: string;
	    fromName: string;
	    fromEmail: string;
	    toList?: string;
	    ccList?: string;
	    bccList?: string;
	    replyTo?: string;
	    date: string;
	    snippet?: string;
	    isRead: boolean;
	    isStarred: boolean;
	    isAnswered: boolean;
	    isForwarded: boolean;
	    isDraft: boolean;
	    isDeleted: boolean;
	    size: number;
	    hasAttachments: boolean;
	    bodyText?: string;
	    bodyHtml?: string;
	    bodyFetched: boolean;
	    readReceiptTo?: string;
	    readReceiptHandled: boolean;
	    receivedAt: string;

	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.folderId = source["folderId"];
	        this.uid = source["uid"];
	        this.messageId = source["messageId"];
	        this.inReplyTo = source["inReplyTo"];
	        this.references = source["references"];
	        this.threadId = source["threadId"];
	        this.subject = source["subject"];
	        this.fromName = source["fromName"];
	        this.fromEmail = source["fromEmail"];
	        this.toList = source["toList"];
	        this.ccList = source["ccList"];
	        this.bccList = source["bccList"];
	        this.replyTo = source["replyTo"];
	        this.date = source["date"];
	        this.snippet = source["snippet"];
	        this.isRead = source["isRead"];
	        this.isStarred = source["isStarred"];
	        this.isAnswered = source["isAnswered"];
	        this.isForwarded = source["isForwarded"];
	        this.isDraft = source["isDraft"];
	        this.isDeleted = source["isDeleted"];
	        this.size = source["size"];
	        this.hasAttachments = source["hasAttachments"];
	        this.bodyText = source["bodyText"];
	        this.bodyHtml = source["bodyHtml"];
	        this.bodyFetched = source["bodyFetched"];
	        this.readReceiptTo = source["readReceiptTo"];
	        this.readReceiptHandled = source["readReceiptHandled"];
	        this.receivedAt = source["receivedAt"];
	    }
	}
	export class Conversation {
	    threadId: string;
	    subject: string;
	    snippet: string;
	    messageCount: number;
	    unreadCount: number;
	    hasAttachments: boolean;
	    isStarred: boolean;
	    latestDate: string;
	    participants: Address[];
	    messageIds: string[];
	    isEncrypted: boolean;
	    messages?: Message[];
	    composeStatus?: string;
	    composeAction?: string;
	    accountId?: string;
	    accountName?: string;
	    accountColor?: string;
	    folderId?: string;

	    static createFrom(source: any = {}) {
	        return new Conversation(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.threadId = source["threadId"];
	        this.subject = source["subject"];
	        this.snippet = source["snippet"];
	        this.messageCount = source["messageCount"];
	        this.unreadCount = source["unreadCount"];
	        this.hasAttachments = source["hasAttachments"];
	        this.isStarred = source["isStarred"];
	        this.latestDate = source["latestDate"];
	        this.participants = this.convertValues(source["participants"], Address);
	        this.messageIds = source["messageIds"];
	        this.isEncrypted = source["isEncrypted"];
	        this.messages = this.convertValues(source["messages"], Message);
	        this.composeStatus = source["composeStatus"];
	        this.composeAction = source["composeAction"];
	        this.accountId = source["accountId"];
	        this.accountName = source["accountName"];
	        this.accountColor = source["accountColor"];
	        this.folderId = source["folderId"];
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
	export class ConversationSearchResult {
	    threadId: string;
	    subject: string;
	    snippet: string;
	    messageCount: number;
	    unreadCount: number;
	    hasAttachments: boolean;
	    isStarred: boolean;
	    latestDate: string;
	    participants: Address[];
	    messageIds: string[];
	    isEncrypted: boolean;
	    messages?: Message[];
	    composeStatus?: string;
	    composeAction?: string;
	    accountId?: string;
	    accountName?: string;
	    accountColor?: string;
	    folderId?: string;
	    highlightedSubject: string;
	    highlightedSnippet: string;
	    highlightedFromName: string;
	    folderName: string;
	    folderType: string;

	    static createFrom(source: any = {}) {
	        return new ConversationSearchResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.threadId = source["threadId"];
	        this.subject = source["subject"];
	        this.snippet = source["snippet"];
	        this.messageCount = source["messageCount"];
	        this.unreadCount = source["unreadCount"];
	        this.hasAttachments = source["hasAttachments"];
	        this.isStarred = source["isStarred"];
	        this.latestDate = source["latestDate"];
	        this.participants = this.convertValues(source["participants"], Address);
	        this.messageIds = source["messageIds"];
	        this.isEncrypted = source["isEncrypted"];
	        this.messages = this.convertValues(source["messages"], Message);
	        this.composeStatus = source["composeStatus"];
	        this.composeAction = source["composeAction"];
	        this.accountId = source["accountId"];
	        this.accountName = source["accountName"];
	        this.accountColor = source["accountColor"];
	        this.folderId = source["folderId"];
	        this.highlightedSubject = source["highlightedSubject"];
	        this.highlightedSnippet = source["highlightedSnippet"];
	        this.highlightedFromName = source["highlightedFromName"];
	        this.folderName = source["folderName"];
	        this.folderType = source["folderType"];
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
	export class FTSIndexStatus {
	    folderId: string;
	    indexedCount: number;
	    totalCount: number;
	    isComplete: boolean;
	    lastIndexedAt?: string;

	    static createFrom(source: any = {}) {
	        return new FTSIndexStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folderId = source["folderId"];
	        this.indexedCount = source["indexedCount"];
	        this.totalCount = source["totalCount"];
	        this.isComplete = source["isComplete"];
	        this.lastIndexedAt = source["lastIndexedAt"];
	    }
	}

}

export namespace settings {

	export class AllowlistEntry {
	    id: number;
	    type: string;
	    value: string;
	    createdAt: string;

	    static createFrom(source: any = {}) {
	        return new AllowlistEntry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.value = source["value"];
	        this.createdAt = source["createdAt"];
	    }
	}

}

export namespace smtp {

	export class Address {
	    name: string;
	    address: string;

	    static createFrom(source: any = {}) {
	        return new Address(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	    }
	}
	export class Attachment {
	    filename: string;
	    content_type: string;
	    content: number[];
	    content_base64?: string;
	    content_id: string;
	    inline: boolean;

	    static createFrom(source: any = {}) {
	        return new Attachment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.content_type = source["content_type"];
	        this.content = source["content"];
	        this.content_base64 = source["content_base64"];
	        this.content_id = source["content_id"];
	        this.inline = source["inline"];
	    }
	}
	export class ComposeMessage {
	    from: Address;
	    to: Address[];
	    cc: Address[];
	    bcc: Address[];
	    reply_to?: Address;
	    subject: string;
	    text_body: string;
	    html_body: string;
	    attachments: Attachment[];
	    in_reply_to?: string;
	    references?: string[];
	    source_message_id?: string;
	    reply_type?: string;
	    request_read_receipt: boolean;

	    static createFrom(source: any = {}) {
	        return new ComposeMessage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = this.convertValues(source["from"], Address);
	        this.to = this.convertValues(source["to"], Address);
	        this.cc = this.convertValues(source["cc"], Address);
	        this.bcc = this.convertValues(source["bcc"], Address);
	        this.reply_to = this.convertValues(source["reply_to"], Address);
	        this.subject = source["subject"];
	        this.text_body = source["text_body"];
	        this.html_body = source["html_body"];
	        this.attachments = this.convertValues(source["attachments"], Attachment);
	        this.in_reply_to = source["in_reply_to"];
	        this.references = source["references"];
	        this.source_message_id = source["source_message_id"];
	        this.reply_type = source["reply_type"];
	        this.request_read_receipt = source["request_read_receipt"];
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

export namespace sync {

	export class IMAPSearchResult {
	    uid: number;
	    messageId?: string;
	    isLocal: boolean;
	    subject: string;
	    fromName: string;
	    fromEmail: string;
	    date: string;
	    snippet?: string;
	    isRead: boolean;
	    isStarred: boolean;
	    hasAttachments: boolean;
	    accountId: string;
	    folderId: string;
	    folderName?: string;

	    static createFrom(source: any = {}) {
	        return new IMAPSearchResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.messageId = source["messageId"];
	        this.isLocal = source["isLocal"];
	        this.subject = source["subject"];
	        this.fromName = source["fromName"];
	        this.fromEmail = source["fromEmail"];
	        this.date = source["date"];
	        this.snippet = source["snippet"];
	        this.isRead = source["isRead"];
	        this.isStarred = source["isStarred"];
	        this.hasAttachments = source["hasAttachments"];
	        this.accountId = source["accountId"];
	        this.folderId = source["folderId"];
	        this.folderName = source["folderName"];
	    }
	}
	export class IMAPSearchResponse {
	    results: IMAPSearchResult[];
	    totalCount: number;

	    static createFrom(source: any = {}) {
	        return new IMAPSearchResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], IMAPSearchResult);
	        this.totalCount = source["totalCount"];
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
