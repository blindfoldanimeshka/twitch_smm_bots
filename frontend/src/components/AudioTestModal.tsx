import { useEffect, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'

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

    const ws = new WebSocket(`ws://localhost:8080/api/ws?api_key=secret-api-key-12345`)

    ws.onopen = () => {
      setStatus('connected')
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'audio_context' && msg.payload?.context) {
          setTranscription(prev => [...prev, msg.payload.context])
        }
      } catch (e) {
        console.error('WebSocket message parse error:', e)
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
      const response = await fetch('http://localhost:8080/api/settings/test-llm', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-Key': 'secret-api-key-12345'
        },
        body: JSON.stringify({ personality: 'нейтрал' })
      })
      if (response.ok) {
        const data = await response.json()
        setLlmResult(data.message || 'Нет ответа')
      } else {
        setLlmResult('Ошибка генерации')
      }
    } catch (error) {
      setLlmResult('Ошибка подключения к API')
    } finally {
      setTestingLLM(false)
    }
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
          backgroundColor: 'rgba(0, 0, 0, 0.8)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 9999,
          padding: 'var(--space-6)'
        }}
        onClick={onClose}
      >
        <motion.div
          initial={{ scale: 0.9, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          exit={{ scale: 0.9, opacity: 0 }}
          onClick={(e) => e.stopPropagation()}
          style={{
            backgroundColor: 'var(--bg-primary)',
            border: '1px solid var(--neon-green)',
            borderRadius: '8px',
            padding: 'var(--space-6)',
            maxWidth: '800px',
            width: '100%',
            maxHeight: '80vh',
            display: 'flex',
            flexDirection: 'column',
            boxShadow: '0 0 20px rgba(0, 255, 136, 0.3)'
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-4)' }}>
            <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '1.5rem', color: 'var(--neon-green)' }}>
              🎤 ТЕСТ ТРАНСКРИПЦИИ
            </h2>
            <button
              onClick={onClose}
              className="btn ghost cursor-target"
              style={{ padding: 'var(--space-2) var(--space-4)' }}
            >
              ✕ ЗАКРЫТЬ
            </button>
          </div>

          <div style={{ marginBottom: 'var(--space-4)' }}>
            <p style={{ color: 'var(--text-secondary)' }}>
              Канал: <strong style={{ color: 'var(--text-primary)' }}>{channel || 'не указан'}</strong>
            </p>
            <p style={{ color: 'var(--text-secondary)', marginTop: 'var(--space-2)' }}>
              Статус: {' '}
              {status === 'connecting' && <span style={{ color: 'var(--neon-yellow)' }}>⟳ Подключение...</span>}
              {status === 'connected' && <span style={{ color: 'var(--neon-green)' }}>✓ Подключено</span>}
              {status === 'error' && <span style={{ color: 'var(--neon-red)' }}>✗ Ошибка подключения</span>}
            </p>
          </div>

          <div
            style={{
              flex: 1,
              overflowY: 'auto',
              backgroundColor: 'rgba(0, 0, 0, 0.3)',
              border: '1px solid var(--border-primary)',
              borderRadius: '4px',
              padding: 'var(--space-4)',
              fontFamily: 'monospace',
              fontSize: '0.9rem',
              lineHeight: '1.6',
              marginBottom: 'var(--space-4)'
            }}
          >
            {transcription.length === 0 ? (
              <p style={{ color: 'var(--text-secondary)', textAlign: 'center', padding: 'var(--space-8)' }}>
                Ожидание транскрипции...
              </p>
            ) : (
              transcription.map((text, idx) => (
                <motion.div
                  key={idx}
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  style={{
                    marginBottom: 'var(--space-3)',
                    padding: 'var(--space-2)',
                    borderLeft: '2px solid var(--neon-green)',
                    paddingLeft: 'var(--space-3)',
                    color: 'var(--text-primary)'
                  }}
                >
                  {text}
                </motion.div>
              ))
            )}
          </div>

          <div style={{ display: 'flex', gap: 'var(--space-4)', alignItems: 'center' }}>
            <button
              onClick={testLLM}
              disabled={testingLLM || transcription.length === 0}
              className="btn primary cursor-target"
            >
              {testingLLM ? '⟳ ГЕНЕРАЦИЯ...' : '🤖 ТЕСТ LLM'}
            </button>
            {llmResult && (
              <div style={{
                flex: 1,
                padding: 'var(--space-3)',
                backgroundColor: 'rgba(138, 43, 226, 0.1)',
                border: '1px solid var(--neon-purple)',
                borderRadius: '4px',
                fontSize: '0.9rem'
              }}>
                <strong style={{ color: 'var(--neon-purple)' }}>LLM:</strong> {llmResult}
              </div>
            )}
          </div>
        </motion.div>
      </motion.div>
      )}
    </AnimatePresence>
  )
}
