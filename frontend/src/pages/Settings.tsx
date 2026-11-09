import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Mic, Plus, X } from 'lucide-react'
import { api, getErrorMessage } from '../api/client'
import type { Settings as SettingsType } from '../api/client'
import { PageHeader } from '../components/layout/PageHeader'
import { AudioTestModal } from '../components/AudioTestModal'
import { Alert } from '../components/ui/Alert'
import { Badge, type BadgeTone } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Input } from '../components/ui/Input'
import { Switch } from '../components/ui/switch'

const HINT_STYLE = { fontSize: 12, color: 'var(--muted)' } as const
const GRID_STYLE = { display: 'grid', gap: 16 } as const

/**
 * Local mirror of the backend rule: both minute pairs must satisfy
 * 1 <= min <= max <= 240. Returns a Russian message for the first violated
 * rule, or null when the ranges are safe to send.
 */
function getPresenceRangeError(s: SettingsType): string | null {
  const { presence_session_min_minutes: sessionMin, presence_session_max_minutes: sessionMax } = s
  const { presence_cooldown_min_minutes: cooldownMin, presence_cooldown_max_minutes: cooldownMax } = s

  if (sessionMin < 1 || sessionMax < 1 || cooldownMin < 1 || cooldownMax < 1) {
    return 'Минуты сессии и паузы должны быть не меньше 1.'
  }
  if (sessionMin > sessionMax) {
    return 'Минимальная длительность сессии не может быть больше максимальной.'
  }
  if (cooldownMin > cooldownMax) {
    return 'Минимальная пауза не может быть больше максимальной.'
  }
  if (sessionMax > 240 || cooldownMax > 240) {
    return 'Максимальные значения — не больше 240 минут.'
  }
  return null
}

/** Short per-field hint for one min/max minute pair. */
function pairFieldError(min: number, max: number): string | undefined {
  if (min < 1 || max < 1) return 'от 1 минуты'
  if (min > max) return 'минимум ≤ максимум'
  if (max > 240) return 'до 240 минут'
  return undefined
}

/**
 * Server settings → local form shape. Every sync from the cache and every
 * baseline update goes through here, so formState and baseline always share
 * one normalized shape and can be compared field by field.
 */
function normalizeSettings(settings: SettingsType): SettingsType {
  return {
    ...settings,
    channels: settings.channels.length > 0 ? settings.channels : [''],
    debug_mode: settings.debug_mode ?? false,
    debug_channel: settings.debug_channel ?? '',
    positive_prompt: settings.positive_prompt ?? '',
    negative_prompt: settings.negative_prompt ?? '',
    max_tokens: settings.max_tokens ?? 60,
    audio_enabled: settings.audio_enabled ?? false,
    // Rotation has no UI any more: keep the stored values and send them
    // back unchanged (UpdateSettings replaces the whole row).
    rotation_enabled: settings.rotation_enabled ?? false,
    rotation_interval: settings.rotation_interval ?? 60,
    llm_provider: settings.llm_provider ?? 'openrouter',
    llm_api_url: settings.llm_api_url ?? 'https://openrouter.ai/api/v1',
    llm_model: settings.llm_model ?? 'openrouter/free',
    llm_paid_model: settings.llm_paid_model ?? 'z-ai/glm-5.3-flash',
    llm_paid_enabled: settings.llm_paid_enabled ?? true,
    llm_paid_daily_cap_usd: settings.llm_paid_daily_cap_usd ?? 1,
    llm_paid_enabled_effective: settings.llm_paid_enabled_effective ?? settings.llm_paid_enabled ?? true,
    llm_paid_daily_cap_usd_effective: settings.llm_paid_daily_cap_usd_effective ?? settings.llm_paid_daily_cap_usd ?? 1,
    llm_api_key: '',
    llm_api_key_set: settings.llm_api_key_set ?? false,
    llm_api_key_source: settings.llm_api_key_source ?? 'none',
    twitch_client_id: settings.twitch_client_id ?? '',
    twitch_client_secret: settings.twitch_client_secret ?? '',
    twitch_client_secret_set: settings.twitch_client_secret_set ?? false,
    presence_enabled: settings.presence_enabled ?? true,
    presence_session_min_minutes: settings.presence_session_min_minutes ?? 20,
    presence_session_max_minutes: settings.presence_session_max_minutes ?? 40,
    presence_cooldown_min_minutes: settings.presence_cooldown_min_minutes ?? 10,
    presence_cooldown_max_minutes: settings.presence_cooldown_max_minutes ?? 20,
    presence_greeting_enabled: settings.presence_greeting_enabled ?? true,
    presence_farewell_enabled: settings.presence_farewell_enabled ?? true,
    updated_at: settings.updated_at ?? '',
  }
}

