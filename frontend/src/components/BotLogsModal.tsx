import React, { useEffect, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useLogStore, type LogEntry } from '../stores/logStore'
import { api } from '../api/client'

interface BotLogsModalProps {
  botId: string
  botName: string
  isOpen: boolean
  onClose: () => void
}

export const BotLogsModal: React.FC<BotLogsModalProps> = ({
  botId,
  botName,
  isOpen,
  onClose,
}) => {
  const [loading, setLoading] = useState(true)
  const logs = useLogStore((state) => state.botLogs.get(botId) || [])
  const setBotLogs = useLogStore((state) => state.setBotLogs)

  useEffect(() => {
    const loadLogs = async () => {
      if (!isOpen) return

      try {
        setLoading(true)
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
      } catch (err) {
        console.error('Failed to load bot logs:', err)
      } finally {
        setLoading(false)
      }
    }

    loadLogs()
  }, [isOpen, botId, setBotLogs])

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
    return timestamp.toLocaleString('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  }

  const renderLogContent = (log: LogEntry) => {
    // Если это сообщение с контекстом STT
    if (log.logType === 'message' && log.context) {
      const sttContext = log.context.stt_context || log.context.audio_context

      return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
          {sttContext && (
            <div style={{
              padding: 'var(--space-2)',
              background: 'rgba(57,255,106,0.05)',
              border: '1px solid rgba(57,255,106,0.2)',
              borderRadius: 'var(--radius-sm)',
            }}>
              <div style={{
                fontSize: '0.625rem',
                color: 'var(--accent-lime)',
                fontFamily: 'var(--font-display)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                marginBottom: 'var(--space-1)',
              }}>
                STT ТРАНСКРИПЦИЯ:
              </div>
              <div style={{
                fontSize: '0.75rem',
                color: 'var(--text-secondary)',
                fontFamily: 'var(--font-mono)',
                lineHeight: 1.5,
              }}>
                {sttContext}
              </div>
            </div>
          )}
          <div style={{
            padding: 'var(--space-2)',
            background: 'rgba(138,180,248,0.05)',
            border: '1px solid rgba(138,180,248,0.2)',
            borderRadius: 'var(--radius-sm)',
          }}>
            <div style={{
              fontSize: '0.625rem',
              color: 'var(--accent-blue)',
              fontFamily: 'var(--font-display)',
              textTransform: 'uppercase',
              letterSpacing: '0.08em',
              marginBottom: 'var(--space-1)',
            }}>
              ОТВЕТ БОТА:
            </div>
            <div style={{
              fontSize: '0.875rem',
              color: 'var(--text-primary)',
              fontFamily: 'var(--font-mono)',
              lineHeight: 1.5,
            }}>
              {log.message}
            </div>
          </div>
        </div>
      )
    }

    // Обычное сообщение
    return (
      <div style={{
        fontFamily: 'var(--font-mono)',
        fontSize: '0.875rem',
        color: 'var(--text-primary)',
        wordBreak: 'break-word',
        lineHeight: 1.5,
      }}>
        {log.message}
      </div>
    )
  }

  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: 'rgba(0, 0, 0, 0.8)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 9999,
          padding: 'var(--space-4)',
        }}
        onClick={onClose}
      >
        <motion.div
          initial={{ scale: 0.9, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          exit={{ scale: 0.9, opacity: 0 }}
          onClick={(e) => e.stopPropagation()}
          style={{
            background: 'var(--bg-base)',
            border: '1px solid var(--border-default)',
            borderRadius: 'var(--radius-md)',
            width: '100%',
            maxWidth: '900px',
            maxHeight: '80vh',
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
          }}
        >
          {/* Header */}
          <div style={{
            padding: 'var(--space-4)',
            borderBottom: '1px solid var(--border-default)',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            background: 'var(--bg-surface)',
          }}>
            <div>
              <h2 style={{
                fontFamily: 'var(--font-display)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                fontSize: '1.25rem',
                fontWeight: 700,
                color: 'var(--accent-lime)',
                margin: 0,
              }}>
                ЛОГИ БОТА
              </h2>
              <div style={{
                fontSize: '0.875rem',
                color: 'var(--text-muted)',
                fontFamily: 'var(--font-mono)',
                marginTop: 'var(--space-1)',
              }}>
                {botName} ({logs.length} записей)
              </div>
            </div>
            <button
              onClick={onClose}
              style={{
                background: 'transparent',
                border: '1px solid var(--border-default)',
                borderRadius: 'var(--radius-sm)',
                padding: 'var(--space-2) var(--space-3)',
                color: 'var(--text-muted)',
                cursor: 'pointer',
                fontFamily: 'var(--font-display)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                fontSize: '0.75rem',
                fontWeight: 700,
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
              ✕ ЗАКРЫТЬ
            </button>
          </div>

          {/* Content */}
          <div style={{
            flex: 1,
            overflowY: 'auto',
            padding: 'var(--space-4)',
          }}>
            {loading ? (
              <div style={{
                textAlign: 'center',
                padding: 'var(--space-8)',
                color: 'var(--text-muted)',
                fontFamily: 'var(--font-display)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
              }}>
                ЗАГРУЗКА...
              </div>
            ) : logs.length === 0 ? (
              <div style={{
                textAlign: 'center',
                padding: 'var(--space-8)',
                color: 'var(--text-muted)',
                fontFamily: 'var(--font-display)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
              }}>
                НЕТ ЛОГОВ
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
                {logs.map((log, index) => (
                  <div
                    key={log.id || index}
                    style={{
                      background: index % 2 === 0 ? 'var(--bg-base)' : 'rgba(6,16,56,0.5)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: 'var(--radius-sm)',
                      padding: 'var(--space-3)',
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
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      marginBottom: 'var(--space-2)',
                    }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                        <span style={{
                          color: getLevelColor(log.level),
                          fontWeight: 700,
                          fontSize: '0.875rem',
                        }}>
                          {getLevelIcon(log.level)}
                        </span>
                        <span style={{
                          fontFamily: 'var(--font-display)',
                          textTransform: 'uppercase',
                          letterSpacing: '0.08em',
                          fontSize: '0.625rem',
                          color: 'var(--text-muted)',
                        }}>
                          {log.logType || 'message'}
                        </span>
                      </div>
                      <span style={{
                        fontFamily: 'var(--font-mono)',
                        fontSize: '0.75rem',
                        color: 'var(--text-muted)',
                      }}>
                        {formatTimestamp(log.timestamp)}
                      </span>
                    </div>
                    {renderLogContent(log)}
                  </div>
                ))}
              </div>
            )}
          </div>
        </motion.div>
      </motion.div>
      )}
    </AnimatePresence>
  )
}
