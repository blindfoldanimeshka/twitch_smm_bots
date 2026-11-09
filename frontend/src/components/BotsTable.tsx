import type { ReactNode } from 'react'
import { Bot as BotIcon, Pencil, ScrollText, Trash2 } from 'lucide-react'
import type { Bot } from '../api/client'
import { Badge, type BadgeTone } from './ui/Badge'
import { Button } from './ui/Button'
import { EmptyState } from './ui/EmptyState'

interface BotsTableProps {
  bots: Bot[]
  /** IDs of bots with a start/stop action in flight; each row's spinner is independent. */
  pendingIds?: ReadonlySet<string>
  onStart: (bot: Bot) => void
  onStop: (bot: Bot) => void
  onEdit: (bot: Bot) => void
  onDelete: (bot: Bot) => void
  onShowLogs: (bot: Bot) => void
  emptyAction?: ReactNode
}

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

/** The bots table: login, name, personality, presence, delays, limit, actions. */
export function BotsTable({
  bots,
  pendingIds,
  onStart,
  onStop,
  onEdit,
  onDelete,
  onShowLogs,
  emptyAction,
}: BotsTableProps) {
  if (bots.length === 0) {
    return (
      <EmptyState
        icon={BotIcon}
        title="Пока нет ботов"
        description="Добавьте первого бота — он появится в этой таблице."
        action={emptyAction}
      />
    )
  }

  return (
    <div style={{ overflowX: 'auto' }}>
      <table className="table">
        <thead>
          <tr>
            <th>Логин</th>
            <th>Имя</th>
            <th>Личность</th>
            <th>Присутствие</th>
            <th>Задержки</th>
            <th>Лимит</th>
            <th style={{ textAlign: 'right' }}>Действия</th>
          </tr>
        </thead>
        <tbody>
          {bots.map((bot) => {
            const badge = presenceBadge(bot)
            const running = isRunning(bot)
            return (
              <tr key={bot.id}>
                <td>@{bot.twitch_login}</td>
                <td>{bot.name}</td>
                <td>{bot.personality}</td>
                <td>
                  <Badge tone={badge.tone}>{badge.label}</Badge>
                </td>
                <td>
                  {bot.min_delay_sec}–{bot.max_delay_sec} сек
                </td>
                <td>{bot.rate_limit_per_min}/мин</td>
                <td>
                  <div style={{ display: 'flex', gap: 4, justifyContent: 'flex-end' }}>
                    {running ? (
                      <Button
                        variant="secondary"
                        size="sm"
                        loading={pendingIds?.has(bot.id) ?? false}
                        onClick={() => onStop(bot)}
                      >
                        Стоп
                      </Button>
                    ) : (
                      <Button
                        variant="accent"
                        size="sm"
                        loading={pendingIds?.has(bot.id) ?? false}
                        onClick={() => onStart(bot)}
                      >
                        Старт
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label="Редактировать"
                      title="Редактировать"
                      onClick={() => onEdit(bot)}
                    >
                      <Pencil size={16} aria-hidden="true" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label="Логи"
                      title="Логи"
                      onClick={() => onShowLogs(bot)}
                    >
                      <ScrollText size={16} aria-hidden="true" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label="Удалить"
                      title="Удалить"
                      onClick={() => onDelete(bot)}
                    >
                      <Trash2 size={16} aria-hidden="true" />
                    </Button>
                  </div>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
