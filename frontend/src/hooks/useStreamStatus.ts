import { useQuery } from '@tanstack/react-query';
import { api, type StreamStatus } from '../api/client';

/**
 * Polls the public status of one monitored Twitch channel.
 * Pass `enabled: false` when no channel is selected.
 */
export const useStreamStatus = (channel: string | undefined, enabled: boolean) =>
  useQuery<StreamStatus>({
    queryKey: ['twitch-stream', channel ?? ''],
    queryFn: async () => (await api.getStreamStatus(channel)).data,
    enabled,
    refetchInterval: 15_000,
    retry: 1,
  });
