import { useQuery } from '@tanstack/react-query';
import { api, type Stats } from '../api/client';

/** Polls aggregate bot/proxy statistics from /api/stats. */
export const useStats = () =>
  useQuery<Stats>({
    queryKey: ['stats'],
    queryFn: async () => (await api.getStats()).data,
  });
