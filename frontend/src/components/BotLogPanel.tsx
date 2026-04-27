import React, { useEffect, useState } from 'react'
import { api } from '../api/client'
import { useLogStore, type LogEntry } from '../stores/logStore'

interface BotLogPanelProps {
  botId: string
  maxHeight?: number
}

export const BotLogPanel: React.FC<BotLogPanelProps> = ({
  botId,
  maxHeight = 300
}) => {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const logs = useLogStore((state) => state.botLogs.get(botId) || [])
  const setBotLogs = useLogStore((state) => state.setBotLogs)
  const clearBotLogs = useLogStore((state) => state.clearBotLogs)

  useEffect(() => {
    const loadLogs = async () => {
      try {
        setLoading(true)
        setError(null)
        const response = await api.getBotLogs(botId, 100)

        const parsedLogs: LogEntry[] = response.data.map((log) => ({
          id: log.id,
          botId: log.bot_id,
          logType: log.log_type,
          level: log.level,
          message: log.message,
          context: log.context,
          timestamp: new Date(log.created_at),
        }))

        setBotLogs(botId, parsedLogs)
      } catch (err: any) {
        console.error('Failed to load bot logs:', err)
        setError('Logs unavailable')
      } finally {
        setLoading(false)
      }
    }

    loadLogs()
  }, [botId, setBotLogs])

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
    const now = new Date()
    const diffMs = now.getTime() - timestamp.getTime()
    const diffSec = Math.floor(diffMs / 1000)
    const diffMin = Math.floor(diffSec / 60)
    const diffHour = Math.floor(diffMin / 60)
    const diffDay = Math.floor(diffHour / 24)

    if (diffSec < 10) return 'just now'
    if (diffSec < 60) return `${diffSec}s ago`
    if (diffMin < 60) return `${diffMin}m ago`
    if (diffHour < 24) return `${diffHour}h ago`
    return `${diffDay}d ago`
  }

  const handleClear = () => {
    clearBotLogs(botId)
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
      }}>
        {error}
      </div>
    )
  }

  return (
    <div style={{
      background: 'var(--bg-base)',
      border: '1px solid var(--border-default)',
      borderRadius: 'var(--radius-sm)',
      overflow: 'hidden',
    }}>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        padding: 'var(--space-3) var(--space-4)',
        background: 'var(--bg-surface)',
        borderBottom: '1px solid var(--border-default)',
      }}>
        <div style={{
          fontFamily: 'var(--font-display)',
          textTransform: 'uppercase',
          letterSpacing: '0.08em',
          fontSize: '0.75rem',
          fontWeight: 700,
          color: 'var(--accent-lime)',
        }}>
          ACTIVITY LOG ({logs.length})
        </div>
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
          NO ACTIVITY YET
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
                display: 'flex',
                gap: 'var(--space-3)',
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
          ))}
        </div>
      )}
    </div>
  )
}
