import type { Bot } from '../api/client';
import { BotCard } from './BotCard';

interface BotListProps {
  bots: Bot[];
  onStart: (id: string) => void;
  onStop: (id: string) => void;
  onDelete: (id: string) => void;
  onStartAll: () => void;
  onStopAll: () => void;
}

export const BotList = ({ bots, onStart, onStop, onDelete, onStartAll, onStopAll }: BotListProps) => {
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
        <h2 style={{ margin: 0 }}>Bots ({bots.length})</h2>
        <div style={{ display: 'flex', gap: '8px' }}>
          <button
            onClick={onStartAll}
            style={{
              padding: '10px 20px',
              backgroundColor: '#4caf50',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontWeight: 'bold',
            }}
          >
            Start All
          </button>
          <button
            onClick={onStopAll}
            style={{
              padding: '10px 20px',
              backgroundColor: '#f44336',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontWeight: 'bold',
            }}
          >
            Stop All
          </button>
        </div>
      </div>
      {bots.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '40px', color: '#999' }}>
          No bots found. Add bots to the database first.
        </div>
      ) : (
        bots.map((bot) => (
          <BotCard
            key={bot.id}
            bot={bot}
            onStart={onStart}
            onStop={onStop}
            onDelete={onDelete}
          />
        ))
      )}
    </div>
  );
};
