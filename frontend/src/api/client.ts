import axios from 'axios';
import { config } from '../config';

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
  llm_api_key: string;
  twitch_client_id: string;
  twitch_client_secret: string;
  updated_at: string;
}

export interface LLMStatus {
  provider: string;
  available: boolean;
  error?: string;
  model: string;
  url: string;
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

export interface SettingsLogEntry {
  id?: number;
  level: string;
  message: string;
  changes?: Record<string, any>;
  created_at: string;
}

const client = axios.create({
  baseURL: config.apiUrl,
  headers: {
    'Content-Type': 'application/json',
    'X-API-Key': config.apiKey,
  },
});

// Логирование запросов
client.interceptors.request.use(
  (config) => {
    console.log(`🌐 [API REQUEST] ${config.method?.toUpperCase()} ${config.url}`, {
      data: config.data,
      headers: config.headers,
    });
    return config;
  },
  (error) => {
    console.error('❌ [API REQUEST ERROR]', error);
    return Promise.reject(error);
  }
);

// Логирование ответов
client.interceptors.response.use(
  (response) => {
    console.log(`✅ [API RESPONSE] ${response.config.method?.toUpperCase()} ${response.config.url}`, {
      status: response.status,
      data: response.data,
    });
    return response;
  },
  (error) => {
    console.error(`❌ [API ERROR] ${error.config?.method?.toUpperCase()} ${error.config?.url}`, {
      status: error.response?.status,
      data: error.response?.data,
      message: error.message,
    });
    return Promise.reject(error);
  }
);

export const api = {
  // Bots
  getBots: () => client.get<Bot[]>('/api/bots'),
  getBot: (id: string) => client.get<Bot>(`/api/bots/${id}`),
  createBot: (data: CreateBotRequest) => client.post('/api/bots', data),
  updateBot: (id: string, data: Partial<Bot>) => client.put(`/api/bots/${id}`, data),
  deleteBot: (id: string) => client.delete(`/api/bots/${id}`),
  startBot: (id: string) => client.post(`/api/bots/${id}/start`),
  stopBot: (id: string) => client.post(`/api/bots/${id}/stop`),
  startAll: () => client.post('/api/bots/start-all'),
  stopAll: () => client.post('/api/bots/stop-all'),

  // Proxies
  getProxies: () => client.get<Proxy[]>('/api/proxies'),
  createProxy: (data: CreateProxyRequest) => client.post('/api/proxies', data),
  deleteProxy: (id: string) => client.delete(`/api/proxies/${id}`),

  // Stats
  getStats: () => client.get<Stats>('/api/stats'),

  // Settings
  getSettings: () => client.get<Settings>('/api/settings'),
  updateSettings: (data: Settings) => client.post('/api/settings', data),

  // LLM Status
  getLLMStatus: () => client.get<LLMStatus>('/api/llm/status'),

  // Bot Logs
  getBotLogs: (botId: string, limit: number = 100) =>
    client.get<BotLogEntry[]>(`/api/bots/${botId}/logs?limit=${limit}`),

  // Settings Logs
  getSettingsLogs: (limit: number = 50) =>
    client.get<SettingsLogEntry[]>(`/api/settings/logs?limit=${limit}`),
};
