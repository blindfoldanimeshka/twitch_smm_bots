import { useEffect, useMemo, useRef } from 'react'
import clsx from 'clsx'
import type { ChatMessage } from '../api/client'

/** One presence event rendered as a centered gray system line. */
export interface ChatPresenceEvent {
  login: string
  /** 'online' means joined; every other state reads as left. */
  state: string
  at: string
}

interface ChatFeedProps {
  messages: ChatMessage[]
  events?: ChatPresenceEvent[]
  /** Visible height of the feed in pixels. */
  height?: number
  className?: string
}

interface FeedItem {
  key: string
  at: number
  message?: ChatMessage
  event?: ChatPresenceEvent
}

const MESSAGE_CAP = 50
const EVENT_CAP = 50
/** How close to the bottom (px) the user must be to count as "at the newest". */
const STICK_THRESHOLD_PX = 48

function timestamp(value: string): number {
  const parsed = Date.parse(value)
  return Number.isNaN(parsed) ? 0 : parsed
}

function formatTime(iso: string): string {
  const parsed = timestamp(iso)
  if (!parsed) {
    return ''
  }
  return new Date(parsed).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

function MessageRow({ message }: { message: ChatMessage }) {
  const isBot = message.is_bot
  const time = formatTime(message.at)
  return (
    <div
      className={clsx('chat-message', isBot && 'chat-message--bot')}
      style={
        isBot
          ? {
              // Lime tint with a lime left border for our own bots.
              background: 'var(--accent-soft)',
              borderColor: 'rgba(181, 224, 60, 0.55)',
              borderLeft: '4px solid var(--accent-strong)',
            }
          : undefined
      }
    >
      <span className="chat-message__meta">
        <span className="chat-message__author" style={isBot ? { fontWeight: 700 } : undefined}>
          @{message.login}
        </span>
        {time && <time dateTime={message.at}>{time}</time>}
      </span>
      <span className="chat-message__text">{message.text}</span>
    </div>
  )
}

function PresenceRow({ event }: { event: ChatPresenceEvent }) {
  const joined = event.state === 'online'
  return (
    <div style={{ textAlign: 'center', fontSize: 12, color: 'var(--muted)', padding: '2px 12px' }}>
      @{event.login} {joined ? 'зашёл' : 'вышел'}
    </div>
  )
}

/**
 * Chat timeline: poll-delivered messages plus WebSocket presence events merged
 * chronologically. Bot messages are lime-tinted with a bold @login, regular
 * messages are plain, presence transitions are centered system lines.
 * Auto-scrolls to the newest item unless the user scrolled up.
 */
export function ChatFeed({ messages, events = [], height = 440, className }: ChatFeedProps) {
  const listRef = useRef<HTMLDivElement>(null)
  const stickToBottomRef = useRef(true)

  const items = useMemo<FeedItem[]>(
    () => [
      // Backend timestamps are nanosecond-precision, so `at`+`login` is a
      // stable identity across poll refreshes (no remount churn).
      ...messages.slice(-MESSAGE_CAP).map((message) => ({
        key: `m-${message.at}-${message.login}`,
        at: timestamp(message.at),
        message,
      })),
      ...events.slice(-EVENT_CAP).map((event) => ({
        key: `e-${event.at}-${event.login}`,
        at: timestamp(event.at),
        event,
      })),
    ].sort((a, b) => a.at - b.at),
    [messages, events],
  )

  const handleScroll = (): void => {
    const el = listRef.current
    if (!el) {
      return
    }
    const distance = el.scrollHeight - el.scrollTop - el.clientHeight
    stickToBottomRef.current = distance < STICK_THRESHOLD_PX
  }

  useEffect(() => {
    const el = listRef.current
    if (el && stickToBottomRef.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [items])

  return (
    <div ref={listRef} onScroll={handleScroll} className={clsx('chat-feed', className)} style={{ height }}>
      {items.length === 0 && (
        <p style={{ margin: 0, textAlign: 'center', fontSize: 13, color: 'var(--muted)' }}>
          Сообщений пока нет — лента наполнится, когда в чате появятся сообщения
        </p>
      )}
      {items.map((item) =>
        item.message ? (
          <MessageRow key={item.key} message={item.message} />
        ) : item.event ? (
          <PresenceRow key={item.key} event={item.event} />
        ) : null,
      )}
    </div>
  )
}
