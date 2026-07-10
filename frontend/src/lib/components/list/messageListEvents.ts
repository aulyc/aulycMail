// @ts-ignore - wailsjs runtime
import { EventsOn } from '../../../../wailsjs/runtime/runtime'

export interface FolderEvent {
  accountId: string
  folderId: string
}

export interface ReadChangedEvent {
  messageIds: string[]
  isRead: boolean
}

export interface FTSProgressEvent {
  folderId: string
  indexed: number
  total: number
  percentage: number
}

export interface FTSCompleteEvent {
  folderId: string
}

export interface FTSIndexingEvent {
  status: string
}

export interface MessageListEventHandlers {
  onFolderSynced: (data: FolderEvent) => void
  onMessagesUpdated: (data: FolderEvent) => void
  onReadChanged: (data: ReadChangedEvent) => void
  onFTSProgress: (data: FTSProgressEvent) => void
  onFTSComplete: (data: FTSCompleteEvent) => void
  onFTSIndexing: (data: FTSIndexingEvent) => void
}

export function registerMessageListEvents(handlers: MessageListEventHandlers): Array<() => void> {
  return [
    EventsOn('folder:synced', handlers.onFolderSynced),
    EventsOn('messages:updated', handlers.onMessagesUpdated),
    EventsOn('messages:readChanged', handlers.onReadChanged),
    EventsOn('fts:progress', handlers.onFTSProgress),
    EventsOn('fts:complete', handlers.onFTSComplete),
    EventsOn('fts:indexing', handlers.onFTSIndexing),
  ]
}
