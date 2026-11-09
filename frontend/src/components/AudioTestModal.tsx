import { useState } from 'react'
import { api, isResponseError } from '../api/client'
import { useWebSocketStatus, useWebSocketSubscription } from '../hooks/useWebSocket'
import { Button } from './ui/Button'
import { Modal } from './ui/Modal'

const TRANSCRIPT_CAP = 200

interface AudioTestModalProps {
  isOpen: boolean
  onClose: () => void
  /** Channel whose audio the backend is analysing (first configured channel). */
  channel: string
}

/**
 * Live view of the backend audio→STT pipeline. Uses the shared WebSocket
 * singleton (no second socket) and the shared light-theme Modal.
 */
export function AudioTestModal({ isOpen, onClose, channel }: AudioTestModalProps) {
  const [transcription, setTranscription] = useState<string[]>([])
  const [testingLLM, setTestingLLM] = useState(false)
  const [llmResult, setLlmResult] = useState('')
  const { connected, error } = useWebSocketStatus()

  // Close clears the feed: the next open must show a fresh session, not the
  // stale transcript of the previous one.
  const handleClose = () => {
    setTranscription([])
    setLlmResult('')
    onClose()
  }

  useWebSocketSubscription((msg) => {
    if (!isOpen) return
    if (msg.type !== 'audio_context') return
    const text = msg.payload.context
    if (!text) return
    setTranscription((prev) => {
      const next = [...prev, text]
      return next.length > TRANSCRIPT_CAP ? next.slice(next.length - TRANSCRIPT_CAP) : next
    })
  })

  const testLLM = async () => {
    setTestingLLM(true)
    setLlmResult('')
    try {
      // Routed through the shared axios client: baseURL and X-API-Key come
      // from its configuration, no manual fetch here.
      const { data } = await api.testLLM('нейтрал')
      setLlmResult(data.message || 'Нет ответа')
    } catch (error) {
      // Same two outcomes as before: the backend answered with an error
      // status (generation failed) vs. we never got through.
      setLlmResult(isResponseError(error) ? 'Ошибка генерации' : 'Ошибка подключения к API')
    } finally {
      setTestingLLM(false)
    }
  }

  return (
    <Modal open={isOpen} onClose={handleClose} title="Тест транскрипции" width={720}>
      <div style={{ display: 'grid', gap: 12 }}>
        <p style={{ fontSize: 14, margin: 0 }}>
          Канал: <strong>{channel || 'не указан'}</strong>
          {' · '}
          Статус: {connected
            ? <span style={{ color: 'var(--success)' }}>подключено</span>
            : error
              ? <span style={{ color: 'var(--error)' }}>ошибка соединения</span>
              : <span style={{ color: 'var(--warning)' }}>подключение…</span>}
        </p>

        <div
          style={{
            maxHeight: 320,
            overflowY: 'auto',
            border: '1px solid var(--border)',
            borderRadius: 'var(--radius-md)',
            padding: 12,
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            fontSize: 13,
            lineHeight: 1.5,
          }}
        >
          {transcription.length === 0 ? (
            <p style={{ color: 'var(--muted)', textAlign: 'center', padding: '24px 0', margin: 0 }}>
              Ожидание транскрипции…
            </p>
          ) : (
            transcription.map((text, idx) => (
              <div
                key={idx}
                style={{
                  marginBottom: 8,
                  paddingLeft: 12,
                  borderLeft: '2px solid var(--accent-strong)',
                  color: 'var(--text)',
                }}
              >
                {text}
              </div>
            ))
          )}
        </div>

        <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <Button
            variant="secondary"
            size="sm"
            loading={testingLLM}
            disabled={transcription.length === 0}
            onClick={testLLM}
          >
            Тест LLM
          </Button>
          <span style={{ fontSize: 12, color: 'var(--muted)' }}>
            Проверка генерации сообщений LLM
          </span>
        </div>

        {llmResult && (
          <div
            style={{
              border: '1px solid var(--border)',
              borderRadius: 'var(--radius-md)',
              padding: 12,
              fontSize: 13,
            }}
          >
            <strong>LLM:</strong> {llmResult}
          </div>
        )}
      </div>
    </Modal>
  )
}
