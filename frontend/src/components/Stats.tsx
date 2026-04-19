import type { Stats } from '../api/client';

interface StatsProps {
  stats: Stats | null;
  wsConnected: boolean;
}

export const StatsComponent = ({ stats, wsConnected }: StatsProps) => {
  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
      gap: '16px',
      marginBottom: '24px',
    }}>
      <div style={{
        padding: '16px',
        backgroundColor: '#f5f5f5',
        borderRadius: '8px',
      }}>
        <div style={{ fontSize: '14px', color: '#666' }}>Total Bots</div>
        <div style={{ fontSize: '32px', fontWeight: 'bold' }}>{stats?.total_bots || 0}</div>
      </div>
      <div style={{
        padding: '16px',
        backgroundColor: '#e8f5e9',
        borderRadius: '8px',
      }}>
        <div style={{ fontSize: '14px', color: '#666' }}>Running Bots</div>
        <div style={{ fontSize: '32px', fontWeight: 'bold', color: '#4caf50' }}>
          {stats?.running_bots || 0}
        </div>
      </div>
      <div style={{
        padding: '16px',
        backgroundColor: '#e3f2fd',
        borderRadius: '8px',
      }}>
        <div style={{ fontSize: '14px', color: '#666' }}>Messages/Min</div>
        <div style={{ fontSize: '32px', fontWeight: 'bold', color: '#2196f3' }}>
          {stats?.messages_per_min || 0}
        </div>
      </div>
      <div style={{
        padding: '16px',
        backgroundColor: wsConnected ? '#e8f5e9' : '#ffebee',
        borderRadius: '8px',
      }}>
        <div style={{ fontSize: '14px', color: '#666' }}>WebSocket</div>
        <div style={{
          fontSize: '24px',
          fontWeight: 'bold',
          color: wsConnected ? '#4caf50' : '#f44336'
        }}>
          {wsConnected ? '● Connected' : '● Disconnected'}
        </div>
      </div>
    </div>
  );
};
