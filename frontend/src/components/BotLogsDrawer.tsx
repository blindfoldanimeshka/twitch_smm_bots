import { useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ScrollText, X } from 'lucide-react'
import { api, getErrorMessage } from '../api/client'
import type { Bot, BotLogEntry } from '../api/client'
import { useFocusTrap } from '../hooks/useFocusTrap'
import { Alert } from './ui/Alert'
import { Badge, type BadgeTone } from './ui/Badge'
import { EmptyState } from './ui/EmptyState'

interface BotLogsDrawerProps {
  /** Bot whose logs are shown, or null when the drawer is closed. */
  bot: Bot | null
  onClose: () => void
}

function levelTone(level: string): BadgeTone {
  switch (level.toLowerCase()) {
    case 'error':
      return 'error'
    case 'warning':
      return 'warning'
    case 'success':
      return 'success'
    default:
      return 'neutral'
  }
}

function formatTime(entry: BotLogEntry): string {
  return new Date(entry.created_at).toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

/** Right-side drawer with the last 100 log entries of a bot. */
export function BotLogsDrawer({ bot, onClose }: BotLogsDrawerProps) {
  const open = bot !== null
  const drawerRef = useRef<HTMLElement>(null)
  useFocusTrap(drawerRef, open, onClose)

  const { data: logs = [], isLoading, error } = useQuery({
    queryKey: ['bot-logs', bot?.id],
    queryFn: async () => (await api.getBotLogs(bot!.id, 100)).data,
    enabled: open,
  })

  if (!open || bot === null) {
    return null
  }

  return (
    <div
      className="modal-overlay"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <aside
        ref={drawerRef}
        role="dialog"
        aria-modal="true"
        aria-label={`Логи бота ${bot.name}`}
        tabIndex={-1}
        style={{
          alignSelf: 'stretch',
          margin: '0 0 0 auto',
          width: 520,
          maxWidth: '100%',
          display: 'flex',
          flexDirection: 'column',
          background: 'var(--card)',
          borderLeft: '1px solid var(--border)',
        }}
      >
        <header
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
            padding: '20px 24px 12px',
            borderBottom: '1px solid var(--border)',
          }}
        >
          <div>
            <h3 style={{ fontSize: 16, fontWeight: 700 }}>Логи — {bot.name}</h3>
            <p style={{ fontSize: 12, color: 'var(--muted)' }}>
              @{bot.twitch_login} · последние 100 записей
            </p>
          </div>
          <button type="button" className="modal__close" onClick={onClose} aria-label="Закрыть">
            <X size={18} aria-hidden="true" />
          </button>
        </header>

        <div
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: 16,
            display: 'grid',
            gap: 8,
            alignContent: 'start',
          }}
        >
          {error && <Alert variant="error">{getErrorMessage(error)}</Alert>}

          {isLoading ? (
            <span style={{ fontSize: 13, color: 'var(--muted)' }}>Загрузка…</span>
          ) : !error && logs.length === 0 ? (
            <EmptyState
              icon={ScrollText}
              title="Логов пока нет"
              description="Записи появятся, когда бот начнёт работать."
            />
          ) : (
            logs.map((log, index) => (
              <div
                key={log.id ?? index}
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'auto 1fr',
                  gap: 10,
                  alignItems: 'baseline',
                  padding: '8px 10px',
                  border: '1px solid var(--border)',
                  borderRadius: 12,
                }}
              >
                <Badge tone={levelTone(log.level)}>{log.level}</Badge>
                <div style={{ display: 'grid', gap: 2 }}>
                  <span style={{ fontSize: 13, wordBreak: 'break-word' }}>{log.message}</span>
                  <time style={{ fontSize: 12, color: 'var(--muted)' }}>
                    {formatTime(log)}
                  </time>
                </div>
              </div>
            ))
          )}
        </div>
      </aside>
    </div>
  )
}
