import { useCallback } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, Bot as BotIcon, MessageSquare, Users } from 'lucide-react'
import { api, getErrorMessage, type Bot } from '../api/client'
import { useStats } from '../hooks/useStats'
import { useStreamStatus } from '../hooks/useStreamStatus'
import { useWebSocketSubscription, type WSMessage } from '../hooks/useWebSocket'
import { PageHeader } from '../components/layout/PageHeader'
import { Alert } from '../components/ui/Alert'
import { Badge, type BadgeTone } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { EmptyState } from '../components/ui/EmptyState'
import { StatCard } from '../components/ui/StatCard'
import { StreamCard } from '../components/StreamCard'
import { SetupBanner, navigateToPage } from '../components/SetupBanner'

/** Bot mutation helper: every successful action refreshes bots and stats. */
function useBotMutation<T>(mutationFn: (arg: T) => Promise<unknown>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] })
      queryClient.invalidateQueries({ queryKey: ['stats'] })
    },
  })
}

/**
 * True when no OpenRouter API key is configured. Uses the shared LLM status
 * endpoint (`api.getLLMStatus`, cache key shared with the Topbar): a status
 * check without a configured key reports `available: false` with a
 * key-missing error — a setup problem (amber hint), never a red connection
 * failure. The settings check covers the case where the status endpoint has
 * not answered yet.
 */
function useOpenRouterKeyMissing(): boolean {
  const { data: llmStatus } = useQuery({
    queryKey: ['llm-status'],
    queryFn: async () => (await api.getLLMStatus()).data,
    refetchInterval: 60_000,
  })

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => (await api.getSettings()).data,
    refetchInterval: 30_000,
  })

  const settingsSayMissing =
    settings !== undefined &&
    (settings.llm_api_key_set === false || settings.llm_api_key_source === 'none')

  const statusError = (llmStatus?.error || llmStatus?.last_error || '').toLowerCase()
  const statusSaysMissing = llmStatus !== undefined && !llmStatus.available && statusError.includes('not configured')

  return settingsSayMissing || statusSaysMissing
}

/** A bot with a live or cooling session is running and can be stopped. */
function isRunning(bot: Bot): boolean {
  return bot.presence_state === 'online' || bot.presence_state === 'cooldown'
}

function presenceBadge(bot: Bot): { tone: BadgeTone; label: string } {
  switch (bot.presence_state) {
    case 'online':
      return { tone: 'success', label: 'в эфире' }
    case 'cooldown': {
      const until = bot.cooldown_until
        ? new Date(bot.cooldown_until).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
        : ''
      return { tone: 'warning', label: until ? `пауза до ${until}` : 'пауза' }
    }
    case 'error':
      return { tone: 'error', label: 'ошибка' }
    default:
      return { tone: 'neutral', label: 'остановлен' }
  }
}

