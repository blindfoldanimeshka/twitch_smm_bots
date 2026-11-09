import { useQuery } from '@tanstack/react-query'
import { Settings as SettingsIcon } from 'lucide-react'
import { api } from '../../api/client'
import { useWebSocketStatus } from '../../hooks/useWebSocket'
import { Badge, type BadgeTone } from '../ui/Badge'
import type { Page } from './Sidebar'

interface TopbarProps {
  title: string
  onNavigate?: (page: Page) => void
}

interface ChipState {
  tone: BadgeTone
  label: string
}

function formatRefreshTime(dataUpdatedAt: number | undefined): string {
  if (!dataUpdatedAt) {
    return '—'
  }
  return new Date(dataUpdatedAt).toLocaleTimeString('ru-RU', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export function Topbar({ title, onNavigate }: TopbarProps) {
  const { connected } = useWebSocketStatus()

  const { data: settings, dataUpdatedAt } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => {
      const res = await api.getSettings()
      return res.data
    },
    refetchInterval: 30_000,
  })

  const { data: llmStatus } = useQuery({
    queryKey: ['llm-status'],
    queryFn: async () => {
      const res = await api.getLLMStatus()
      return res.data
    },
    refetchInterval: 60_000,
  })

  // Red requires certainty: settings must be loaded AND a key must be present.
  // While settings are loading (or their request failed, leaving `settings`
  // undefined) the chip stays neutral: a status check without a configured
  // key reports `available: false`, which must not flash red.
  const hasKey =
    settings !== undefined &&
    settings.llm_api_key_set !== false &&
    settings.llm_api_key_source !== 'none'

  let openRouter: ChipState
  if (settings === undefined) {
    openRouter = { tone: 'neutral', label: 'OpenRouter: …' }
  } else if (!hasKey) {
    // Missing key is a configuration problem (amber), not a connection
    // error — even when the status check reports `available: false`.
    openRouter = { tone: 'warning', label: 'OpenRouter: ключ не задан' }
  } else if (llmStatus?.available) {
    openRouter = { tone: 'success', label: 'OpenRouter: подключён' }
  } else if (llmStatus) {
    // A key is present and the backend's real check failed (auth/network).
    openRouter = { tone: 'error', label: 'OpenRouter: ошибка' }
  } else {
    openRouter = { tone: 'neutral', label: 'OpenRouter: …' }
  }

  const stt: ChipState = settings
    ? settings.audio_enabled
      ? { tone: 'success', label: 'STT: вкл' }
      : { tone: 'neutral', label: 'STT: выкл' }
    : { tone: 'neutral', label: 'STT: …' }

  const ws: ChipState = connected
    ? { tone: 'success', label: 'WS: онлайн' }
    : { tone: 'neutral', label: 'WS: офлайн' }

  return (
    <header className="topbar">
      <h1 className="topbar__title">{title}</h1>
      <div className="topbar__status">
        <Badge tone={openRouter.tone}>{openRouter.label}</Badge>
        <Badge tone={stt.tone}>{stt.label}</Badge>
        <Badge tone={ws.tone}>{ws.label}</Badge>
        {settings?.debug_mode && <Badge tone="accent">DEBUG</Badge>}
        <span className="topbar__refresh">обновлено {formatRefreshTime(dataUpdatedAt)}</span>
        {onNavigate && (
          <button
            type="button"
            className="topbar__settings-btn"
            onClick={() => onNavigate('settings')}
            aria-label="Открыть настройки"
            title="Открыть настройки"
          >
            <SettingsIcon size={16} aria-hidden="true" />
          </button>
        )}
      </div>
    </header>
  )
}
