/**
 * Composer API Abstraction Layer
 *
 * Provides a unified interface for the in-app (modal/inline) composer's
 * operations, backed by the main window's App bindings. The API is injected
 * via Svelte context.
 */

// @ts-ignore - Wails generated imports
import { smtp, account, contact, app } from '../../wailsjs/go/models'
import {
  DeleteDraft,
  GetAccount,
  GetAllAccountIdentities,
  GetIdentities,
  IsFlatpak,
  PickAttachmentFiles,
  ReadFileAsAttachment,
  SaveDraft,
  SearchContacts,
  SendMessage
} from '../../wailsjs/go/app/App.js'

/**
 * Interface for composer API operations.
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

  /** Get all accounts with their identities (for the cross-account From dropdown) */
  getAllAccountIdentities?: () => Promise<app.AccountIdentityGroup[]>
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
  return {
    sendMessage: async (accountId: string, message: smtp.ComposeMessage) => {
      return SendMessage(accountId, message)
    },

    searchContacts: async (query: string, limit: number) => {
      return SearchContacts(query, limit) || []
    },

    getIdentities: async (accountId: string) => {
      return GetIdentities(accountId)
    },

    saveDraft: async (accountId: string, message: smtp.ComposeMessage, draftId: string) => {
      const result = await SaveDraft(accountId, message, draftId)
      return { id: result?.draft?.id || '', syncStatus: result?.draft?.syncStatus || 'pending' }
    },

    deleteDraft: async (draftId: string) => {
      return DeleteDraft(draftId)
    },

    pickAttachmentFiles: async () => {
      return PickAttachmentFiles()
    },

    getAccount: async (accountId: string) => {
      return GetAccount(accountId)
    },

    readFileAsAttachment: async (filePath: string) => {
      return ReadFileAsAttachment(filePath)
    },

    isFlatpak: async () => {
      return IsFlatpak()
    },

    getAllAccountIdentities: async () => {
      return GetAllAccountIdentities()
    },
  }
}
