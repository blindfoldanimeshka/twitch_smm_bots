import { useEffect, useState, useCallback } from 'react';
import { QueryClient, QueryClientProvider, useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type Stats } from './api/client';
import { useWebSocket, type WSMessage } from './hooks/useWebSocket';
import { StatsComponent } from './components/Stats';
import { BotList } from './components/BotList';

const queryClient = new QueryClient();

function Dashboard() {
  const queryClient = useQueryClient();
  const [stats, setStats] = useState<Stats | null>(null);

  const { data: bots = [], refetch: refetchBots } = useQuery({
    queryKey: ['bots'],
    queryFn: async () => {
      const res = await api.getBots();
      return res.data;
    },
    refetchInterval: 5000,
  });

  const { data: statsData } = useQuery({
    queryKey: ['stats'],
    queryFn: async () => {
      const res = await api.getStats();
      return res.data;
    },
    refetchInterval: 3000,
  });

  useEffect(() => {
    if (statsData) {
      setStats(statsData);
    }
  }, [statsData]);

  const startBotMutation = useMutation({
    mutationFn: (id: string) => api.startBot(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] });
      queryClient.invalidateQueries({ queryKey: ['stats'] });
    },
  });

  const stopBotMutation = useMutation({
    mutationFn: (id: string) => api.stopBot(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] });
      queryClient.invalidateQueries({ queryKey: ['stats'] });
    },
  });

  const deleteBotMutation = useMutation({
    mutationFn: (id: string) => api.deleteBot(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] });
      queryClient.invalidateQueries({ queryKey: ['stats'] });
    },
  });

  const startAllMutation = useMutation({
    mutationFn: () => api.startAll(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] });
      queryClient.invalidateQueries({ queryKey: ['stats'] });
    },
  });

  const stopAllMutation = useMutation({
    mutationFn: () => api.stopAll(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] });
      queryClient.invalidateQueries({ queryKey: ['stats'] });
    },
  });

  const handleWSMessage = useCallback((msg: WSMessage) => {
    console.log('WebSocket message:', msg);

    if (msg.type === 'bot_status_changed') {
      refetchBots();
      queryClient.invalidateQueries({ queryKey: ['stats'] });
    } else if (msg.type === 'bot_message_sent') {
      console.log('Bot message:', msg.payload);
    } else if (msg.type === 'stats_updated') {
      setStats(msg.payload);
    }
  }, [refetchBots, queryClient]);

  const { connected } = useWebSocket(handleWSMessage);

  return (
    <div style={{ maxWidth: '1200px', margin: '0 auto', padding: '20px' }}>
      <h1 style={{ marginBottom: '24px' }}>Twitch Bot Farm - Control Panel</h1>

      <StatsComponent stats={stats} wsConnected={connected} />

      <BotList
        bots={bots}
        onStart={(id) => startBotMutation.mutate(id)}
        onStop={(id) => stopBotMutation.mutate(id)}
        onDelete={(id) => {
          if (confirm(`Delete bot ${id}?`)) {
            deleteBotMutation.mutate(id);
          }
        }}
        onStartAll={() => startAllMutation.mutate()}
        onStopAll={() => stopAllMutation.mutate()}
      />
    </div>
  );
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <Dashboard />
    </QueryClientProvider>
  );
}

export default App;