/**
 * Deep compare of the form against the last synced/saved baseline.
 * `updated_at` (the server stamps a new one on every save) is excluded;
 * `llm_api_key` is excluded here because it never round-trips through the
 * server — the `dirty` memo counts a typed key separately.
 */
function settingsFormEqual(a: SettingsType, b: SettingsType): boolean {
  for (const key of Object.keys(a) as (keyof SettingsType)[]) {
    if (key === 'updated_at' || key === 'llm_api_key') continue
    const av = a[key]
    const bv = b[key]
    if (Array.isArray(av) && Array.isArray(bv)) {
      if (av.length !== bv.length || av.some((v, i) => v !== bv[i])) return false
    } else if (av !== bv) {
      return false
    }
  }
  return true
}

/**
 * Parse a token-limit keystroke without normalizing it: mid-typing values
 * (like the "5" of "50") must reach the DOM untouched. null = ignore this
 * input (empty or garbage) and keep the previous value.
 */
function parseMaxTokensInput(raw: string): number | null {
  if (raw.trim() === '') return null // empty input: keep the previous value, never send 0
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : null
}

/** Clamp a finalized token limit into the backend-accepted 20..200 range. */
function clampMaxTokensValue(value: number): number {
  return Math.min(200, Math.max(20, Math.round(value)))
}

function SwitchField({
  label,
  checked,
  disabled,
  hint,
  onChange,
}: {
  label: string
  checked: boolean
  disabled?: boolean
  hint?: string
  onChange: (checked: boolean) => void
}) {
  return (
    <div style={{ display: 'grid', gap: 4 }}>
      <label style={{ display: 'flex', alignItems: 'center', gap: 12, cursor: 'pointer' }}>
        <Switch checked={checked} disabled={disabled} onCheckedChange={onChange} />
        <span style={{ fontSize: 14, fontWeight: 500 }}>{label}</span>
      </label>
      {hint && <span style={HINT_STYLE}>{hint}</span>}
    </div>
  )
}

