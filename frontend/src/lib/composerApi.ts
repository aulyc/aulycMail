/**
 * Composer API Abstraction Layer
 *
 * Provides a unified interface for composer operations that works in both:
 * - Main window (modal/inline composer) - uses App bindings
 * - Detached composer window - uses ComposerApp bindings
 *
 * The API is injected via Svelte context to allow different implementations
 * depending on the window type.
 */

// @ts-ignore - Wails generated imports
import { smtp, account, contact, app } from '../../wailsjs/go/models'

/**
 * Interface for composer API operations.
 * Both App and ComposerApp implement these methods with the same signatures.
 */
export interface ComposerApi {
  /** Send a composed email */
  sendMessage: (accountId: string, message: smtp.ComposeMessage) => Promise<void>

  /** Search contacts for autocomplete */
  searchContacts: (query: string, limit: number) => Promise<contact.Contact[]>

  /** Get identities for an account */
  getIdentities: (accountId: string) => Promise<account.Identity[]>

  /** Save a draft (creates new or updates existing if draftId provided) */
  saveDraft: (accountId: string, message: smtp.ComposeMessage, draftId: string) => Promise<{ id: string; syncStatus: string }>

  /** Delete a draft */
  deleteDraft: (draftId: string) => Promise<void>

  /** Pick attachment files via native file picker */
  pickAttachmentFiles: () => Promise<app.ComposerAttachment[]>

  /** Get account details */
  getAccount: (accountId: string) => Promise<account.Account>

  /** Read a file from a filesystem path as an attachment */
  readFileAsAttachment: (filePath: string) => Promise<app.ComposerAttachment | null>

  /** Check if running inside a Flatpak sandbox */
  isFlatpak: () => Promise<boolean>

  /**
   * Get all accounts with their identities (only available in main window).
   * Returns undefined in detached composer windows.
   */
  getAllAccountIdentities?: () => Promise<app.AccountIdentityGroup[]>

  /**
   * Open a detached composer window (only available in main window).
   * Returns undefined in detached composer windows.
   */
  openComposerWindow?: (accountId: string, mode: string, messageId: string, draftId: string, mailtoURL?: string) => Promise<void>
}

/**
 * Context key for accessing the composer API.
 * Use with getContext/setContext.
 */
export const COMPOSER_API_KEY = 'composer-api'

/**
 * Creates the composer API implementation for the main window.
 * Uses App bindings.
 */
export function createMainWindowApi(): ComposerApi {
  // Dynamic import to avoid bundling issues
  // These will be resolved at runtime based on which entry point is used
  return {
    sendMessage: async (accountId: string, message: smtp.ComposeMessage) => {
      const { SendMessage } = await import('../../wailsjs/go/app/App.js')
      return SendMessage(accountId, message)
    },

    searchContacts: async (query: string, limit: number) => {
      const { SearchContacts } = await import('../../wailsjs/go/app/App.js')
      return SearchContacts(query, limit) || []
    },

    getIdentities: async (accountId: string) => {
      const { GetIdentities } = await import('../../wailsjs/go/app/App.js')
      return GetIdentities(accountId)
    },

    saveDraft: async (accountId: string, message: smtp.ComposeMessage, draftId: string) => {
      const { SaveDraft } = await import('../../wailsjs/go/app/App.js')
      const result = await SaveDraft(accountId, message, draftId)
      return { id: result?.draft?.id || '', syncStatus: result?.draft?.syncStatus || 'pending' }
    },

    deleteDraft: async (draftId: string) => {
      const { DeleteDraft } = await import('../../wailsjs/go/app/App.js')
      return DeleteDraft(draftId)
    },

    pickAttachmentFiles: async () => {
      const { PickAttachmentFiles } = await import('../../wailsjs/go/app/App.js')
      return PickAttachmentFiles()
    },

    getAccount: async (accountId: string) => {
      const { GetAccount } = await import('../../wailsjs/go/app/App.js')
      return GetAccount(accountId)
    },

    readFileAsAttachment: async (filePath: string) => {
      const { ReadFileAsAttachment } = await import('../../wailsjs/go/app/App.js')
      return ReadFileAsAttachment(filePath)
    },

    isFlatpak: async () => {
      const { IsFlatpak } = await import('../../wailsjs/go/app/App.js')
      return IsFlatpak()
    },

    getAllAccountIdentities: async () => {
      const { GetAllAccountIdentities } = await import('../../wailsjs/go/app/App.js')
      return GetAllAccountIdentities()
    },

    openComposerWindow: async (accountId: string, mode: string, messageId: string, draftId: string, mailtoURL?: string) => {
      const { OpenComposerWindow } = await import('../../wailsjs/go/app/App.js')
      return OpenComposerWindow(accountId, mode, messageId, draftId, mailtoURL || '')
    },
  }
}

/**
 * Creates the composer API implementation for the detached composer window.
 * Uses ComposerApp bindings.
 */
export function createComposerWindowApi(_accountId: string): ComposerApi {
  return {
    sendMessage: async (accountId: string, message: smtp.ComposeMessage) => {
      const { SendMessage } = await import('../../wailsjs/go/app/ComposerApp.js')
      return SendMessage(accountId, message)
    },

    searchContacts: async (query: string, limit: number) => {
      const { SearchContacts } = await import('../../wailsjs/go/app/ComposerApp.js')
      return SearchContacts(query, limit) || []
    },

    getIdentities: async (accountId: string) => {
      const { GetIdentities } = await import('../../wailsjs/go/app/ComposerApp.js')
      return GetIdentities(accountId)
    },

    saveDraft: async (accountId: string, message: smtp.ComposeMessage, draftId: string) => {
      const { SaveDraft } = await import('../../wailsjs/go/app/ComposerApp.js')
      const result = await SaveDraft(accountId, message, draftId || '')
      return { id: result?.id || '', syncStatus: result?.syncStatus || 'pending' }
    },

    deleteDraft: async (draftId: string) => {
      const { DeleteDraft } = await import('../../wailsjs/go/app/ComposerApp.js')
      return DeleteDraft(draftId)
    },

    pickAttachmentFiles: async () => {
      const { PickAttachmentFiles } = await import('../../wailsjs/go/app/ComposerApp.js')
      return PickAttachmentFiles()
    },

    getAccount: async (accountId: string) => {
      const { GetAccount } = await import('../../wailsjs/go/app/ComposerApp.js')
      return GetAccount(accountId)
    },

    readFileAsAttachment: async (filePath: string) => {
      const { ReadFileAsAttachment } = await import('../../wailsjs/go/app/ComposerApp.js')
      return ReadFileAsAttachment(filePath)
    },

    isFlatpak: async () => {
      const { IsFlatpak } = await import('../../wailsjs/go/app/ComposerApp.js')
      return IsFlatpak()
    },

    getAllAccountIdentities: async () => {
      const { GetAllAccountIdentities } = await import('../../wailsjs/go/app/ComposerApp.js')
      return GetAllAccountIdentities()
    },
  }
}
