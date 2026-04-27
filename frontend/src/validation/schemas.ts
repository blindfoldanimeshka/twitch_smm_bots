import { z } from 'zod';

// Схема валидации для создания бота
export const createBotSchema = z.object({
  id: z.string()
    .min(1, 'ID is required')
    .max(50, 'ID must be less than 50 characters')
    .regex(/^[a-zA-Z0-9_-]+$/, 'ID can only contain letters, numbers, underscores and hyphens'),

  twitch_login: z.string()
    .min(4, 'Twitch login must be at least 4 characters')
    .max(25, 'Twitch login must be less than 25 characters')
    .regex(/^[a-zA-Z0-9_]+$/, 'Twitch login can only contain letters, numbers and underscores'),

  password: z.string()
    .min(3, 'Password must be at least 3 characters')
    .max(100, 'Password is too long'),

  name: z.string()
    .min(1, 'Name is required')
    .max(100, 'Name must be less than 100 characters'),

  personality: z.enum(['веселый_фанат', 'токсик', 'нейтрал', 'хайпер'], {
    message: 'Invalid personality type',
  }),

  min_delay_sec: z.number()
    .int('Must be an integer')
    .min(1, 'Minimum delay must be at least 1 second')
    .max(3600, 'Minimum delay must be less than 1 hour'),

  max_delay_sec: z.number()
    .int('Must be an integer')
    .min(1, 'Maximum delay must be at least 1 second')
    .max(3600, 'Maximum delay must be less than 1 hour'),

  rate_limit_per_min: z.number()
    .int('Must be an integer')
    .min(1, 'Rate limit must be at least 1')
    .max(60, 'Rate limit must be less than 60'),
}).refine((data) => data.max_delay_sec >= data.min_delay_sec, {
  message: 'Maximum delay must be greater than or equal to minimum delay',
  path: ['max_delay_sec'],
});

// Схема валидации для обновления бота
export const updateBotSchema = z.object({
  name: z.string()
    .min(1, 'Name is required')
    .max(100, 'Name must be less than 100 characters')
    .optional(),

  personality: z.enum(['веселый_фанат', 'токсик', 'нейтрал', 'хайпер'])
    .optional(),

  min_delay_sec: z.number()
    .int('Must be an integer')
    .min(1, 'Minimum delay must be at least 1 second')
    .max(3600, 'Minimum delay must be less than 1 hour')
    .optional(),

  max_delay_sec: z.number()
    .int('Must be an integer')
    .min(1, 'Maximum delay must be at least 1 second')
    .max(3600, 'Maximum delay must be less than 1 hour')
    .optional(),

  rate_limit_per_min: z.number()
    .int('Must be an integer')
    .min(1, 'Rate limit must be at least 1')
    .max(60, 'Rate limit must be less than 60')
    .optional(),
});

// Схема валидации для создания прокси
export const createProxySchema = z.object({
  id: z.string()
    .min(1, 'ID is required')
    .max(50, 'ID must be less than 50 characters')
    .regex(/^[a-zA-Z0-9_-]+$/, 'ID can only contain letters, numbers, underscores and hyphens'),

  host: z.string()
    .min(1, 'Host is required')
    .max(255, 'Host is too long')
    .refine((val) => {
      // Проверка IP адреса или домена
      const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/;
      const domainRegex = /^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]?(\.[a-zA-Z]{2,})+$/;
      return ipRegex.test(val) || domainRegex.test(val);
    }, 'Invalid host format (must be IP address or domain)'),

  port: z.number()
    .int('Port must be an integer')
    .min(1, 'Port must be at least 1')
    .max(65535, 'Port must be less than 65536'),

  username: z.string()
    .max(100, 'Username is too long')
    .optional(),

  password: z.string()
    .max(100, 'Password is too long')
    .optional(),

  protocol: z.enum(['http', 'https', 'socks5'], {
    message: 'Protocol must be http, https or socks5',
  }),
});

// Схема валидации для настроек
export const settingsSchema = z.object({
  channels: z.array(z.string().min(1, 'Channel name cannot be empty'))
    .min(1, 'At least one channel is required'),

  debug_mode: z.boolean(),

  debug_channel: z.string()
    .max(100, 'Debug channel name is too long'),

  positive_prompt: z.string()
    .max(1000, 'Positive prompt is too long'),

  negative_prompt: z.string()
    .max(1000, 'Negative prompt is too long'),

  max_tokens: z.number()
    .int('Must be an integer')
    .min(10, 'Max tokens must be at least 10')
    .max(500, 'Max tokens must be less than 500'),

  audio_enabled: z.boolean(),

  rotation_enabled: z.boolean(),

  rotation_interval: z.number()
    .int('Must be an integer')
    .min(1, 'Rotation interval must be at least 1 minute')
    .max(1440, 'Rotation interval must be less than 24 hours'),
});

export type CreateBotInput = z.infer<typeof createBotSchema>;
export type UpdateBotInput = z.infer<typeof updateBotSchema>;
export type CreateProxyInput = z.infer<typeof createProxySchema>;
export type SettingsInput = z.infer<typeof settingsSchema>;
