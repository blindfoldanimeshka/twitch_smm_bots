import { useEffect, useState } from 'react';
import { api, type LLMStatus } from '../api/client';

interface StatusBarProps {
  onNavigateToSettings?: () => void;
}

export function StatusBar({ onNavigateToSettings }: StatusBarProps) {
  const [status, setStatus] = useState<LLMStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);

  const fetchStatus = async () => {
    try {
      const response = await api.getLLMStatus();
      setStatus(response.data);
      setLastChecked(new Date());
    } catch (error) {
      console.error('Failed to fetch LLM status:', error);
      setStatus(null);
      setLastChecked(new Date());
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 30000);
    return () => clearInterval(interval);
  }, []);

  const formatTime = (date: Date | null) => {
    if (!date) return '';
    return date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  if (loading) {
    return (
      <div style={{
        background: 'var(--bg-surface)',
        borderBottom: '2px solid var(--border-default)',
        padding: '0.75rem 1rem',
        fontSize: '0.75rem',
        fontFamily: 'var(--font-display)'
      }}>
        <div style={{ maxWidth: '1200px', margin: '0 auto', display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <div style={{
            width: '8px',
            height: '8px',
            borderRadius: '50%',
            background: 'var(--text-muted)',
            animation: 'pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite'
          }} />
          <span style={{ color: 'var(--text-secondary)', fontWeight: '600', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
            ПРОВЕРКА СТАТУСА LLM...
          </span>
        </div>
      </div>
    );
  }

  if (!status) {
    return (
      <div style={{
        background: 'rgba(255,77,109,0.08)',
        borderBottom: '2px solid var(--state-error)',
        padding: '0.75rem 1rem',
        fontSize: '0.75rem',
        fontFamily: 'var(--font-display)'
      }}>
        <div style={{ maxWidth: '1200px', margin: '0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <div style={{
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              background: 'var(--state-error)',
              boxShadow: '0 0 8px rgba(255,77,109,0.5)'
            }} />
            <span style={{ color: 'var(--state-error)', fontWeight: '700', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
              LLM НЕДОСТУПЕН
            </span>
            <span style={{ color: 'var(--text-secondary)', fontSize: '0.7rem', fontFamily: 'var(--font-mono)' }}>
              • НЕ УДАЛОСЬ ПОДКЛЮЧИТЬСЯ
            </span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            {lastChecked && (
              <span style={{ color: 'var(--text-muted)', fontSize: '0.7rem', fontFamily: 'var(--font-mono)' }}>
                {formatTime(lastChecked)}
              </span>
            )}
            {onNavigateToSettings && (
              <button
                onClick={onNavigateToSettings}
                className="btn danger"
                style={{
                  padding: '0.375rem 0.875rem',
                  fontSize: '0.7rem'
                }}
              >
                НАСТРОИТЬ →
              </button>
            )}
          </div>
        </div>
      </div>
    );
  }

  if (!status.available) {
    const errorShort = status.error?.includes('401') ? 'НЕВЕРНЫЙ API КЛЮЧ' :
                       status.error?.includes('connection refused') ? 'СЕРВЕР НЕДОСТУПЕН' :
                       status.error?.includes('timeout') ? 'TIMEOUT' :
                       'ОШИБКА ПОДКЛЮЧЕНИЯ';

    return (
      <div style={{
        background: 'rgba(255,210,77,0.08)',
        borderBottom: '2px solid var(--state-warning)',
        padding: '0.75rem 1rem',
        fontSize: '0.75rem',
        fontFamily: 'var(--font-display)'
      }}>
        <div style={{ maxWidth: '1200px', margin: '0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <div style={{
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              background: 'var(--state-warning)',
              animation: 'pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite',
              boxShadow: '0 0 8px rgba(255,210,77,0.5)'
            }} />
            <span style={{ color: 'var(--state-warning)', fontWeight: '700', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
              {status.provider.toUpperCase()} НЕДОСТУПЕН
            </span>
            <span style={{ color: 'var(--text-secondary)', fontSize: '0.7rem', fontFamily: 'var(--font-mono)' }}>
              • {errorShort}
            </span>
            {status.model && (
              <span style={{ color: 'var(--text-muted)', fontSize: '0.7rem', fontFamily: 'var(--font-mono)' }}>
                • {status.model}
              </span>
            )}
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            {lastChecked && (
              <span style={{ color: 'var(--text-muted)', fontSize: '0.7rem', fontFamily: 'var(--font-mono)' }}>
                {formatTime(lastChecked)}
              </span>
            )}
            {onNavigateToSettings && (
              <button
                onClick={onNavigateToSettings}
                style={{
                  background: 'var(--state-warning)',
                  color: 'var(--text-inverse)',
                  padding: '0.375rem 0.875rem',
                  borderRadius: 'var(--radius-sm)',
                  border: 'none',
                  fontSize: '0.7rem',
                  fontWeight: '700',
                  cursor: 'pointer',
                  transition: 'all 0.2s',
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                  fontFamily: 'var(--font-display)'
                }}
              >
                НАСТРОИТЬ →
              </button>
            )}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div style={{
      background: 'rgba(57,255,106,0.08)',
      borderBottom: '2px solid var(--state-success)',
      padding: '0.75rem 1rem',
      fontSize: '0.75rem',
      fontFamily: 'var(--font-display)',
      boxShadow: '0 2px 8px rgba(57,255,106,0.1)'
    }}>
      <div style={{ maxWidth: '1200px', margin: '0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <div style={{
            width: '8px',
            height: '8px',
            borderRadius: '50%',
            background: 'var(--state-success)',
            boxShadow: 'var(--neon-glow-soft)'
          }} />
          <span style={{ color: 'var(--primary-500)', fontWeight: '700', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
            {status.provider.toUpperCase()} ПОДКЛЮЧЕН
          </span>
          {status.model && (
            <span style={{ color: 'var(--accent-lime)', fontSize: '0.7rem', fontFamily: 'var(--font-mono)' }}>
              • {status.model}
            </span>
          )}
          <span style={{ color: 'var(--text-muted)', fontSize: '0.7rem', fontFamily: 'var(--font-mono)' }}>
            • {status.url}
          </span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          {lastChecked && (
            <span style={{ color: 'var(--text-muted)', fontSize: '0.7rem', fontFamily: 'var(--font-mono)' }}>
              {formatTime(lastChecked)}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
