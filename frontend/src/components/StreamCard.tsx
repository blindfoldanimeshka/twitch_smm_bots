import type { ReactNode } from 'react'
import { MonitorOff, Settings } from 'lucide-react'
import type { StreamStatus } from '../api/client'
import { Alert } from './ui/Alert'
import { Badge } from './ui/Badge'
import { Button } from './ui/Button'
import { Card } from './ui/Card'
import { EmptyState } from './ui/EmptyState'
import { useAppNavigate } from './layout/navigation'

/** Formats seconds as a compact clock uptime («1:04:32»). */
function formatUptime(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds))
  const h = Math.floor(total / 3600)
  const m = String(Math.floor((total % 3600) / 60)).padStart(2, '0')
  const s = String(total % 60).padStart(2, '0')
  return `${h}:${m}:${s}`
}

function formatClock(iso: string): string {
  const parsed = Date.parse(iso)
  if (Number.isNaN(parsed)) {
    return '—'
  }
  return new Date(parsed).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function Metric({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 96 }}>
      <span style={{ fontSize: 12, color: 'var(--muted)' }}>{label}</span>
      <span style={{ fontSize: 15, fontWeight: 600 }}>{value}</span>
    </div>
  )
}

/** Fixed 16:9 media frame: hosts the Twitch iframe or a neutral placeholder. */
function MediaFrame({ compact, children }: { compact: boolean; children: ReactNode }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'center' }}>
      <div
        style={{
          position: 'relative',
          width: '100%',
          maxWidth: compact ? 560 : 1080,
          aspectRatio: '16 / 9',
          borderRadius: 'var(--radius-md)',
          overflow: 'hidden',
          background: 'var(--bg)',
          border: '1px solid var(--border)',
        }}
      >
        {children}
      </div>
    </div>
  )
}

interface StreamCardProps {
  status?: StreamStatus
  /** Channel the status belongs to; empty means none is configured. */
  channel?: string
  /** Compact variant for the Dashboard: smaller player, fewer metrics. */
  compact?: boolean
  /** The status request is in flight and there is no data yet. */
  isLoading?: boolean
  /** The request itself failed (backend unreachable): an error, not a load. */
  isError?: boolean
  /** Retriggers the failed status request. */
  onRetry?: () => void
  className?: string
}

/**
 * Stream preview card: 16:9 player (or a neutral placeholder), metrics row and
 * state handling — loading, request failure (error + retry), GQL failure
 * (calm warning, player stays), untracked channel and no-channel (CTA).
 */
export function StreamCard({
  status,
  channel = '',
  compact = false,
  isLoading,
  isError,
  onRetry,
  className,
}: StreamCardProps) {
  const navigate = useAppNavigate()

  const hasChannel = channel !== ''
  // The status request failed: the card must say so instead of "loading" forever.
  const fetchFailed = isError === true
  // Twitch itself did not answer, but the backend did — a calm warning, the
  // player stays and the last successful attempt time is shown.
  const gqlFailed = !fetchFailed && status !== undefined && status.error !== undefined && status.error !== ''
  // The monitor reports `configured: false` (with no error) when the backend
  // does not track this channel yet — that is neither loading nor offline.
  const notTracked = !fetchFailed && !gqlFailed && status !== undefined && status.configured === false
  const loading = hasChannel && !fetchFailed && status === undefined && isLoading !== false
  const isLive = hasChannel && !fetchFailed && !gqlFailed && !notTracked && status?.live === true
  // Metrics and the chat badge are meaningless while the data is not live and
  // something failed: zeros would read as real numbers.
  const showDetails = hasChannel && status !== undefined && !notTracked && !(fetchFailed || gqlFailed)

  const metrics: Array<{ label: string; value: ReactNode }> = [
    { label: 'Uptime', value: isLive && status ? formatUptime(status.uptime_seconds) : '—' },
    { label: 'Всего зрителей', value: isLive && status ? status.viewers_total : '—' },
    { label: 'Активные', value: isLive && status ? status.active_viewers : '—' },
    { label: 'Пассивные', value: isLive && status ? status.passive_viewers : '—' },
  ]
  if (!compact) {
    metrics.push({ label: 'Игра', value: isLive && status && status.game !== '' ? status.game : '—' })
  }

  const player = (
    <MediaFrame compact={compact}>
      <iframe
        src={`https://player.twitch.tv/?channel=${encodeURIComponent(channel)}&parent=${window.location.hostname}&muted=true&autoplay=true`}
        title={`Трансляция канала ${channel}`}
        allow="fullscreen; autoplay"
        allowFullScreen
        style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', border: 'none' }}
      />
    </MediaFrame>
  )

  let body: ReactNode
  if (!hasChannel) {
    body = (
      <EmptyState
        icon={Settings}
        title="Канал не задан"
        description="Добавьте хотя бы один Twitch-канал в настройках, чтобы следить за стримом, зрителями и чатом."
        action={<Button onClick={() => navigate('settings')}>Открыть настройки</Button>}
      />
    )
  } else if (fetchFailed) {
    body = (
      <>
        {player}
        <Alert variant="error" title="Не удалось получить данные Twitch">
          {status
            ? `Обновить не удалось. Последняя попытка: ${formatClock(status.updated_at)}.`
            : 'Сервер не отвечает: статус канала неизвестен.'}
        </Alert>
        {onRetry && (
          <div>
            <Button variant="secondary" size="sm" onClick={onRetry}>
              Повторить
            </Button>
          </div>
        )}
      </>
    )
  } else if (loading) {
    body = (
      <EmptyState
        title="Загружаем статус…"
        description="Запрашиваем состояние трансляции у Twitch."
      />
    )
  } else if (gqlFailed && status) {
    // GQL failure: the player stays, the warning is calm and carries the time
    // of the last attempt (the backend stamps `updated_at` before it knows
    // whether the request succeeds) instead of fake zeros.
    body = (
      <>
        {player}
        <Alert variant="warning" title="Не удалось получить данные Twitch">
          Метрики появятся, когда Twitch снова ответит. Последняя попытка обновления: {formatClock(status.updated_at)}.
        </Alert>
      </>
    )
  } else if (notTracked) {
    body = (
      <EmptyState
        icon={MonitorOff}
        title="Канал не отслеживается"
        description="Сервер не следит за этим каналом — проверьте список каналов в настройках."
        action={
          <Button variant="secondary" onClick={() => navigate('settings')}>
            Открыть настройки
          </Button>
        }
      />
    )
  } else if (!isLive) {
    body = (
      <MediaFrame compact={compact}>
        <EmptyState
          icon={MonitorOff}
          title="Стрим не идёт"
          description="Канал сейчас офлайн. Метрики появятся, когда трансляция начнётся."
        />
      </MediaFrame>
    )
  } else {
    body = (
      <>
        {player}
        {status.title && (
          <div style={{ marginTop: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            <strong>{status.title}</strong>
          </div>
        )}
      </>
    )
  }

  return (
    <Card
      className={className}
      title="Трансляция"
      subtitle={hasChannel ? `Канал ${channel}` : undefined}
      actions={
        showDetails && status ? (
          <Badge tone={status.chat_connected ? 'success' : 'neutral'}>
            {status.chat_connected ? 'Чат: подключён' : 'Чат: не подключён'}
          </Badge>
        ) : undefined
      }
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {body}
        {showDetails && (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12 }}>
            {metrics.map((metric) => (
              <Metric key={metric.label} label={metric.label} value={metric.value} />
            ))}
          </div>
        )}
      </div>
    </Card>
  )
}
