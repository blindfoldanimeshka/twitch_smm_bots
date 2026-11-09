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
        case 'bot_log':
          // Handled elsewhere: the bot-log drawer pulls over REST; no query
          // cache here depends on individual log lines. Intentionally ignored.
          break
        case 'audio_context':
          // Handled elsewhere: AudioTestModal renders the transcript feed.
          // Intentionally ignored here.
          break
        default: {
          // Compile-time exhaustiveness: adding a backend event type without a
          // branch here makes this assignment fail.
          const _exhaustive: never = msg
          void _exhaustive
        }
      }
    },
    [queryClient],
  )

  useWebSocketSubscription(onMessage)

  return null
}
