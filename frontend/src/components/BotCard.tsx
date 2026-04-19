import type { Bot } from '../api/client';

interface BotCardProps {
  bot: Bot;
  onStart: (id: string) => void;
  onStop: (id: string) => void;
  onDelete: (id: string) => void;
}

export const BotCard = ({ bot, onStart, onStop, onDelete }: BotCardProps) => {
  const isRunning = bot.status === 'running';

  return (
    <div style={{
      border: '1px solid #ddd',
      borderRadius: '8px',
      padding: '16px',
      marginBottom: '12px',
      backgroundColor: isRunning ? '#e8f5e9' : '#fff',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h3 style={{ margin: '0 0 8px 0' }}>
            {bot.name} ({bot.twitch_login})
          </h3>
          <div style={{ fontSize: '14px', color: '#666' }}>
            <div>ID: {bot.id}</div>
            <div>Personality: {bot.personality}</div>
            <div>Rate: {bot.rate_limit_per_min}/min</div>
            <div>Delay: {bot.min_delay_sec}-{bot.max_delay_sec}s</div>
            <div>Status: <strong style={{ color: isRunning ? 'green' : 'red' }}>{bot.status}</strong></div>
          </div>
        </div>
        <div style={{ display: 'flex', gap: '8px', flexDirection: 'column' }}>
          {isRunning ? (
            <button
              onClick={() => onStop(bot.id)}
              style={{
                padding: '8px 16px',
                backgroundColor: '#f44336',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: 'pointer',
              }}
            >
              Stop
            </button>
          ) : (
            <button
              onClick={() => onStart(bot.id)}
              style={{
                padding: '8px 16px',
                backgroundColor: '#4caf50',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: 'pointer',
              }}
            >
              Start
            </button>
          )}
          <button
            onClick={() => onDelete(bot.id)}
            style={{
              padding: '8px 16px',
              backgroundColor: '#9e9e9e',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
            }}
          >
            Delete
          </button>
        </div>
      </div>
    </div>
  );
};
