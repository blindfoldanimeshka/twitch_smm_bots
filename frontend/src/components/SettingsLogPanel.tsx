import React, { useEffect, useState } from 'react'
import { api } from '../api/client'
import { useLogStore, type SettingsLogEntry } from '../stores/logStore'

interface SettingsLogPanelProps {
  maxHeight?: number
  collapsed?: boolean
}

export const SettingsLogPanel: React.FC<SettingsLogPanelProps> = ({
  maxHeight = 250,
  collapsed = true
}) => {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isExpanded, setIsExpanded] = useState(!collapsed)

  const logs = useLogStore((state) => state.settingsLogs)
  const setSettingsLogs = useLogStore((state) => state.setSettingsLogs)
  const clearSettingsLogs = useLogStore((state) => state.clearSettingsLogs)

  useEffect(() => {
    const loadLogs = async () => {
      try {
        setLoading(true)
        setError(null)
        const response = await api.getSettingsLogs(50)

        const parsedLogs: SettingsLogEntry[] = response.data.map((log) => ({
          id: log.id,
          level: log.level,
          message: log.message,
          changes: log.changes,
          timestamp: new Date(log.created_at),
        }))

        setSettingsLogs(parsedLogs)
      } catch (err: any) {
        console.error('Failed to load settings logs:', err)
        setError('Logs unavailable')
      } finally {
        setLoading(false)
      }
    }

    loadLogs()
  }, [setSettingsLogs])

  const getLevelColor = (level: string): string => {
    switch (level) {
      case 'success':
        return 'var(--state-success)'
      case 'error':
        return 'var(--state-error)'
      case 'warning':
        return 'var(--state-warning)'
      case 'info':
        return 'var(--state-info)'
      default:
        return 'var(--text-secondary)'
    }
  }

  const getLevelIcon = (level: string): string => {
    switch (level) {
      case 'success':
        return '✓'
      case 'error':
        return '✗'
      case 'warning':
        return '⚠'
      case 'info':
        return 'ℹ'
      default:
        return '•'
    }
  }

  const formatTimestamp = (timestamp: Date): string => {
    const day = String(timestamp.getDate()).padStart(2, '0')
    const month = String(timestamp.getMonth() + 1).padStart(2, '0')
    const hours = String(timestamp.getHours()).padStart(2, '0')
    const minutes = String(timestamp.getMinutes()).padStart(2, '0')
    const seconds = String(timestamp.getSeconds()).padStart(2, '0')
    return `${day}.${month} ${hours}:${minutes}:${seconds}`
  }

  const getLastSaveTimestamp = (): string | null => {
    if (logs.length === 0) return null
    return formatTimestamp(logs[0].timestamp)
  }

  const handleClear = () => {
    clearSettingsLogs()
  }

  const toggleExpand = () => {
    setIsExpanded(!isExpanded)
  }

  const renderChangeDetails = (changes?: Record<string, any>) => {
    if (!changes || Object.keys(changes).length === 0) {
      return null
    }

    return (
      <div style={{
        marginTop: 'var(--space-2)',
        paddingLeft: 'var(--space-6)',
        fontFamily: 'var(--font-mono)',
        fontSize: '0.8125rem',
        color: 'var(--text-secondary)',
      }}>
        {Object.entries(changes).map(([field, change]: [string, any]) => {
          const oldValue = change.old !== undefined ? JSON.stringify(change.old) : 'null'
          const newValue = change.new !== undefined ? JSON.stringify(change.new) : 'null'

          return (
            <div key={field} style={{ marginBottom: 'var(--space-1)' }}>
              <span style={{ color: 'var(--accent-lime)', fontWeight: 600 }}>{field}:</span>{' '}
              <span style={{ color: 'var(--text-muted)' }}>{oldValue}</span>
              {' → '}
              <span style={{ color: 'var(--state-success)' }}>{newValue}</span>
            </div>
          )
        })}
      </div>
    )
  }

  if (loading) {
    return (
      <div style={{
        padding: 'var(--space-4)',
        textAlign: 'center',
        color: 'var(--text-muted)',
        fontFamily: 'var(--font-display)',
        textTransform: 'uppercase',
        letterSpacing: '0.08em',
        fontSize: '0.875rem',
        border: '2px solid var(--accent-lime)',
        borderRadius: 'var(--radius-sm)',
        background: 'var(--bg-base)',
      }}>
        LOADING LOGS...
      </div>
    )
  }

  if (error) {
    return (
      <div style={{
        padding: 'var(--space-4)',
        textAlign: 'center',
        color: 'var(--state-error)',
        fontFamily: 'var(--font-display)',
        textTransform: 'uppercase',
        letterSpacing: '0.08em',
        fontSize: '0.875rem',
        border: '2px solid var(--state-error)',
        borderRadius: 'var(--radius-sm)',
        background: 'var(--bg-base)',
      }}>
        {error}
      </div>
    )
  }

  const lastSave = getLastSaveTimestamp()

  return (
    <div style={{
      background: 'var(--bg-base)',
      border: '2px solid var(--accent-lime)',
      borderRadius: 'var(--radius-sm)',
      overflow: 'hidden',
    }}>
      <div
        onClick={toggleExpand}
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          padding: 'var(--space-3) var(--space-4)',
          background: 'var(--bg-surface)',
          borderBottom: isExpanded ? '2px solid var(--accent-lime)' : 'none',
          cursor: 'pointer',
          transition: 'all var(--duration-base) var(--easing)',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.background = 'rgba(57,255,106,0.1)'
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.background = 'var(--bg-surface)'
        }}
      >
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--space-3)',
        }}>
          <span style={{
            fontFamily: 'var(--font-display)',
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
            fontSize: '0.75rem',
            fontWeight: 700,
            color: 'var(--accent-lime)',
          }}>
            SETTINGS CHANGE LOG ({logs.length})
          </span>
          {lastSave && (
            <span style={{
              fontFamily: 'var(--font-mono)',
              fontSize: '0.6875rem',
              color: 'var(--text-muted)',
            }}>
              Last save: {lastSave}
            </span>
          )}
        </div>
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--space-2)',
        }}>
          <span style={{
            color: 'var(--accent-lime)',
            fontSize: '0.875rem',
            fontWeight: 700,
            transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
            transition: 'transform var(--duration-base) var(--easing)',
            display: 'inline-block',
          }}>
            ▼
          </span>
        </div>
      </div>

      {isExpanded && (
        <>
          <div style={{
            display: 'flex',
            justifyContent: 'flex-end',
            padding: 'var(--space-2) var(--space-4)',
            background: 'var(--bg-surface)',
            borderBottom: '1px solid var(--border-subtle)',
          }}>
            <button
              onClick={handleClear}
              style={{
                padding: 'var(--space-1) var(--space-3)',
                background: 'transparent',
                border: '1px solid var(--border-default)',
                borderRadius: 'var(--radius-sm)',
                color: 'var(--text-muted)',
                fontFamily: 'var(--font-display)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                fontSize: '0.625rem',
                fontWeight: 700,
                cursor: 'pointer',
                transition: 'all var(--duration-base) var(--easing)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = 'var(--state-error)'
                e.currentTarget.style.color = 'var(--state-error)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = 'var(--border-default)'
                e.currentTarget.style.color = 'var(--text-muted)'
              }}
            >
              CLEAR
            </button>
          </div>

          {logs.length === 0 ? (
            <div style={{
              padding: 'var(--space-8)',
              textAlign: 'center',
              color: 'var(--text-muted)',
              fontFamily: 'var(--font-display)',
              textTransform: 'uppercase',
              letterSpacing: '0.08em',
              fontSize: '0.875rem',
            }}>
              NO CHANGES YET
            </div>
          ) : (
            <div style={{
              maxHeight: `${maxHeight}px`,
              overflowY: 'auto',
            }}>
              {logs.map((log, index) => (
                <div
                  key={log.id || index}
                  style={{
                    padding: 'var(--space-3) var(--space-4)',
                    background: index % 2 === 0 ? 'var(--bg-base)' : 'rgba(6,16,56,0.5)',
                    borderBottom: index < logs.length - 1 ? '1px solid var(--border-subtle)' : 'none',
                    transition: 'background var(--duration-fast) var(--easing)',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.background = 'rgba(57,255,106,0.05)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.background = index % 2 === 0 ? 'var(--bg-base)' : 'rgba(6,16,56,0.5)'
                  }}
                >
                  <div style={{
                    display: 'flex',
                    gap: 'var(--space-3)',
                    alignItems: 'flex-start',
                  }}>
                    <div style={{
                      color: getLevelColor(log.level),
                      fontWeight: 700,
                      fontSize: '0.875rem',
                      minWidth: '16px',
                      textAlign: 'center',
                    }}>
                      {getLevelIcon(log.level)}
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{
                        fontFamily: 'var(--font-mono)',
                        fontSize: '0.875rem',
                        color: 'var(--text-primary)',
                        wordBreak: 'break-word',
                        lineHeight: 1.5,
                      }}>
                        {log.message}
                      </div>
                      {renderChangeDetails(log.changes)}
                    </div>
                    <div style={{
                      fontFamily: 'var(--font-mono)',
                      fontSize: '0.75rem',
                      color: 'var(--text-muted)',
                      whiteSpace: 'nowrap',
                    }}>
                      {formatTimestamp(log.timestamp)}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}
