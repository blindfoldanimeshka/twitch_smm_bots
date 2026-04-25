import { useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import { useWebSocket, type WSMessage } from '../hooks/useWebSocket'
import { StatsComponent } from '../components/Stats'
import { BotList } from '../components/BotList'

function useBotMutation<T>(
  mutationFn: (arg: T) => Promise<any>
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] })
      queryClient.invalidateQueries({ queryKey: ['stats'] })
    },
  })
}

export default function Dashboard() {
  const queryClient = useQueryClient()

  const { data: bots = [], refetch: refetchBots } = useQuery({
    queryKey: ['bots'],
    queryFn: async () => {
      const res = await api.getBots()
      return res.data
    },
  })

  const { data: stats = null } = useQuery({
    queryKey: ['stats'],
    queryFn: async () => {
      const res = await api.getStats()
      return res.data
    },
  })

  const startBotMutation = useBotMutation((id: string) => api.startBot(id))
  const stopBotMutation = useBotMutation((id: string) => api.stopBot(id))
  const deleteBotMutation = useBotMutation((id: string) => api.deleteBot(id))
  const startAllMutation = useBotMutation(() => api.startAll())
  const stopAllMutation = useBotMutation(() => api.stopAll())

  const handleWSMessage = useCallback((msg: WSMessage) => {
    if (msg.type === 'bot_status_changed' || msg.type === 'stats_updated') {
      refetchBots()
      queryClient.invalidateQueries({ queryKey: ['stats'] })
    }
  }, [refetchBots, queryClient])

  const { connected } = useWebSocket(handleWSMessage)

  return (
    <>
      <header className="hero">
        <h1 className="hero__title">УПРАВЛЕНИЕ БОТАМИ</h1>
        <p className="hero__lead">
          Панель управления ботами для автоматизации чата. Запускайте, останавливайте и настраивайте ботов с AI-генерацией сообщений.
        </p>
      </header>

      <StatsComponent stats={stats} wsConnected={connected} />

      <BotList
        bots={bots}
        onStart={(id) => startBotMutation.mutate(id)}
        onStop={(id) => stopBotMutation.mutate(id)}
        onDelete={(id) => {
          if (confirm(`Удалить бота ${id}?`)) {
            deleteBotMutation.mutate(id)
          }
        }}
        onStartAll={() => startAllMutation.mutate(undefined)}
        onStopAll={() => stopAllMutation.mutate(undefined)}
      />
    </>
  )
}
