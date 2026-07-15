type ActivityLogType = 'sync' | 'backup' | string
type ActivityLogStatus = 'success' | 'partial' | 'failed' | 'cancelled' | string

export interface ActivityLogPayload {
  accountEmail?: string
  folderName?: string
  scope?: string
  mode?: 'full' | 'incremental'
  total?: number
  completed?: number
  success?: number
  added?: number
  skipped?: number
  missing?: number
  unavailable?: number
  failed?: number
  directory?: string
}

export interface ActivityLog {
  id: string
  createdAt: string
  type: ActivityLogType
  status: ActivityLogStatus
  title: string
  summary: string
  detail?: string
  payload?: ActivityLogPayload
  payloadJson?: string
}

export interface ActivityLogQuery {
  type: string
  problemOnly: boolean
  date: string
  timezoneOffsetMinutes: number
  directory: string
  limit: number
  offset: number
}
