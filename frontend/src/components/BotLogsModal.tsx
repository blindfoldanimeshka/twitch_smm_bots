import React, { useEffect, useState, useRef, useCallback } from 'react'
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

function WarpAnimations() {
  const enterDuration = 0.5
  const exitDuration = 0.25
  return (
    <>
      <motion.div
        className="absolute rounded-full h-1/2 w-1/2 blur-lg origin-center will-change-transform left-[25%] top-[100%]"
        initial={{
          scale: 0,
          opacity: 1,
          backgroundColor: 'hsl(280, 64%, 77%)',
        }}
        animate={{
          scale: 10,
          opacity: 0.2,
          backgroundColor: 'hsl(270, 93%, 56%)',
          transition: {
            duration: enterDuration,
            opacity: { duration: enterDuration, ease: 'easeInOut' },
          },
        }}
        exit={{
          scale: 0,
          opacity: 1,
          backgroundColor: 'hsl(280, 64%, 77%)',
          transition: { duration: exitDuration },
        }}
      />
      <motion.div
        className="absolute blur-[100px] rounded-full bg-purple-500/90 h-full w-full top-[-25%] left-[-50%]"
        initial={{ opacity: 0 }}
        animate={{
          opacity: 0.9,
          transition: {
            duration: enterDuration,
            scale: {
              duration: 15,
              repeat: Infinity,
              repeatType: 'loop',
              ease: 'easeInOut',
              delay: 0.35,
            },
          },
          scale: [1, 0.7, 1],
        }}
        exit={{
          opacity: 0,
          transition: { duration: exitDuration },
        }}
      />
      <motion.div
        className="absolute blur-[100px] rounded-full bg-purple-500/80 w-full h-full top-[25%] left-[50%]"
        initial={{ opacity: 0 }}
        animate={{
          opacity: 0.9,
          transition: {
            duration: enterDuration,
            scale: {
              duration: 15,
              repeat: Infinity,
              repeatType: 'loop',
              ease: 'easeInOut',
              delay: 0.35,
            },
          },
          scale: [1, 0.7, 1],
        }}
        exit={{
          opacity: 0,
          transition: { duration: exitDuration },
        }}
      />
    </>
  )
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

  return (
    <AnimatePresence>
      {isOpen && (
        <div className="absolute">
          <div className="fixed inset-0 z-50 bg-purple-500/10">
            <WarpAnimations />
          </div>

          <motion.div
            onClick={onClose}
            className="fixed inset-0 flex items-center justify-center z-[1000]"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.35, ease: [0.59, 0, 0.35, 1] }}
          >
            <motion.div
              className="relative flex gap-4 w-[90vw] h-[80vh] max-w-[1400px]"
              onClick={(e) => e.stopPropagation()}
              initial={{
                rotateX: -5,
                skewY: -1.5,
                scaleY: 2,
                scaleX: 0.4,
                y: 100,
              }}
              animate={{
                rotateX: 0,
                skewY: 0,
                scaleY: 1,
                scaleX: 1,
                y: 0,
                transition: {
                  duration: 0.35,
                  ease: [0.59, 0, 0.35, 1],
                  y: { type: 'spring', visualDuration: 0.7, bounce: 0.2 },
                },
              }}
              exit={{
                rotateX: -5,
                skewY: -1.5,
                scaleY: 2,
                scaleX: 0.4,
                y: 100,
              }}
              transition={{ duration: 0.35, ease: [0.59, 0, 0.35, 1] }}
              style={{
                transformPerspective: 1000,
                originX: 0.5,
                originY: 0,
              }}
            >
              {/* LEFT BLOCK - Messages */}
              <div
                className="flex-1 rounded-2xl overflow-hidden"
                style={{
                  background: 'rgba(15, 15, 20, 0.85)',
                  backdropFilter: 'blur(20px) saturate(180%)',
                  border: '1px solid rgba(139, 92, 246, 0.2)',
                  boxShadow: '0 8px 32px rgba(139, 92, 246, 0.15)',
                }}
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
                    chatMessages.map((msg) => (
                      <div
                        key={msg.id}
                        className="rounded-xl p-4"
                        style={{
                          background: msg.type === 'stt'
                            ? 'rgba(139, 92, 246, 0.1)'
                            : 'rgba(59, 130, 246, 0.1)',
                          border: `1px solid ${msg.type === 'stt' ? 'rgba(139, 92, 246, 0.3)' : 'rgba(59, 130, 246, 0.3)'}`,
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
                      </div>
                    ))
                  )}
                  <div ref={messagesEndRef} />
                </div>
              </div>

              {/* RIGHT COLUMN */}
              <div className="flex flex-col gap-4 w-[350px]">
                {/* TOP RIGHT - Bot Info */}
                <div
                  className="rounded-2xl p-6 flex-1"
                  style={{
                    background: 'rgba(15, 15, 20, 0.85)',
                    backdropFilter: 'blur(20px) saturate(180%)',
                    border: '1px solid rgba(139, 92, 246, 0.2)',
                    boxShadow: '0 8px 32px rgba(139, 92, 246, 0.15)',
                  }}
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
                </div>

                {/* BOTTOM RIGHT - Stats */}
                <div
                  className="rounded-2xl p-6 flex-1"
                  style={{
                    background: 'rgba(15, 15, 20, 0.85)',
                    backdropFilter: 'blur(20px) saturate(180%)',
                    border: '1px solid rgba(139, 92, 246, 0.2)',
                    boxShadow: '0 8px 32px rgba(139, 92, 246, 0.15)',
                  }}
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
                </div>
              </div>

              {/* Close Button */}
              <button
                onClick={onClose}
                className="absolute -top-4 -right-4 w-12 h-12 rounded-full flex items-center justify-center text-white font-bold text-xl hover:scale-110 transition-transform"
                style={{
                  background: 'rgba(139, 92, 246, 0.9)',
                  boxShadow: '0 4px 16px rgba(139, 92, 246, 0.4)',
                }}
              >
                ✕
              </button>
            </motion.div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  )
}
