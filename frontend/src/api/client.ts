import axios from 'axios';
import { config } from '../config';

export type BotPresenceState = 'online' | 'cooldown' | 'stopped' | 'error';

export interface Bot {
  id: string;
  twitch_login: string;
  name: string;
  personality: string;
  min_delay_sec: number;
  max_delay_sec: number;
  rate_limit_per_min: number;
  enabled: boolean;
  status: string;
  proxy_id?: string;
  auto_rotate_proxy: boolean;
  presence_state: BotPresenceState;
  session_ends_at?: string;
  cooldown_until?: string;
}

export interface CreateBotRequest {
  id: string;
  twitch_login: string;
  password: string;
  name: string;
  personality: string;
  min_delay_sec: number;
  max_delay_sec: number;
  rate_limit_per_min: number;
}

export interface Proxy {
  id: string;
  host: string;
  port: number;
  username?: string;
  protocol: string;
  enabled: boolean;
  last_used_at?: string;
  fail_count: number;
  created_at: string;
}

export interface CreateProxyRequest {
  id: string;
  host: string;
  port: number;
  username?: string;
  password?: string;
  protocol: string;
}

export interface Stats {
  total_bots: number;
  running_bots: number;
  messages_per_min: number;
  total_proxies: number;
  enabled_proxies: number;
}

export interface Settings {
  id: string;
  channels: string[];
  debug_mode: boolean;
  debug_channel: string;
  positive_prompt: string;
  negative_prompt: string;
  max_tokens: number;
  audio_enabled: boolean;
  rotation_enabled: boolean;
  rotation_interval: number;
  llm_provider: string;
  llm_api_url: string;
  llm_model: string;
  llm_paid_model: string;
  llm_paid_enabled: boolean;
  llm_paid_daily_cap_usd: number;
  llm_paid_enabled_effective: boolean;
  llm_paid_daily_cap_usd_effective: number;
  llm_api_key: string;
  llm_api_key_set: boolean;
  llm_api_key_source: 'env' | 'database' | 'none';
  twitch_client_id: string;
  twitch_client_secret: string;
  twitch_client_secret_set: boolean;
  presence_enabled: boolean;
  presence_session_min_minutes: number;
  presence_session_max_minutes: number;
  presence_cooldown_min_minutes: number;
  presence_cooldown_max_minutes: number;
  presence_greeting_enabled: boolean;
  presence_farewell_enabled: boolean;
  updated_at: string;
}

export interface LLMStatus {
  provider: string;
  available: boolean;
  generation_available: boolean;
  error?: string;
  model: string;
  free_model: string;
  paid_model: string;
  url: string;
  paid_enabled: boolean;
  paid_daily_cap_usd: number;
  paid_spent_today_usd: number;
  free_requests_limit: number;
  free_requests_remaining: number;
  free_requests_used: number;
  free_cooldown_until: string;
  inflight_paid_requests: number;
  last_error?: string;
}

export interface BotLogEntry {
  id?: number;
  bot_id: string;
  log_type: string;
  level: string;
  message: string;
  context?: Record<string, any>;
  created_at: string;
}

export interface Chatter {
  login: string;
  last_message_at: string;
}

export interface ChatMessage {
  login: string;
  text: string;
  at: string;
  is_bot: boolean;
}

export interface StreamStatus {
  configured: boolean;
  channel: string;
  live: boolean;
  title: string;
  game: string;
  started_at?: string;
  uptime_seconds: number;
  viewers_total: number;
  active_viewers: number;
  passive_viewers: number;
  active_window_seconds: number;
  chat_connected: boolean;
  chatters: Chatter[];
  messages: ChatMessage[];
  updated_at: string;
  error?: string;
}

const client = axios.create({
  baseURL: config.apiUrl,
  headers: {
    'Content-Type': 'application/json',
    'X-API-Key': config.apiKey,
  },
});

// Converts any thrown error into a user-facing Russian message.
// Never exposes response bodies, headers or stack traces: they may
// contain OAuth tokens and other secrets.
export function getErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    if (!error.response) {
      return 'Ошибка сети: не удалось подключиться к серверу';
    }
    switch (error.response.status) {
      case 401:
        return 'Ошибка авторизации: неверный или отсутствующий API-ключ';
      case 403:
        return 'Доступ запрещён';
      case 404:
        return 'Ресурс не найден';
      case 500:
        return 'Внутренняя ошибка сервера';
      default:
        return `Ошибка запроса (код ${error.response.status})`;
    }
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return 'Неизвестная ошибка';
}

export const api = {
  // Bots
  getBots: () => client.get<Bot[]>('/api/bots'),
  getBot: (id: string) => client.get<Bot>(`/api/bots/${id}`),
  createBot: (data: CreateBotRequest) => client.post('/api/bots', data),
  updateBot: (id: string, data: Partial<Bot>) => client.put(`/api/bots/${id}`, data),
  deleteBot: (id: string) => client.delete(`/api/bots/${id}`),
  startBot: (id: string, disableLlm?: boolean) => {
    const params = disableLlm ? '?disable_llm=true' : ''
    return client.post(`/api/bots/${id}/start${params}`)
  },
  stopBot: (id: string) => client.post(`/api/bots/${id}/stop`),
  startAll: () => client.post('/api/bots/start-all'),
  stopAll: () => client.post('/api/bots/stop-all'),

  // Proxies
  getProxies: () => client.get<Proxy[]>('/api/proxies'),
  createProxy: (data: CreateProxyRequest) => client.post('/api/proxies', data),
  deleteProxy: (id: string) => client.delete(`/api/proxies/${id}`),

  // Stats
  getStats: () => client.get<Stats>('/api/stats'),

  // Twitch stream status
  getStreamStatus: (channel?: string) =>
    client.get<StreamStatus>('/api/twitch/stream', { params: channel ? { channel } : {} }),

  // Settings
  getSettings: () => client.get<Settings>('/api/settings'),
  updateSettings: (data: Settings) => client.post('/api/settings', data),

  // LLM Status
  getLLMStatus: () => client.get<LLMStatus>('/api/llm/status'),

  // Bot Logs
  getBotLogs: (botId: string, limit: number = 100) =>
    client.get<BotLogEntry[]>(`/api/bots/${botId}/logs?limit=${limit}`),
};
