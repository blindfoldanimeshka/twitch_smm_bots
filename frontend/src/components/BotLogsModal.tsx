import React, { useEffect, useState, useRef, useCallback } from 'react'
import { createPortal } from 'react-dom'
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

  const handleWSMessage = useCallback((msg: WSMessage) => {
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
  }, [isOpen, botId])

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

  const sttCount = chatMessages.filter(m => m.type === 'stt').length
  const llmCount = chatMessages.filter(m => m.type === 'llm').length

  if (!isOpen) return null

  return createPortal(
    <AnimatePresence mode="wait">
      {isOpen && (
        <>
          {/* Backdrop with blur */}
          <motion.div
            onClick={onClose}
            className="fixed inset-0 z-[10000] bg-black/70 backdrop-blur-sm cursor-pointer"
            initial={{ opacity: 0, backdropFilter: 'blur(0px)' }}
            animate={{
              opacity: 1,
              backdropFilter: 'blur(8px)',
              transition: { duration: 0.3, ease: [0.16, 1, 0.3, 1] }
            }}
            exit={{
              opacity: 0,
              backdropFilter: 'blur(0px)',
              transition: { duration: 0.2, ease: [0.16, 1, 0.3, 1] }
            }}
          />

          {/* Modal Container */}
          <div className="fixed inset-0 z-[10001] flex items-center justify-center pointer-events-none">
            <motion.div
              className="relative flex gap-4 w-[90vw] h-[80vh] max-w-[1400px] pointer-events-auto cursor-default"
              onClick={(e) => e.stopPropagation()}
              initial={{
                opacity: 0,
                scale: 0.92,
                y: 40,
                rotateX: 8
              }}
              animate={{
                opacity: 1,
                scale: 1,
                y: 0,
                rotateX: 0,
                transition: {
                  duration: 0.4,
                  ease: [0.16, 1, 0.3, 1],
                  opacity: { duration: 0.3 },
                  scale: { duration: 0.4 },
                  y: { duration: 0.4 },
                  rotateX: { duration: 0.4 }
                }
              }}
              exit={{
                opacity: 0,
                scale: 0.95,
                y: 20,
                transition: {
                  duration: 0.25,
                  ease: [0.16, 1, 0.3, 1]
                }
              }}
              style={{
                willChange: 'transform, opacity',
                perspective: '1200px',
                transformStyle: 'preserve-3d'
              }}
            >
              {/* LEFT BLOCK - Messages */}
              <motion.div
                className="flex-1 rounded-2xl overflow-hidden cursor-default"
                style={{
                  background: 'rgba(15, 15, 20, 0.95)',
                  backdropFilter: 'blur(10px)',
                  border: '1px solid rgba(139, 92, 246, 0.3)',
                  boxShadow: '0 8px 32px rgba(139, 92, 246, 0.2), 0 0 0 1px rgba(139, 92, 246, 0.1) inset',
                }}
                initial={{ opacity: 0, x: -20 }}
                animate={{
                  opacity: 1,
                  x: 0,
                  transition: { delay: 0.1, duration: 0.4, ease: [0.16, 1, 0.3, 1] }
                }}
                exit={{ opacity: 0, x: -10 }}
              >
                <div className="p-6 border-b border-purple-500/20">
                  <h2 className="text-2xl font-bold text-white">💬 STT ↔ LLM ЧАТ</h2>
                  <p className="text-purple-300 text-sm mt-1">{botName}</p>
                </div>

                <div className="h-[calc(100%-88px)] overflow-y-auto p-6 space-y-4">
                  {loading ? (
                    <div className="flex items-center justify-center h-full">
                      <div className="text-purple-300">Загрузка...</div>
                    </div>
                  ) : chatMessages.length === 0 ? (
                    <div className="flex items-center justify-center h-full">
                      <div className="text-purple-300/50 text-center">
                        НЕТ СООБЩЕНИЙ
                      </div>
                    </div>
                  ) : (
                    chatMessages.map((msg, index) => (
                      <motion.div
                        key={msg.id}
                        className="rounded-xl p-4 cursor-default hover:scale-[1.01] transition-transform"
                        style={{
                          background: msg.type === 'stt'
                            ? 'rgba(139, 92, 246, 0.1)'
                            : 'rgba(59, 130, 246, 0.1)',
                          border: `1px solid ${msg.type === 'stt' ? 'rgba(139, 92, 246, 0.3)' : 'rgba(59, 130, 246, 0.3)'}`,
                        }}
                        initial={{ opacity: 0, y: 10 }}
                        animate={{
                          opacity: 1,
                          y: 0,
                          transition: {
                            delay: Math.min(index * 0.03, 0.5),
                            duration: 0.3,
                            ease: [0.16, 1, 0.3, 1]
                          }
                        }}
                        whileHover={{
                          boxShadow: msg.type === 'stt'
                            ? '0 4px 20px rgba(139, 92, 246, 0.2)'
                            : '0 4px 20px rgba(59, 130, 246, 0.2)',
                          transition: { duration: 0.2 }
                        }}
                      >
                        <div className="flex items-center gap-2 mb-2">
                          <span className="text-2xl">{msg.type === 'stt' ? '🎤' : '🤖'}</span>
                          <span className="text-white font-semibold">
                            {msg.type === 'stt' ? 'STT' : 'LLM'}
                          </span>
                          <span className="text-purple-300/50 text-xs ml-auto">
                            {formatTimestamp(msg.timestamp)}
                          </span>
                        </div>
                        <p className="text-white/90 text-sm leading-relaxed">{msg.message}</p>
                        {msg.confidence && (
                          <div className="mt-2 text-xs text-purple-300/70">
                            Уверенность: {(msg.confidence * 100).toFixed(0)}%
                          </div>
                        )}
                      </motion.div>
                    ))
                  )}
                  <div ref={messagesEndRef} />
                </div>
              </motion.div>

              {/* RIGHT COLUMN */}
              <div className="flex flex-col gap-4 w-[350px]">
                {/* TOP RIGHT - Bot Info */}
                <motion.div
                  className="rounded-2xl p-6 flex-1 cursor-default"
                  style={{
                    background: 'rgba(15, 15, 20, 0.95)',
                    backdropFilter: 'blur(10px)',
                    border: '1px solid rgba(139, 92, 246, 0.3)',
                    boxShadow: '0 8px 32px rgba(139, 92, 246, 0.2), 0 0 0 1px rgba(139, 92, 246, 0.1) inset',
                  }}
                  initial={{ opacity: 0, x: 20, y: -10 }}
                  animate={{
                    opacity: 1,
                    x: 0,
                    y: 0,
                    transition: { delay: 0.15, duration: 0.4, ease: [0.16, 1, 0.3, 1] }
                  }}
                  exit={{ opacity: 0, x: 10 }}
                >
                  <h3 className="text-lg font-bold text-white mb-4">ИНФОРМАЦИЯ О БОТЕ</h3>
                  <div className="space-y-3">
                    <div>
                      <div className="text-purple-300/70 text-xs mb-1">ИМЯ</div>
                      <div className="text-white font-mono">{botName}</div>
                    </div>
                    <div>
                      <div className="text-purple-300/70 text-xs mb-1">ID</div>
                      <div className="text-white font-mono text-sm">{botId}</div>
                    </div>
                    <div>
                      <div className="text-purple-300/70 text-xs mb-1">КАНАЛ</div>
                      <div className="text-white font-mono">@{botName.toLowerCase()}</div>
                    </div>
                  </div>
                </motion.div>

                {/* BOTTOM RIGHT - Stats */}
                <motion.div
                  className="rounded-2xl p-6 flex-1 cursor-default"
                  style={{
                    background: 'rgba(15, 15, 20, 0.95)',
                    backdropFilter: 'blur(10px)',
                    border: '1px solid rgba(139, 92, 246, 0.3)',
                    boxShadow: '0 8px 32px rgba(139, 92, 246, 0.2), 0 0 0 1px rgba(139, 92, 246, 0.1) inset',
                  }}
                  initial={{ opacity: 0, x: 20, y: 10 }}
                  animate={{
                    opacity: 1,
                    x: 0,
                    y: 0,
                    transition: { delay: 0.2, duration: 0.4, ease: [0.16, 1, 0.3, 1] }
                  }}
                  exit={{ opacity: 0, x: 10 }}
                >
                  <h3 className="text-lg font-bold text-white mb-4">СТАТИСТИКА</h3>
                  <div className="space-y-4">
                    <div>
                      <div className="text-purple-300/70 text-xs mb-1">STT СООБЩЕНИЙ</div>
                      <div className="text-3xl font-bold text-purple-400">{sttCount}</div>
                    </div>
                    <div>
                      <div className="text-purple-300/70 text-xs mb-1">LLM ОТВЕТОВ</div>
                      <div className="text-3xl font-bold text-blue-400">{llmCount}</div>
                    </div>
                    <div>
                      <div className="text-purple-300/70 text-xs mb-1">ВСЕГО</div>
                      <div className="text-3xl font-bold text-white">{chatMessages.length}</div>
                    </div>
                  </div>
                </motion.div>
              </div>

              {/* Close Button */}
              <motion.button
                onClick={onClose}
                className="absolute -top-4 -right-4 w-12 h-12 rounded-full flex items-center justify-center text-white font-bold text-xl transition-all cursor-pointer hover:scale-110 active:scale-95"
                style={{
                  background: 'rgba(139, 92, 246, 0.9)',
                  boxShadow: '0 4px 16px rgba(139, 92, 246, 0.4)',
                }}
                initial={{ opacity: 0, scale: 0, rotate: -180 }}
                animate={{
                  opacity: 1,
                  scale: 1,
                  rotate: 0,
                  transition: { delay: 0.25, duration: 0.4, ease: [0.16, 1, 0.3, 1] }
                }}
                exit={{
                  opacity: 0,
                  scale: 0,
                  rotate: 180,
                  transition: { duration: 0.2 }
                }}
                whileHover={{
                  boxShadow: '0 6px 24px rgba(139, 92, 246, 0.6)',
                  transition: { duration: 0.2 }
                }}
                whileTap={{ scale: 0.9 }}
              >
                ✕
              </motion.button>
            </motion.div>
          </div>
        </>
      )}
    </AnimatePresence>,
    document.body
  )
}
