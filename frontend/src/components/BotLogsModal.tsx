import React, { useEffect, useState, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useLogStore, type LogEntry } from '../stores/logStore'
import { useWebSocket, type WSMessage } from '../hooks/useWebSocket'
import { api } from '../api/client'

interface BotLogsModalProps {
  botId: string
  botName: string
  isOpen: boolean
  onClose: () => void
}

interface ChatMessage {
  id: string
  type: 'stt' | 'llm'
  message: string
  timestamp: Date
  confidence?: number
}

export const BotLogsModal: React.FC<BotLogsModalProps> = ({
  botId,
  botName,
  isOpen,
  onClose,
}) => {
  const [loading, setLoading] = useState(true)
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const botLogsMap = useLogStore((state) => state.botLogs)
  const setBotLogs = useLogStore((state) => state.setBotLogs)

  useEffect(() => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [chatMessages])

  const handleWSMessage = (msg: WSMessage) => {
    if (!isOpen) return

    if (msg.type === 'bot_message_sent' && msg.payload?.bot_id === botId) {
      const newMessage: ChatMessage = {
        id: Date.now().toString() + Math.random(),
        type: 'llm',
        message: msg.payload.message || msg.payload.text || '',
        timestamp: new Date(msg.payload.timestamp || Date.now()),
      }
      setChatMessages(prev => [...prev, newMessage])
    }

    if (msg.type === 'stt_transcript' && msg.payload?.bot_id === botId) {
      const newMessage: ChatMessage = {
        id: Date.now().toString() + Math.random(),
        type: 'stt',
        message: msg.payload.text || msg.payload.transcript || '',
        timestamp: new Date(msg.payload.timestamp || Date.now()),
        confidence: msg.payload.confidence,
      }
      setChatMessages(prev => [...prev, newMessage])
    }
  }

  useWebSocket(handleWSMessage)

  useEffect(() => {
    const loadHistory = async () => {
      if (!isOpen) return

      try {
        setLoading(true)
        const response = await api.getBotLogs(botId, 100)

        const messages: ChatMessage[] = []
        response.data.forEach((log) => {
          if (log.log_type === 'message' && log.context?.stt_context) {
            // STT message
            messages.push({
              id: `stt-${log.id}`,
              type: 'stt',
              message: log.context.stt_context,
              timestamp: new Date(log.created_at),
            })
          }

          if (log.log_type === 'message') {
            // LLM response
            messages.push({
              id: `llm-${log.id}`,
              type: 'llm',
              message: log.message,
              timestamp: new Date(log.created_at),
            })
          }
        })

        setChatMessages(messages)
      } catch (err) {
        console.error('Failed to load chat history:', err)
      } finally {
        setLoading(false)
      }
    }

    loadHistory()
  }, [isOpen, botId])

  const formatTimestamp = (timestamp: Date): string => {
    return timestamp.toLocaleTimeString('ru-RU', {
      hour: '2-digit',
      minute: '2-digit',
    })
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
                💬 STT ↔ LLM ЧАТ
              </h2>
              <div style={{
                fontSize: '0.875rem',
                color: 'var(--text-muted)',
                fontFamily: 'var(--font-mono)',
                marginTop: 'var(--space-1)',
              }}>
                {botName} • {chatMessages.length} сообщений
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
            background: 'var(--bg-base)',
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
            ) : chatMessages.length === 0 ? (
              <div style={{
                textAlign: 'center',
                padding: 'var(--space-8)',
                color: 'var(--text-muted)',
                fontFamily: 'var(--font-display)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
              }}>
                НЕТ СООБЩЕНИЙ
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
                {chatMessages.map((msg) => (
                  <motion.div
                    key={msg.id}
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.2 }}
                    style={{
                      display: 'flex',
                      gap: 'var(--space-3)',
                      alignItems: 'flex-start',
                    }}
                  >
                    {/* Avatar */}
                    <div style={{
                      width: '40px',
                      height: '40px',
                      borderRadius: '50%',
                      background: msg.type === 'stt'
                        ? 'linear-gradient(135deg, rgba(57,255,106,0.2), rgba(57,255,106,0.05))'
                        : 'linear-gradient(135deg, rgba(138,180,248,0.2), rgba(138,180,248,0.05))',
                      border: msg.type === 'stt'
                        ? '2px solid var(--accent-lime)'
                        : '2px solid var(--accent-blue)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      fontSize: '1.25rem',
                      flexShrink: 0,
                    }}>
                      {msg.type === 'stt' ? '🎤' : '🤖'}
                    </div>

                    {/* Message */}
                    <div style={{ flex: 1 }}>
                      <div style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 'var(--space-2)',
                        marginBottom: 'var(--space-1)',
                      }}>
                        <span style={{
                          fontFamily: 'var(--font-display)',
                          textTransform: 'uppercase',
                          letterSpacing: '0.08em',
                          fontSize: '0.75rem',
                          fontWeight: 700,
                          color: msg.type === 'stt' ? 'var(--accent-lime)' : 'var(--accent-blue)',
                        }}>
                          {msg.type === 'stt' ? 'STT' : 'LLM'}
                        </span>
                        <span style={{
                          fontFamily: 'var(--font-mono)',
                          fontSize: '0.7rem',
                          color: 'var(--text-muted)',
                        }}>
                          {formatTimestamp(msg.timestamp)}
                        </span>
                        {msg.confidence && (
                          <span style={{
                            fontFamily: 'var(--font-mono)',
                            fontSize: '0.7rem',
                            color: 'var(--accent-lime)',
                            background: 'rgba(57,255,106,0.1)',
                            padding: '2px 6px',
                            borderRadius: 'var(--radius-sm)',
                          }}>
                            {Math.round(msg.confidence * 100)}%
                          </span>
                        )}
                      </div>
                      <div style={{
                        background: msg.type === 'stt'
                          ? 'rgba(57,255,106,0.05)'
                          : 'rgba(138,180,248,0.05)',
                        border: msg.type === 'stt'
                          ? '1px solid rgba(57,255,106,0.2)'
                          : '1px solid rgba(138,180,248,0.2)',
                        borderRadius: 'var(--radius-sm)',
                        padding: 'var(--space-3)',
                        fontFamily: 'var(--font-mono)',
                        fontSize: '0.875rem',
                        color: 'var(--text-primary)',
                        lineHeight: 1.5,
                        wordBreak: 'break-word',
                      }}>
                        {msg.message}
                      </div>
                    </div>
                  </motion.div>
                ))}
                <div ref={messagesEndRef} />
              </div>
            )}
          </div>
        </motion.div>
      </motion.div>
      )}
    </AnimatePresence>
  )
}
