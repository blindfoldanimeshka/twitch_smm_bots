import { useState } from 'react'
import type { Bot } from '../api/client'
import InteractiveHoverButton from './ui/interactive-hover-button'
import { BotLogsModal } from './BotLogsModal'
import { LaunchBotModal } from './LaunchBotModal'

interface BotCardProps {
  bot: Bot
  onStart: (id: string) => void
  onStop: (id: string) => void
}

export const BotCard = ({ bot, onStart, onStop }: BotCardProps) => {
  const isRunning = bot.status === 'running'
  const [isLogsModalOpen, setIsLogsModalOpen] = useState(false)
  const [isLaunchModalOpen, setIsLaunchModalOpen] = useState(false)

  return (
    <div className={`bot-card ${isRunning ? 'bot-card--running' : ''}`}>
      <div className="bot-card__header">
        <div>
          <div className="bot-card__title">{bot.name}</div>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
            @{bot.twitch_login}
          </div>
        </div>
        <span className={`bot-card__status bot-card__status--${bot.status}`}>
          {isRunning ? '● РАБОТАЕТ' : '○ ОСТАНОВЛЕН'}
        </span>
      </div>

      <div className="bot-card__info">
        <div>ID: <code>{bot.id}</code></div>
        <div>ЛИЧНОСТЬ: {bot.personality}</div>
        <div>ЗАДЕРЖКА: {bot.min_delay_sec}-{bot.max_delay_sec} сек</div>
        <div>ЛИМИТ: {bot.rate_limit_per_min} сообщ/мин</div>
      </div>

      <div className="bot-card__actions">
        {isRunning ? (
          <InteractiveHoverButton
            text="■ СТОП"
            loadingText="Остановка..."
            successText="Остановлен!"
            onClick={() => onStop(bot.id)}
            classes="flex-1 bg-destructive text-destructive-foreground border-destructive"
          />
        ) : (
          <InteractiveHoverButton
            text="▶ СТАРТ"
            loadingText="Запуск..."
            successText="Запущен!"
            onClick={() => setIsLaunchModalOpen(true)}
            classes="flex-1 bg-primary text-primary-foreground border-primary"
          />
        )}
        <InteractiveHoverButton
          text="📋 ЛОГИ"
          loadingText="Открытие..."
          successText="Открыто!"
          onClick={() => setIsLogsModalOpen(true)}
          classes="flex-1 bg-transparent text-primary border-border hover:bg-accent"
        />
      </div>

      <BotLogsModal
        botId={bot.id}
        botName={bot.name}
        isOpen={isLogsModalOpen}
        onClose={() => setIsLogsModalOpen(false)}
      />

      <LaunchBotModal
        isOpen={isLaunchModalOpen}
        onClose={() => setIsLaunchModalOpen(false)}
        bot={bot}
        onStart={() => {
          setIsLaunchModalOpen(false)
          onStart(bot.id)
        }}
      />
    </div>
  )
}