export default function Settings() {
  const queryClient = useQueryClient()

  const [formState, setFormState] = useState<SettingsType>({
    id: 'default',
    channels: [''],
    debug_mode: false,
    debug_channel: '',
    positive_prompt: '',
    negative_prompt: '',
    max_tokens: 60,
    audio_enabled: false,
    rotation_enabled: false,
    rotation_interval: 60,
    llm_provider: 'openrouter',
    llm_api_url: 'https://openrouter.ai/api/v1',
    llm_model: 'openrouter/free',
    llm_paid_model: 'z-ai/glm-5.3-flash',
    llm_paid_enabled: true,
    llm_paid_daily_cap_usd: 1,
    llm_paid_enabled_effective: true,
    llm_paid_daily_cap_usd_effective: 1,
    llm_api_key: '',
    llm_api_key_set: false,
    llm_api_key_source: 'none',
    twitch_client_id: '',
    twitch_client_secret: '',
    twitch_client_secret_set: false,
    // Mirrors the backend column defaults: a transient load failure must never
    // be the reason the form silently flips presence off or edits fake ranges.
    presence_enabled: true,
    presence_session_min_minutes: 20,
    presence_session_max_minutes: 40,
    presence_cooldown_min_minutes: 10,
    presence_cooldown_max_minutes: 20,
    presence_greeting_enabled: true,
    presence_farewell_enabled: true,
    updated_at: '',
  })
  const [audioModalOpen, setAudioModalOpen] = useState(false)
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saveBlocked, setSaveBlocked] = useState<string | null>(null)

  const {
    data: settings,
    isLoading,
    isError: settingsLoadFailed,
    refetch: refetchSettings,
  } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => (await api.getSettings()).data,
  })

  // Shared ['llm-status'] cache (same key as the Topbar and Dashboard).
  const {
    data: llmStatus,
    isFetching: llmChecking,
    isError: llmCheckFailed,
    refetch: refetchLLMStatus,
  } = useQuery({
    queryKey: ['llm-status'],
    queryFn: async () => (await api.getLLMStatus()).data,
  })

  // Baseline = the settings the server last returned (or that we last saved).
  // While the form differs from it (`dirty`), server data is never applied on
  // top of the user's unsaved edits — the Topbar polls ['settings'] every 30s.
  // `llm_api_key` never comes back from the server, so it has no comparable
  // baseline value: any typed (non-empty) key counts as an unsaved edit too,
  // otherwise a background refetch would silently wipe the input.
  const [baseline, setBaseline] = useState<SettingsType | null>(null)
  const dirty = useMemo(
    () =>
      baseline
        ? !settingsFormEqual(formState, baseline) || formState.llm_api_key !== ''
        : false,
    [formState, baseline],
  )

  // Read through a ref so this effect re-runs only on genuinely new server
  // data, not when `dirty` flips (a dirty→false transition must not replay a
  // stale cache object over the form we just saved).
  const dirtyRef = useRef(false)
  useEffect(() => {
    dirtyRef.current = dirty
  }, [dirty])

  useEffect(() => {
    if (!settings || dirtyRef.current) return
    const normalized = normalizeSettings(settings)
    setFormState(normalized)
    setBaseline(normalized)
  }, [settings])

  const updateSettingsMutation = useMutation({
    mutationFn: (data: SettingsType) => api.updateSettings(data),
    onSuccess: (_result, variables) => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      // What we just sent is now the server truth: rebase the baseline onto
      // its normalized shape (trim/drop empty channels, clamped token limit)
      // so `dirty` clears and background sync works again. The form was
      // already aligned with the payload in handleSave, so nothing is
      // overwritten here except the now-saved API key — and only when the
      // user has not edited it since the click. Edits made mid-flight stay.
      const normalized = normalizeSettings(variables)
      const savedMaxTokens = clampMaxTokensValue(normalized.max_tokens)
      setBaseline({ ...normalized, max_tokens: savedMaxTokens })
      setFormState((prev) => ({
        ...prev,
        llm_api_key: prev.llm_api_key === variables.llm_api_key ? '' : prev.llm_api_key,
      }))
      setSaveError(null)
      setSaveSuccess(true)
      window.setTimeout(() => setSaveSuccess(false), 4000)
    },
    onError: (error) => {
      setSaveSuccess(false)
      setSaveError(getErrorMessage(error))
      window.setTimeout(() => setSaveError(null), 6000)
    },
  })

  const addChannel = () => {
    setFormState((prev) => ({ ...prev, channels: [...prev.channels, ''] }))
  }

  const removeChannel = (index: number) => {
    setFormState((prev) => ({ ...prev, channels: prev.channels.filter((_, i) => i !== index) }))
  }

  const updateChannel = (index: number, value: string) => {
    setFormState((prev) => {
      const channels = [...prev.channels]
      channels[index] = value
      return { ...prev, channels }
    })
  }

  const update = <K extends keyof SettingsType>(key: K, value: SettingsType[K]) => {
    setFormState((prev) => ({ ...prev, [key]: value }))
  }

  // Env overrides: when the effective value differs from the stored one,
  // the field is read-only and shows what is actually in effect.
  const paidEnabledOverridden =
    formState.llm_paid_enabled_effective !== formState.llm_paid_enabled
  const paidCapOverridden =
    formState.llm_paid_daily_cap_usd_effective !== formState.llm_paid_daily_cap_usd

  const apiKeyBadge: { tone: BadgeTone; label: string } =
    formState.llm_api_key_source === 'env'
      ? { tone: 'success', label: 'Ключ из окружения' }
      : formState.llm_api_key_source === 'database'
        ? { tone: 'neutral', label: 'Ключ сохранён в базе' }
        : { tone: 'warning', label: 'Ключ не задан' }

  const presenceError = getPresenceRangeError(formState)
  const sessionPairError = pairFieldError(
    formState.presence_session_min_minutes,
    formState.presence_session_max_minutes,
  )
  const cooldownPairError = pairFieldError(
    formState.presence_cooldown_min_minutes,
    formState.presence_cooldown_max_minutes,
  )

  const handleSave = () => {
    // Invalid ranges are never sent: the backend would reject them with 400.
    if (presenceError) {
      setSaveBlocked(presenceError)
      setSaveSuccess(false)
      setSaveError(null)
      return
    }
    setSaveBlocked(null)
    setSaveSuccess(false)
    setSaveError(null)
    const sentChannels = formState.channels.map((c) => c.trim()).filter((c) => c !== '')
    const payload: SettingsType = {
      ...formState,
      channels: sentChannels,
      // Final clamp: a half-typed value that never got blurred still must
      // not reach the backend outside 20..200.
      max_tokens: clampMaxTokensValue(formState.max_tokens),
      updated_at: new Date().toISOString(),
    }
    // The form must show exactly what was sent: onSuccess then only rebases
    // the baseline, and anything typed after this click stays untouched.
    // An empty channel list keeps the display shape — one blank row, the very
    // fallback normalizeSettings uses for the baseline — so saving zero
    // channels clears `dirty` instead of staying dirty as [] vs [''].
    setFormState({ ...payload, channels: sentChannels.length > 0 ? sentChannels : [''] })
    updateSettingsMutation.mutate(payload)
  }

  // All hooks run unconditionally above; everything below may return early.
  if (settingsLoadFailed) {
    return (
      <>
        <PageHeader
          title="Настройки"
          description="Каналы, мониторинг Twitch, LLM, аудио и поведение ботов"
        />
        <div style={GRID_STYLE}>
          <Alert variant="error">Не удалось загрузить настройки.</Alert>
          <div>
            <Button variant="secondary" size="sm" onClick={() => refetchSettings()}>
              Повторить
            </Button>
          </div>
        </div>
      </>
    )
  }

  // No editable defaults on screen until real settings arrive: saving the
  // fallback literal would wipe the stored configuration.
  if (isLoading || !settings) {
    return <p style={{ color: 'var(--muted)' }}>Загрузка настроек…</p>
  }

  const cooldownUntil = (() => {
    const raw = llmStatus?.free_cooldown_until
    if (!raw) return null
    const timestamp = new Date(raw).getTime()
    if (!Number.isFinite(timestamp) || timestamp <= Date.now()) return null
    return new Date(timestamp)
  })()

  return (
    <>
      <PageHeader
        title="Настройки"
        description="Каналы, мониторинг Twitch, LLM, аудио и поведение ботов"
      />

      <div style={GRID_STYLE}>
        {/* ── Channels ─────────────────────────────────────────────── */}
        <Card title="Twitch-каналы" subtitle="Чаты, в которые заходят боты">
          <div style={{ display: 'grid', gap: 12 }}>
            {formState.channels.map((channel, index) => (
              <div key={index} style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <input
                  className="input"
                  type="text"
                  aria-label={`Канал ${index + 1}`}
                  placeholder="имя_канала"
                  value={channel}
                  onChange={(e) => updateChannel(index, e.target.value)}
                />
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => removeChannel(index)}
                  aria-label={`Удалить канал ${index + 1}`}
                >
                  <X size={14} aria-hidden="true" />
                  Удалить
                </Button>
              </div>
            ))}

            <div>
              <Button variant="secondary" size="sm" onClick={addChannel}>
                <Plus size={14} aria-hidden="true" />
                Добавить канал
              </Button>
            </div>

            <p style={HINT_STYLE}>
              Один канал — обычная настройка для индивидуального использования.
              Добавляйте больше, только если боты должны одновременно заходить в несколько чатов.
            </p>
          </div>
        </Card>

        {/* ── Twitch monitoring ────────────────────────────────────── */}
        <Card title="Мониторинг Twitch" subtitle="Как боты подключаются к стримам">
          <div style={{ display: 'grid', gap: 12 }}>
            <Alert variant="info">
              Мониторинг работает через публичный GraphQL-интерфейс Twitch и IRC-чат
              в режиме только чтение. Регистрация приложения и токены для этого не нужны.
            </Alert>

            <Input
              label="Twitch Client ID"
              type="text"
              placeholder="ваш_client_id"
              value={formState.twitch_client_id}
              onChange={(e) => update('twitch_client_id', e.target.value)}
            />
            <p style={HINT_STYLE}>Не требуется для мониторинга.</p>

            <Input
              label="Twitch Client Secret"
              type="password"
              placeholder={
                formState.twitch_client_secret_set
                  ? 'Секрет сохранён — оставьте пустым, чтобы не менять'
                  : 'ваш_client_secret'
              }
              value={formState.twitch_client_secret}
              onChange={(e) => update('twitch_client_secret', e.target.value)}
              autoComplete="new-password"
            />
            <p style={HINT_STYLE}>
              Не требуется для мониторинга. Пригодится только в будущем — как
              резервный вариант через официальный Helix API.
            </p>
          </div>
        </Card>

        {/* ── OpenRouter ───────────────────────────────────────────── */}
        <Card title="OpenRouter" subtitle="Генерация сообщений ботов">
          <div style={{ display: 'grid', gap: 12 }}>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 12 }}>
              <Input
                label="Бесплатная модель"
                type="text"
                placeholder="openrouter/free"
                value={formState.llm_model}
                onChange={(e) => update('llm_model', e.target.value)}
              />
              <Input
                label="Платная модель (fallback)"
                type="text"
                placeholder="z-ai/glm-5.3-flash"
                value={formState.llm_paid_model}
                onChange={(e) => update('llm_paid_model', e.target.value)}
              />
            </div>

            <div style={{ display: 'grid', gap: 12 }}>
              <SwitchField
                label="Платный fallback включён"
                checked={formState.llm_paid_enabled}
                disabled={paidEnabledOverridden}
                onChange={(checked) => update('llm_paid_enabled', checked)}
                hint={
                  paidEnabledOverridden
                    ? `Задаётся окружением. Фактически: ${formState.llm_paid_enabled_effective ? 'включён' : 'выключен'}.`
                    : 'Включается, когда бесплатная модель недоступна или исчерпана.'
                }
              />

              <Input
                label="Дневной лимит расходов, $"
                type="number"
                min={0}
                step={0.5}
                value={formState.llm_paid_daily_cap_usd}
                disabled={paidCapOverridden}
                onChange={(e) => update('llm_paid_daily_cap_usd', Number(e.target.value))}
              />
              <p style={HINT_STYLE}>
                {paidCapOverridden
                  ? `Задаётся окружением. Фактический лимит: $${Number(formState.llm_paid_daily_cap_usd_effective).toFixed(2)} в день.`
                  : '0 или отрицательное значение — без лимита.'}
              </p>
            </div>

            <div style={{ display: 'grid', gap: 6 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <label className="field__label" htmlFor="llm-api-key">
                  OpenRouter API-ключ
                </label>
                <Badge tone={apiKeyBadge.tone}>{apiKeyBadge.label}</Badge>
              </div>
              <input
                id="llm-api-key"
                className="input"
                type="password"
                autoComplete="new-password"
                placeholder={formState.llm_api_key_set ? '•••••••• (ключ задан)' : 'sk-or-v1-…'}
                value={formState.llm_api_key}
                onChange={(e) => update('llm_api_key', e.target.value)}
              />
              <p style={HINT_STYLE}>
                Оставьте пустым, чтобы сохранить текущий ключ. Ключ читается из
                переменной окружения OPENROUTER_API_KEY (приоритет) или из базы данных.
              </p>
            </div>

            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center' }}>
              <Button variant="secondary" size="sm" loading={llmChecking} onClick={() => refetchLLMStatus()}>
                Проверить подключение
              </Button>
              {llmStatus && (
                <span style={HINT_STYLE}>
                  {llmStatus.available ? 'Подключение: OK' : 'Подключение: ошибка'}
                  {' • '}
                  Модель: {llmStatus.free_model || llmStatus.model || '—'}
                  {' • '}
                  Генерация: {llmStatus.generation_available ? 'доступна' : 'недоступна'}
                  {' • '}
                  Расход сегодня: ${Number(llmStatus.paid_spent_today_usd ?? 0).toFixed(2)}
                  {' • '}
                  Бесплатных осталось: {llmStatus.free_requests_remaining ?? 0}
                  {' из '}
                  {llmStatus.free_requests_limit ?? 0}
                  {cooldownUntil && (
                    <>
                      {' • '}
                      Бесплатная модель на паузе до{' '}
                      {cooldownUntil.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}
                    </>
                  )}
                  {!llmStatus.generation_available && (llmStatus.last_error || llmStatus.error) && (
                    <>
                      {' • '}
                      Причина: {llmStatus.last_error || llmStatus.error}
                    </>
                  )}
                </span>
              )}
              {!llmStatus && llmCheckFailed && !llmChecking && (
                <span style={{ ...HINT_STYLE, color: 'var(--error)' }}>
                  Не удалось проверить подключение.
                </span>
              )}
            </div>
          </div>
        </Card>

        {/* ── Audio ────────────────────────────────────────────────── */}
        <Card title="Аудио" subtitle="Анализ аудиодорожки стрима">
          <div style={{ display: 'grid', gap: 12 }}>
            <SwitchField
              label="Включить анализ аудио со стрима"
              checked={formState.audio_enabled}
              onChange={(checked) => update('audio_enabled', checked)}
              hint="Backend периодически записывает аудио стрима и отправляет фрагменты в STT API. Нужны streamlink и ffmpeg."
            />

            <p style={HINT_STYLE}>
              Язык транскрипции задаётся переменной окружения AUDIO_STT_LANGUAGE
              (по умолчанию ru) и здесь не меняется.
            </p>

            <div>
              <Button variant="secondary" size="sm" onClick={() => setAudioModalOpen(true)}>
                <Mic size={14} aria-hidden="true" />
                Открыть тест аудио
              </Button>
            </div>
          </div>
        </Card>

        {/* ── Behavior ─────────────────────────────────────────────── */}
        <Card title="Поведение" subtitle="Отладка, промпты и лимиты сообщений">
          <div style={{ display: 'grid', gap: 12 }}>
            <SwitchField
              label="Режим отладки"
              checked={formState.debug_mode}
              onChange={(checked) => update('debug_mode', checked)}
              hint="Боты подключаются к тестовому каналу, но сообщения не отправляются в реальный чат."
            />

            {formState.debug_mode && (
              <Input
                label="Тестовый канал"
                type="text"
                placeholder="test_streamer"
                value={formState.debug_channel}
                onChange={(e) => update('debug_channel', e.target.value)}
              />
            )}

            <div style={{ display: 'grid', gap: 12 }}>
              <div style={{ display: 'grid', gap: 6 }}>
                <label className="field__label" htmlFor="positive-prompt">
                  Позитивный промпт
                </label>
                <textarea
                  id="positive-prompt"
                  className="input"
                  rows={4}
                  placeholder="Пиши короткие естественные сообщения, используй сленг…"
                  value={formState.positive_prompt}
                  onChange={(e) => update('positive_prompt', e.target.value)}
                />
              </div>

              <div style={{ display: 'grid', gap: 6 }}>
                <label className="field__label" htmlFor="negative-prompt">
                  Негативный промпт
                </label>
                <textarea
                  id="negative-prompt"
                  className="input"
                  rows={4}
                  placeholder="Не используй эмодзи, не оскорбляй, не спамь…"
                  value={formState.negative_prompt}
                  onChange={(e) => update('negative_prompt', e.target.value)}
                />
              </div>
            </div>

            <Input
              label="Максимум токенов в сообщении"
              type="number"
              min={20}
              max={200}
              value={formState.max_tokens}
              onChange={(e) => {
                // No clamping mid-typing: it would make "50" impossible to enter.
                const next = parseMaxTokensInput(e.target.value)
                if (next !== null) update('max_tokens', next)
              }}
              onBlur={(e) => {
                const next = parseMaxTokensInput(e.target.value)
                if (next !== null) update('max_tokens', clampMaxTokensValue(next))
              }}
            />
          </div>
        </Card>

        {/* ── Presence simulation ──────────────────────────────────── */}
        <Card title="Симуляция присутствия" subtitle="Сессии в чате и паузы между ними">
          <div style={{ display: 'grid', gap: 12 }}>
            <SwitchField
              label="Симуляция присутствия включена"
              checked={formState.presence_enabled}
              onChange={(checked) => update('presence_enabled', checked)}
              hint="Боты находятся в чате ограниченными сессиями и делают паузы между ними."
            />

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: 12 }}>
              <Input
                label="Мин. сессия, мин"
                type="number"
                min={1}
                max={240}
                value={formState.presence_session_min_minutes}
                error={sessionPairError}
                onChange={(e) => update('presence_session_min_minutes', Number(e.target.value))}
              />
              <Input
                label="Макс. сессия, мин"
                type="number"
                min={1}
                max={240}
                value={formState.presence_session_max_minutes}
                error={sessionPairError}
                onChange={(e) => update('presence_session_max_minutes', Number(e.target.value))}
              />
              <Input
                label="Мин. пауза, мин"
                type="number"
                min={1}
                max={240}
                value={formState.presence_cooldown_min_minutes}
                error={cooldownPairError}
                onChange={(e) => update('presence_cooldown_min_minutes', Number(e.target.value))}
              />
              <Input
                label="Макс. пауза, мин"
                type="number"
                min={1}
                max={240}
                value={formState.presence_cooldown_max_minutes}
                error={cooldownPairError}
                onChange={(e) => update('presence_cooldown_max_minutes', Number(e.target.value))}
              />
            </div>

            <p style={HINT_STYLE}>Диапазоны: от 1 до 240 минут, минимум не больше максимума.</p>

            <div style={{ display: 'grid', gap: 12 }}>
              <SwitchField
                label="Приветствие при входе в чат"
                checked={formState.presence_greeting_enabled}
                onChange={(checked) => update('presence_greeting_enabled', checked)}
              />
              <SwitchField
                label="Прощание при выходе из чата"
                checked={formState.presence_farewell_enabled}
                onChange={(checked) => update('presence_farewell_enabled', checked)}
              />
            </div>
          </div>
        </Card>

        {/* ── Save ─────────────────────────────────────────────────── */}
        <div style={{ display: 'grid', gap: 12, justifyItems: 'start' }}>
          {dirty && (
            <Alert variant="info">
              Есть несохранённые изменения. Фоновые обновления настроек не перезаписывают их,
              пока вы не сохраните форму.
            </Alert>
          )}
          {saveBlocked && presenceError && <Alert variant="error">{saveBlocked}</Alert>}
          {saveSuccess && <Alert variant="success">Настройки сохранены.</Alert>}
          {saveError && <Alert variant="error">{saveError}</Alert>}

          <Button
            variant="primary"
            loading={updateSettingsMutation.isPending}
            onClick={handleSave}
          >
            Сохранить настройки
          </Button>
        </div>
      </div>

      <AudioTestModal
        isOpen={audioModalOpen}
        onClose={() => setAudioModalOpen(false)}
        channel={formState.channels.map((c) => c.trim()).find(Boolean) ?? ''}
      />
    </>
  )
}
