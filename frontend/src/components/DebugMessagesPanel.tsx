import { useState, useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import { useWebSocket, type WSMessage } from '../hooks/useWebSocket'
import { motion, AnimatePresence } from 'framer-motion'

interface BotMessage {
  id: string
  botId: string
  botName: string
  message: string
  timestamp: string
  channel?: string
}

export function DebugMessagesPanel() {
  const [messages, setMessages] = useState<BotMessage[]>([])
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const [isExpanded, setIsExpanded] = useState(true)

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => {
      const res = await api.getSettings()
      return res.data
    },
    refetchInterval: 5000,
  })

  const handleWSMessage = (msg: WSMessage) => {
    if ((msg.type === 'bot_message_sent' || msg.type === 'bot_message') && msg.payload) {
      const newMessage: BotMessage = {
        id: Date.now().toString() + Math.random(),
        botId: msg.payload.bot_id || 'unknown',
        botName: msg.payload.bot_name || 'Unknown Bot',
        message: msg.payload.message || msg.payload.text || '',
        timestamp: new Date().toLocaleTimeString('ru-RU'),
        channel: msg.payload.channel,
      }
      setMessages(prev => [...prev.slice(-49), newMessage])
    }
  }

  useWebSocket(handleWSMessage)

  useEffect(() => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages])

  const isDebugMode = settings?.debug_mode || false

  if (!isDebugMode) {
    return null
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: 20 }}
      className="debug-messages-panel"
    >
      <div className="debug-messages-header" onClick={() => setIsExpanded(!isExpanded)}>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-green-500 rounded-full animate-pulse"></div>
          <h3 className="text-lg font-bold text-white">Сообщения ботов (Debug)</h3>
          <span className="text-xs text-gray-400">({messages.length})</span>
        </div>
        <button className="text-gray-400 hover:text-white transition-colors">
          {isExpanded ? '▼' : '▲'}
        </button>
      </div>

      <AnimatePresence>
        {isExpanded && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.3 }}
            className="debug-messages-content"
          >
            {messages.length === 0 ? (
              <div className="text-center text-gray-500 py-8">
                Ожидание сообщений от ботов...
              </div>
            ) : (
              <div className="space-y-2">
                {messages.map((msg) => (
                  <motion.div
                    key={msg.id}
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    className="debug-message-item"
                  >
                    <div className="flex items-start gap-3">
                      <div className="flex-shrink-0 w-8 h-8 bg-purple-600/30 rounded-full flex items-center justify-center">
                        <span className="text-xs font-bold text-purple-300">
                          {msg.botName.charAt(0).toUpperCase()}
                        </span>
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="text-sm font-semibold text-purple-300">
                            {msg.botName}
                          </span>
                          {msg.channel && (
                            <span className="text-xs text-gray-500">
                              → {msg.channel}
                            </span>
                          )}
                          <span className="text-xs text-gray-500 ml-auto">
                            {msg.timestamp}
                          </span>
                        </div>
                        <p className="text-sm text-gray-300 break-words">
                          {msg.message}
                        </p>
                      </div>
                    </div>
                  </motion.div>
                ))}
                <div ref={messagesEndRef} />
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  )
}
