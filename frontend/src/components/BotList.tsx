import type { Bot } from '../api/client'
import { BotCard } from './BotCard'

interface BotListProps {
  bots: Bot[]
  onStart: (id: string) => void
  onStop: (id: string) => void
  onDelete: (id: string) => void
  onStartAll: () => void
  onStopAll: () => void
}

export const BotList = ({ bots, onStart, onStop, onDelete, onStartAll, onStopAll }: BotListProps) => {
  return (
    <div className="card">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <h2>Активные боты ({bots.length})</h2>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button onClick={onStartAll} className="btn primary">
            Запустить всех
          </button>
          <button onClick={onStopAll} className="btn danger">
            Остановить всех
          </button>
        </div>
      </div>
      {bots.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>
          Нет ботов. Добавьте ботов в разделе "Боты".
        </div>
      ) : (
        <div className="bot-list">
          {bots.map((bot) => (
            <BotCard
              key={bot.id}
              bot={bot}
              onStart={onStart}
              onStop={onStop}
              onDelete={onDelete}
            />
          ))}
        </div>
      )}
    </div>
  )
}
