import { useCallback, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import { useStreamStatus } from '../hooks/useStreamStatus'
import { useWebSocketSubscription, type WSMessage } from '../hooks/useWebSocket'
import { PageHeader } from '../components/layout/PageHeader'
import { Card } from '../components/ui/Card'
import { AsyncState } from '../components/ui/AsyncState'
import { Select } from '../components/ui/Select'
import { StreamCard } from '../components/StreamCard'
import { ChatFeed, type ChatPresenceEvent } from '../components/ChatFeed'
import { SetupBanner } from '../components/SetupBanner'

const EVENT_CAP = 50

/** Formats an ISO timestamp as «HH:MM» for the chatter list. */
function formatTime(iso: string): string {
  const parsed = Date.parse(iso)
  if (Number.isNaN(parsed)) {
    return ''
  }
  return new Date(parsed).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

export default function Stream() {
  // Shared ['settings'] cache (same key as the Topbar, which owns the polling;
  // the app-level LiveDataBridge invalidates it when settings change).
  const {
    data: settings,
    isLoading: settingsLoading,
    isError: settingsFailed,
    refetch: refetchSettings,
  } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => (await api.getSettings()).data,
  })
  const channels = settings?.channels.filter((c) => c.trim() !== '') ?? []

  const [selected, setSelected] = useState('')
  // Falls back to the first channel until the user picks one (or if the
  // selected channel disappears from settings).
  const channel = channels.includes(selected) ? selected : (channels[0] ?? '')
  const {
    data: status,
    isLoading: statusLoading,
    isError: statusFailed,
    refetch: refetchStatus,
  } = useStreamStatus(channel || undefined, channel !== '')

  // Presence events arrive over the shared WebSocket singleton; this page only
  // adds a subscriber, no new socket.
  const [presenceEvents, setPresenceEvents] = useState<ChatPresenceEvent[]>([])
  useWebSocketSubscription(
    useCallback((msg: WSMessage) => {
      if (msg.type !== 'bot_presence_changed') {
        return
      }
      const { login, state, at } = msg.payload
      if (!login) {
        return
      }
      // Only `state` keeps a runtime guard: the backend treats it as a free
      // string and any unexpected value must read as "left", not crash.
      const event: ChatPresenceEvent = {
        login,
        state: typeof state === 'string' ? state : '',
        at: at || new Date().toISOString(),
      }
      setPresenceEvents((prev) => {
        const next = [...prev, event]
        return next.length > EVENT_CAP ? next.slice(-EVENT_CAP) : next
      })
    }, []),
  )

  // The backend answered, but Twitch itself failed: chat data is absent, the
  // chat is not. Nothing may claim "nobody wrote" while this flag is set.
  const gqlOutage = Boolean(status?.error)

  return (
    <>
      <PageHeader
        title="Стрим"
        description="Плеер, метрики зрителей и чат выбранного канала"
        actions={
          channels.length > 0 && (
            <Select
              aria-label="Канал"
              options={channels.map((c) => ({ value: c, label: c }))}
              value={channel}
              onChange={(e) => setSelected(e.target.value)}
              style={{ minWidth: 220 }}
            />
          )
        }
      />

      {/* The first settings load failed: an error with retry, not "channel is
          missing". Settings kept from a failed background poll stay on screen. */}
      <AsyncState
        loading={settingsLoading && !settings}
        loadingText="Загружаем настройки…"
        error={settingsFailed && !settings}
        errorText="Не удалось загрузить настройки — список каналов недоступен."
        onRetry={() => refetchSettings()}
      >
        {channels.length === 0 ? (
          <SetupBanner
            tone="info"
            title="Канал не задан"
            description="Добавьте Twitch-каналы в настройках — здесь появятся плеер, зрители и чат."
            actionPage="settings"
          />
        ) : (
          <div style={{ display: 'grid', gap: 16 }}>
            <StreamCard
              status={status}
              channel={channel}
              isLoading={statusLoading}
              isError={statusFailed}
              onRetry={() => refetchStatus()}
            />
            <Card title="Активные чаттеры" subtitle="Писали в чат за последние 5 минут">
              <AsyncState
                loading={statusLoading}
                loadingText="Загружаем чат…"
                error={statusFailed}
                errorText="Не удалось получить список активных чаттеров."
                onRetry={() => refetchStatus()}
                isEmpty={!gqlOutage && (status?.chatters.length ?? 0) === 0}
                emptyTitle="Пока никто не писал"
              >
                {gqlOutage && (status?.chatters.length ?? 0) === 0 ? (
                  <p style={{ margin: 0, fontSize: 13, color: 'var(--muted)' }}>
                    Twitch не отвечает — данных чата нет
                  </p>
                ) : (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                    {status?.chatters.slice(0, 10).map((chatter) => (
                      <span
                        key={chatter.login}
                        style={{
                          display: 'inline-flex',
                          alignItems: 'baseline',
                          gap: 6,
                          padding: '4px 12px',
                          border: '1px solid var(--border)',
                          borderRadius: 'var(--radius-pill)',
                          fontSize: 13,
                        }}
                      >
                        <strong>@{chatter.login}</strong>
                        <span style={{ fontSize: 12, color: 'var(--muted)' }}>{formatTime(chatter.last_message_at)}</span>
                      </span>
                    ))}
                  </div>
                )}
              </AsyncState>
            </Card>
            <Card title="Чат" subtitle={`Последние 50 сообщений канала ${channel} и события присутствия ботов`}>
              <ChatFeed
                messages={status?.messages ?? []}
                events={presenceEvents}
                loading={statusLoading}
                error={statusFailed}
                onRetry={() => refetchStatus()}
                isEmpty={!gqlOutage && (status?.messages.length ?? 0) === 0}
              />
            </Card>
          </div>
        )}
      </AsyncState>
    </>
  )
}
