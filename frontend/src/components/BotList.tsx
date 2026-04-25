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
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-6)' }}>
        <h2>АКТИВНЫЕ БОТЫ ({bots.length})</h2>
        <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
          <button onClick={onStartAll} className="btn primary">
            ▶ ЗАПУСТИТЬ ВСЕХ
          </button>
          <button onClick={onStopAll} className="btn danger">
            ■ ОСТАНОВИТЬ ВСЕХ
          </button>
        </div>
      </div>
      {bots.length === 0 ? (
        <div style={{ textAlign: 'center', padding: 'var(--space-12)', color: 'var(--text-muted)' }}>
          <p style={{ fontFamily: 'var(--font-display)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
            НЕТ БОТОВ. ДОБАВЬТЕ БОТОВ В РАЗДЕЛЕ "БОТЫ".
          </p>
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
