import React, { useEffect, useState, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
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
            messages.push({
              id: `stt-${log.id}`,
              type: 'stt',
              message: log.context.stt_context,
              timestamp: new Date(log.created_at),
            })
          }

          if (log.log_type === 'message') {
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

  if (!isOpen) return null

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        onClick={onClose}
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: 'rgba(0, 0, 0, 0.8)',
          backdropFilter: 'blur(8px)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 9999,
          padding: 'var(--space-4)',
        }}
      >
        <motion.div
          initial={{ scale: 0.9, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          exit={{ scale: 0.9, opacity: 0 }}
          onClick={(e) => e.stopPropagation()}
          style={{
            width: '100%',
            maxWidth: '700px',
            maxHeight: '85vh',
            display: 'flex',
            flexDirection: 'column',
            padding: '6px',
            borderRadius: '16px',
            background: 'rgba(255, 255, 255, 0.05)',
            backdropFilter: 'blur(40px) saturate(180%)',
            border: '1px solid rgba(255, 255, 255, 0.1)',
            boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
          }}
        >
          {/* Header */}
          <div style={{
            padding: 'var(--space-4)',
            borderBottom: '1px solid rgba(255, 255, 255, 0.08)',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            background: 'linear-gradient(to bottom right, rgba(255, 255, 255, 0.08), transparent)',
            borderRadius: '12px 12px 0 0',
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
                color: 'rgba(255, 255, 255, 0.6)',
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
                border: '1px solid rgba(255, 255, 255, 0.1)',
                borderRadius: 'var(--radius-sm)',
                padding: 'var(--space-2) var(--space-3)',
                color: 'rgba(255, 255, 255, 0.6)',
                cursor: 'pointer',
                fontFamily: 'var(--font-display)',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                fontSize: '0.75rem',
                fontWeight: 700,
                transition: 'all 0.2s ease',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = 'var(--state-error)'
                e.currentTarget.style.color = 'var(--state-error)'
                e.currentTarget.style.background = 'rgba(255, 59, 48, 0.1)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = 'rgba(255, 255, 255, 0.1)'
                e.currentTarget.style.color = 'rgba(255, 255, 255, 0.6)'
                e.currentTarget.style.background = 'transparent'
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
            background: 'rgba(0, 0, 0, 0.2)',
          }}>
            {loading ? (
              <div style={{
                textAlign: 'center',
                padding: 'var(--space-8)',
                color: 'rgba(255, 255, 255, 0.5)',
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
                color: 'rgba(255, 255, 255, 0.5)',
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
                      padding: '6px',
                      borderRadius: '12px',
                      background: 'rgba(255, 255, 255, 0.05)',
                      backdropFilter: 'blur(20px) saturate(150%)',
                      border: '1px solid rgba(255, 255, 255, 0.08)',
                      boxShadow: '0 4px 16px rgba(0, 0, 0, 0.2)',
                      transition: 'all 0.2s ease',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.background = 'rgba(255, 255, 255, 0.08)'
                      e.currentTarget.style.transform = 'translateY(-2px)'
                      e.currentTarget.style.boxShadow = '0 6px 20px rgba(0, 0, 0, 0.3)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.background = 'rgba(255, 255, 255, 0.05)'
                      e.currentTarget.style.transform = 'translateY(0)'
                      e.currentTarget.style.boxShadow = '0 4px 16px rgba(0, 0, 0, 0.2)'
                    }}
                  >
                    <div style={{
                      padding: 'var(--space-3)',
                      borderRadius: '8px',
                      background: 'linear-gradient(to bottom right, rgba(255, 255, 255, 0.03), transparent)',
                    }}>
                      <div style={{ display: 'flex', gap: 'var(--space-3)', alignItems: 'flex-start' }}>
                        {/* Avatar */}
                        <div style={{
                          width: '40px',
                          height: '40px',
                          borderRadius: '50%',
                          background: msg.type === 'stt'
                            ? 'linear-gradient(135deg, rgba(57,255,106,0.3), rgba(57,255,106,0.1))'
                            : 'linear-gradient(135deg, rgba(138,180,248,0.3), rgba(138,180,248,0.1))',
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
                              color: 'rgba(255, 255, 255, 0.5)',
                            }}>
                              {formatTimestamp(msg.timestamp)}
                            </span>
                            {msg.confidence && (
                              <span style={{
                                fontFamily: 'var(--font-mono)',
                                fontSize: '0.7rem',
                                color: 'var(--accent-lime)',
                                background: 'rgba(57,255,106,0.15)',
                                padding: '2px 6px',
                                borderRadius: 'var(--radius-sm)',
                                border: '1px solid rgba(57,255,106,0.3)',
                              }}>
                                {Math.round(msg.confidence * 100)}%
                              </span>
                            )}
                          </div>
                          <div style={{
                            fontFamily: 'var(--font-mono)',
                            fontSize: '0.875rem',
                            color: 'rgba(255, 255, 255, 0.9)',
                            lineHeight: 1.5,
                            wordBreak: 'break-word',
                          }}>
                            {msg.message}
                          </div>
                        </div>
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
    </AnimatePresence>
  )
}
