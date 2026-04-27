import type { Stats } from '../api/client'
import { TiltCard } from './ui/tilt-card'

interface StatsProps {
  stats: Stats | null
  wsConnected: boolean
}

export const StatsComponent = ({ stats, wsConnected }: StatsProps) => {
  return (
    <div className="stats-grid">
      <TiltCard tiltLimit={10} scale={1.02} spotlight={true} className="cursor-target">
        <div className="stat-card">
          <div className="stat-card__label">ВСЕГО БОТОВ</div>
          <div className="stat-card__value">{stats?.total_bots || 0}</div>
        </div>
      </TiltCard>
      <TiltCard tiltLimit={10} scale={1.02} spotlight={true} className="cursor-target">
        <div className="stat-card">
          <div className="stat-card__label">РАБОТАЕТ</div>
          <div className="stat-card__value" style={{ color: 'var(--state-success)' }}>
            {stats?.running_bots || 0}
          </div>
        </div>
      </TiltCard>
      <TiltCard tiltLimit={10} scale={1.02} spotlight={true} className="cursor-target">
        <div className="stat-card">
          <div className="stat-card__label">СООБЩЕНИЙ/МИН</div>
          <div className="stat-card__value" style={{ color: 'var(--accent-cyan)' }}>
            {stats?.messages_per_min || 0}
          </div>
        </div>
      </TiltCard>
      <TiltCard tiltLimit={10} scale={1.02} spotlight={true} className="cursor-target">
        <div className="stat-card">
          <div className="stat-card__label">WEBSOCKET</div>
          <div className={`ws-status ${wsConnected ? 'ws-status--connected' : ''}`}>
            <span className="ws-status__dot"></span>
            <span>{wsConnected ? 'ПОДКЛЮЧЕН' : 'ОТКЛЮЧЕН'}</span>
          </div>
        </div>
      </TiltCard>
    </div>
  )
}
