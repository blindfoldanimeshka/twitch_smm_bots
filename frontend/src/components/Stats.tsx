import type { Stats } from '../api/client'

interface StatsProps {
  stats: Stats | null
  wsConnected: boolean
}

export const StatsComponent = ({ stats, wsConnected }: StatsProps) => {
  return (
    <div className="stats-grid">
      <div className="stat-card">
        <div className="stat-card__label">Всего ботов</div>
        <div className="stat-card__value">{stats?.total_bots || 0}</div>
      </div>
      <div className="stat-card">
        <div className="stat-card__label">Работает</div>
        <div className="stat-card__value" style={{ color: 'var(--color-success)' }}>
          {stats?.running_bots || 0}
        </div>
      </div>
      <div className="stat-card">
        <div className="stat-card__label">Сообщений/мин</div>
        <div className="stat-card__value" style={{ color: 'var(--color-primary)' }}>
          {stats?.messages_per_min || 0}
        </div>
      </div>
      <div className="stat-card">
        <div className="stat-card__label">WebSocket</div>
        <div className={`ws-status ${wsConnected ? 'ws-status--connected' : ''}`}>
          <span className="ws-status__dot"></span>
          <span>{wsConnected ? 'Подключен' : 'Отключен'}</span>
        </div>
      </div>
    </div>
  )
}
