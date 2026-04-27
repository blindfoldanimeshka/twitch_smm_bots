import { create } from 'zustand'

export interface LogEntry {
  id?: number
  botId?: string
  logType?: string // 'lifecycle', 'message', 'error', 'rate_limit', 'llm'
  level: string // 'info', 'success', 'warning', 'error'
  message: string
  context?: Record<string, any>
  timestamp: Date
}

export interface SettingsLogEntry {
  id?: number
  level: string
  message: string
  changes?: Record<string, any>
  timestamp: Date
}

interface LogStore {
  // Bot logs: Map<botId, LogEntry[]>
  botLogs: Map<string, LogEntry[]>

  // Settings logs
  settingsLogs: SettingsLogEntry[]

  // Actions
  addBotLog: (botId: string, log: LogEntry) => void
  setBotLogs: (botId: string, logs: LogEntry[]) => void
  clearBotLogs: (botId: string) => void

  addSettingsLog: (log: SettingsLogEntry) => void
  setSettingsLogs: (logs: SettingsLogEntry[]) => void
  clearSettingsLogs: () => void
}

export const useLogStore = create<LogStore>((set) => ({
  botLogs: new Map(),
  settingsLogs: [],

  addBotLog: (botId, log) => set((state) => {
    const newBotLogs = new Map(state.botLogs)
    const existingLogs = newBotLogs.get(botId) || []

    // Add new log at the beginning (newest first)
    const updatedLogs = [log, ...existingLogs]

    // Keep only last 100 entries
    if (updatedLogs.length > 100) {
      updatedLogs.splice(100)
    }

    newBotLogs.set(botId, updatedLogs)
    return { botLogs: newBotLogs }
  }),

  setBotLogs: (botId, logs) => set((state) => {
    const newBotLogs = new Map(state.botLogs)
    newBotLogs.set(botId, logs)
    return { botLogs: newBotLogs }
  }),

  clearBotLogs: (botId) => set((state) => {
    const newBotLogs = new Map(state.botLogs)
    newBotLogs.delete(botId)
    return { botLogs: newBotLogs }
  }),

  addSettingsLog: (log) => set((state) => {
    const updatedLogs = [log, ...state.settingsLogs]

    // Keep only last 50 entries
    if (updatedLogs.length > 50) {
      updatedLogs.splice(50)
    }

    return { settingsLogs: updatedLogs }
  }),

  setSettingsLogs: (logs) => set({ settingsLogs: logs }),

  clearSettingsLogs: () => set({ settingsLogs: [] }),
}))