export default function Dashboard() {
  const queryClient = useQueryClient()

  const { data: bots = [] } = useQuery({
    queryKey: ['bots'],
    queryFn: async () => (await api.getBots()).data,
  })

  const { data: stats } = useStats()

  // Shared ['settings'] cache (same key as the Topbar), so no extra requests.
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => (await api.getSettings()).data,
    refetchInterval: 30_000,
  })
  const channels = settings?.channels.filter((c) => c.trim() !== '') ?? []
  const channel = channels[0] ?? ''
  const { data: streamStatus } = useStreamStatus(channel || undefined, channel !== '')

  const openRouterKeyMissing = useOpenRouterKeyMissing()

  const startBot = useBotMutation((id: string) => api.startBot(id))
  const stopBot = useBotMutation((id: string) => api.stopBot(id))
  const startAll = useBotMutation(() => api.startAll())
  const stopAll = useBotMutation(() => api.stopAll())

  const mutationError = startBot.error ?? stopBot.error ?? startAll.error ?? stopAll.error

  // Shared WebSocket (no new socket): refresh bots and stats on changes.
  useWebSocketSubscription(
    useCallback(
      (msg: WSMessage) => {
        if (msg.type === 'bot_status_changed' || msg.type === 'stats_updated') {
          queryClient.invalidateQueries({ queryKey: ['bots'] })
          queryClient.invalidateQueries({ queryKey: ['stats'] })
        }
      },
      [queryClient],
    ),
  )

  const viewersKnown = streamStatus?.live === true && !streamStatus.error

  return (
    <>
      <PageHeader title="Дашборд" description="Сводка по ботам и текущая трансляция" />

      <div style={{ display: 'grid', gap: 16 }}>
        {/* Setup banners are calm hints, not errors. */}
        {channels.length === 0 && (
          <SetupBanner
            tone="info"
            title="Канал не задан"
            description="Добавьте Twitch-канал в настройках — без него не видно стрима, зрителей и чата."
            actionPage="settings"
          />
        )}
        {openRouterKeyMissing && (
          <SetupBanner
            tone="warning"
            title="OpenRouter не настроен"
            description="Боты не смогут генерировать сообщения, пока не задан API-ключ OpenRouter."
            actionPage="settings"
          />
        )}

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 16 }}>
          <StatCard title="Всего ботов" value={stats ? stats.total_bots : '—'} icon={BotIcon} />
          <StatCard
            title="Запущено"
            value={stats ? stats.running_bots : '—'}
            icon={Activity}
            tone={stats && stats.running_bots > 0 ? 'success' : 'neutral'}
          />
          <StatCard title="Сообщений/мин" value={stats ? stats.messages_per_min : '—'} icon={MessageSquare} />
          <StatCard
            title="Зрителей сейчас"
            value={viewersKnown ? streamStatus.viewers_total : '—'}
            icon={Users}
            hint={channel ? `Канал ${channel}` : 'Канал не задан'}
          />
        </div>

        <StreamCard status={streamStatus} channel={channel} compact />

        <Card
          title="Активные боты"
          actions={
            <>
              <Button
                variant="accent"
                size="sm"
                loading={startAll.isPending}
                disabled={bots.length === 0}
                onClick={() => startAll.mutate(undefined)}
              >
                Запустить всех
              </Button>
              <Button
                variant="secondary"
                size="sm"
                loading={stopAll.isPending}
                disabled={bots.length === 0}
                onClick={() => stopAll.mutate(undefined)}
              >
                Остановить всех
              </Button>
            </>
          }
        >
          <div style={{ display: 'grid', gap: 12 }}>
            {mutationError && <Alert variant="error">{getErrorMessage(mutationError)}</Alert>}

            {bots.length === 0 ? (
              <EmptyState
                icon={BotIcon}
                title="Пока нет ботов"
                description="Создайте первого бота — он появится в этой таблице."
                action={<Button onClick={() => navigateToPage('bots')}>Добавить бота</Button>}
              />
            ) : (
              <table className="table">
                <thead>
                  <tr>
                    <th>Имя</th>
                    <th>Логин</th>
                    <th>Статус</th>
                    <th>Действия</th>
                  </tr>
                </thead>
                <tbody>
                  {bots.map((bot) => {
                    const badge = presenceBadge(bot)
                    return (
                      <tr key={bot.id}>
                        <td>{bot.name}</td>
                        <td>{bot.twitch_login}</td>
                        <td>
                          <Badge tone={badge.tone}>{badge.label}</Badge>
                        </td>
                        <td>
                          {isRunning(bot) ? (
                            <Button
                              variant="secondary"
                              size="sm"
                              loading={stopBot.isPending && stopBot.variables === bot.id}
                              onClick={() => stopBot.mutate(bot.id)}
                            >
                              Остановить
                            </Button>
                          ) : (
                            <Button
                              variant="accent"
                              size="sm"
                              loading={startBot.isPending && startBot.variables === bot.id}
                              onClick={() => startBot.mutate(bot.id)}
                            >
                              Запустить
                            </Button>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )}
          </div>
        </Card>
      </div>
    </>
  )
}
