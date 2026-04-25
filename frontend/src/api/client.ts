import axios, { AxiosError } from 'axios';
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
  updated_at: string;
}

export interface ApiError {
  message: string;
  status?: number;
  code?: string;
}

const client = axios.create({
  baseURL: config.apiUrl,
  headers: {
    'Content-Type': 'application/json',
    'X-API-Key': config.apiKey,
  },
  timeout: 30000,
});

// Interceptor для обработки ошибок
client.interceptors.response.use(
  (response) => response,
  (error: AxiosError<{ error?: string }>) => {
    const apiError: ApiError = {
      message: 'Unknown error occurred',
      status: error.response?.status,
    };

    if (error.code === 'ECONNABORTED') {
      apiError.message = 'Request timeout. Please try again.';
      apiError.code = 'TIMEOUT';
    } else if (error.code === 'ERR_NETWORK') {
      apiError.message = 'Network error. Please check your connection.';
      apiError.code = 'NETWORK_ERROR';
    } else if (error.response) {
      // Сервер ответил с ошибкой
      apiError.message = error.response.data?.error || `HTTP ${error.response.status}: ${error.response.statusText}`;

      switch (error.response.status) {
        case 401:
          apiError.message = 'Unauthorized. Please check your API key.';
          apiError.code = 'UNAUTHORIZED';
          break;
        case 403:
          apiError.message = 'Access forbidden.';
          apiError.code = 'FORBIDDEN';
          break;
        case 404:
          apiError.message = 'Resource not found.';
          apiError.code = 'NOT_FOUND';
          break;
        case 409:
          apiError.message = error.response.data?.error || 'Conflict. Resource already exists.';
          apiError.code = 'CONFLICT';
          break;
        case 429:
          apiError.message = 'Too many requests. Please slow down.';
          apiError.code = 'RATE_LIMIT';
          break;
        case 500:
          apiError.message = 'Server error. Please try again later.';
          apiError.code = 'SERVER_ERROR';
          break;
        case 503:
          apiError.message = 'Service unavailable. Please try again later.';
          apiError.code = 'SERVICE_UNAVAILABLE';
          break;
      }
    } else if (error.request) {
      // Запрос был отправлен, но ответа не получено
      apiError.message = 'No response from server. Please check your connection.';
      apiError.code = 'NO_RESPONSE';
    }

    return Promise.reject(apiError);
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
};
