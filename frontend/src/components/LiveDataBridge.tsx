import { useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useWebSocketSubscription, type WSMessage } from '../hooks/useWebSocket'

/**
 * The single owner of live invalidation. Mounted once under both
 * `QueryClientProvider` and `WebSocketProvider`, so every page (Dashboard,
 * Bots, Stream, Settings) sees fresh data instead of only the page that used
 * to carry its own subscription.
 */
export function LiveDataBridge() {
  const queryClient = useQueryClient()

  const onMessage = useCallback(
    (msg: WSMessage) => {
      switch (msg.type) {
        case 'bot_status_changed':
          queryClient.invalidateQueries({ queryKey: ['bots'] })
          queryClient.invalidateQueries({ queryKey: ['stats'] })
          break
        case 'stats_updated':
          queryClient.invalidateQueries({ queryKey: ['stats'] })
          break
        case 'bot_presence_changed':
          queryClient.invalidateQueries({ queryKey: ['bots'] })
          break
        case 'bot_message_sent':
          queryClient.invalidateQueries({ queryKey: ['stats'] })
          break
        case 'settings_log':
          queryClient.invalidateQueries({ queryKey: ['settings'] })
          break
      }
    },
    [queryClient],
  )

  useWebSocketSubscription(onMessage)

  return null
}
