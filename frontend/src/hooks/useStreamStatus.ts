import { useQuery } from '@tanstack/react-query';
import { api, type StreamStatus } from '../api/client';

/**
 * Polls the public status of one monitored Twitch channel every 15s.
 * Pass `enabled: false` when no channel is selected.
 *
 * Returns the whole query result on purpose: call sites need `isLoading`,
 * `isError` and `refetch` to render loading, failure and emptiness apart,
 * instead of reading `data === undefined` as "still loading".
 */
export const useStreamStatus = (channel: string | undefined, enabled: boolean) =>
  useQuery<StreamStatus>({
    queryKey: ['twitch-stream', channel ?? ''],
    queryFn: async () => (await api.getStreamStatus(channel)).data,
    enabled,
    refetchInterval: 15_000,
    retry: 1,
  });
