import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { motion, AnimatePresence } from 'framer-motion'
import { config } from '../config'

interface AudioTestModalProps {
  isOpen: boolean
  onClose: () => void
  channel: string
}

export function AudioTestModal({ isOpen, onClose, channel }: AudioTestModalProps) {
  const [transcription, setTranscription] = useState<string[]>([])
  const [status, setStatus] = useState<'connecting' | 'connected' | 'error'>('connecting')
  const [testingLLM, setTestingLLM] = useState(false)
  const [llmResult, setLlmResult] = useState('')

  useEffect(() => {
    if (!isOpen) {
      setTranscription([])
      setStatus('connecting')
      setLlmResult('')
      return
    }

    const ws = new WebSocket(`${config.wsUrl}/api/ws?api_key=${config.apiKey}`)

    ws.onopen = () => {
      setStatus('connected')
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'audio_context' && msg.payload?.context) {
          setTranscription(prev => [...prev, msg.payload.context])
        }
      } catch {
        // Malformed payload: drop silently (never log message contents).
      }
    }

    ws.onerror = () => {
      setStatus('error')
    }

    ws.onclose = () => {
      setStatus('connecting')
    }

    return () => {
      ws.close()
    }
  }, [isOpen])

  const testLLM = async () => {
    setTestingLLM(true)
    setLlmResult('')
    try {
      const response = await fetch(`${config.apiUrl}/api/settings/test-llm`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-Key': config.apiKey
        },
        body: JSON.stringify({ personality: 'нейтрал' })
      })
      if (response.ok) {
        const data = await response.json()
        setLlmResult(data.message || 'Нет ответа')
      } else {
        setLlmResult('Ошибка генерации')
      }
    } catch {
      setLlmResult('Ошибка подключения к API')
    } finally {
      setTestingLLM(false)
    }
  }

  if (!isOpen) return null

  return createPortal(
    <AnimatePresence mode="wait">
      {isOpen && (
        <>
          {/* Backdrop */}
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
          <div className="fixed inset-0 z-[10001] flex items-center justify-center pointer-events-none p-6">
            <motion.div
              onClick={(e) => e.stopPropagation()}
              className="pointer-events-auto cursor-default"
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
                  ease: [0.16, 1, 0.3, 1]
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
                transformStyle: 'preserve-3d',
                backgroundColor: 'rgba(15, 15, 20, 0.95)',
                backdropFilter: 'blur(10px)',
                border: '1px solid rgba(139, 92, 246, 0.3)',
                borderRadius: '16px',
                padding: '24px',
                maxWidth: '800px',
                width: '100%',
                maxHeight: '80vh',
                display: 'flex',
                flexDirection: 'column',
                boxShadow: '0 8px 32px rgba(139, 92, 246, 0.2), 0 0 0 1px rgba(139, 92, 246, 0.1) inset'
              }}
            >
              {/* Header */}
              <motion.div
                style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}
                initial={{ opacity: 0, y: -10 }}
                animate={{
                  opacity: 1,
                  y: 0,
                  transition: { delay: 0.1, duration: 0.3, ease: [0.16, 1, 0.3, 1] }
                }}
              >
                <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '1.5rem', color: '#8b5cf6' }}>
                  🎤 ТЕСТ ТРАНСКРИПЦИИ
                </h2>
                <motion.button
                  onClick={onClose}
                  className="cursor-pointer"
                  style={{
                    background: 'rgba(139, 92, 246, 0.2)',
                    border: '1px solid rgba(139, 92, 246, 0.4)',
                    borderRadius: '8px',
                    padding: '8px 16px',
                    color: '#fff',
                    fontFamily: 'var(--font-display)',
                    fontSize: '0.9rem',
                    cursor: 'pointer',
                    transition: 'all 0.2s'
                  }}
                  whileHover={{
                    background: 'rgba(139, 92, 246, 0.3)',
                    scale: 1.05
                  }}
                  whileTap={{ scale: 0.95 }}
                >
                  ✕ ЗАКРЫТЬ
                </motion.button>
              </motion.div>

              {/* Info Section */}
              <motion.div
                style={{ marginBottom: '20px' }}
                initial={{ opacity: 0, y: -5 }}
                animate={{
                  opacity: 1,
                  y: 0,
                  transition: { delay: 0.15, duration: 0.3, ease: [0.16, 1, 0.3, 1] }
                }}
              >
                <p style={{ color: '#a1a1a1', marginBottom: '8px' }}>
                  Канал: <strong style={{ color: '#fafafa' }}>{channel || 'не указан'}</strong>
                </p>
                <p style={{ color: '#a1a1a1' }}>
                  Статус: {' '}
                  {status === 'connecting' && <span style={{ color: '#fbbf24' }}>⟳ Подключение...</span>}
                  {status === 'connected' && <span style={{ color: '#8b5cf6' }}>✓ Подключено</span>}
                  {status === 'error' && <span style={{ color: '#ff6467' }}>✗ Ошибка подключения</span>}
                </p>
              </motion.div>

              {/* Transcription Box */}
              <motion.div
                style={{
                  flex: 1,
                  overflowY: 'auto',
                  backgroundColor: 'rgba(0, 0, 0, 0.5)',
                  border: '1px solid rgba(139, 92, 246, 0.3)',
                  borderRadius: '12px',
                  padding: '16px',
                  fontFamily: 'monospace',
                  fontSize: '0.9rem',
                  lineHeight: '1.6',
                  marginBottom: '16px'
                }}
                initial={{ opacity: 0, y: 10 }}
                animate={{
                  opacity: 1,
                  y: 0,
                  transition: { delay: 0.2, duration: 0.3, ease: [0.16, 1, 0.3, 1] }
                }}
              >
            {transcription.length === 0 ? (
              <p style={{ color: '#a1a1a1', textAlign: 'center', padding: '40px 0' }}>
                Ожидание транскрипции...
              </p>
            ) : (
              transcription.map((text, idx) => (
                <motion.div
                  key={idx}
                  initial={{ opacity: 0, x: -10 }}
                  animate={{
                    opacity: 1,
                    x: 0,
                    transition: { delay: idx * 0.05, duration: 0.3 }
                  }}
                  style={{
                    marginBottom: '12px',
                    padding: '8px',
                    borderLeft: '2px solid #8b5cf6',
                    paddingLeft: '12px',
                    color: '#fafafa'
                  }}
                >
                  {text}
                </motion.div>
              ))
            )}
          </motion.div>

          {/* LLM Test Section */}
          <motion.div
            style={{ display: 'flex', gap: '16px', alignItems: 'center' }}
            initial={{ opacity: 0, y: 10 }}
            animate={{
              opacity: 1,
              y: 0,
              transition: { delay: 0.25, duration: 0.3, ease: [0.16, 1, 0.3, 1] }
            }}
          >
            <motion.button
              onClick={testLLM}
              disabled={testingLLM || transcription.length === 0}
              className="cursor-pointer"
              style={{
                background: testingLLM || transcription.length === 0 ? 'rgba(139, 92, 246, 0.2)' : 'rgba(139, 92, 246, 0.3)',
                border: '1px solid rgba(139, 92, 246, 0.4)',
                borderRadius: '8px',
                padding: '12px 24px',
                color: '#fff',
                fontFamily: 'var(--font-display)',
                fontSize: '0.9rem',
                cursor: testingLLM || transcription.length === 0 ? 'not-allowed' : 'pointer',
                opacity: testingLLM || transcription.length === 0 ? 0.5 : 1,
                transition: 'all 0.2s'
              }}
              whileHover={testingLLM || transcription.length === 0 ? {} : {
                background: 'rgba(139, 92, 246, 0.4)',
                scale: 1.05
              }}
              whileTap={testingLLM || transcription.length === 0 ? {} : { scale: 0.95 }}
            >
              {testingLLM ? '⟳ ГЕНЕРАЦИЯ...' : '🤖 ТЕСТ LLM'}
            </motion.button>
            {llmResult && (
              <motion.div
                style={{
                  flex: 1,
                  padding: '12px',
                  backgroundColor: 'rgba(139, 92, 246, 0.15)',
                  border: '1px solid rgba(139, 92, 246, 0.3)',
                  borderRadius: '8px',
                  fontSize: '0.9rem'
                }}
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
              >
                <strong style={{ color: '#8b5cf6' }}>LLM:</strong> <span style={{ color: '#fafafa' }}>{llmResult}</span>
              </motion.div>
            )}
          </motion.div>
            </motion.div>
          </div>
        </>
      )}
    </AnimatePresence>,
    document.body
  )
}
