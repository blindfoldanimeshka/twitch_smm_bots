import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Copy, Mail, Mic, Plus, RefreshCw, Trash2, X } from 'lucide-react'
import { api, getErrorMessage, getErrorStatus, getTempMailErrorMessage } from '../api/client'
import type {
  Settings as SettingsType,
  TempMailInbox,
  TempMailMessage,
  TempMailMessageBody,
} from '../api/client'
import { PageHeader } from '../components/layout/PageHeader'
import { AudioTestModal } from '../components/AudioTestModal'
import { Alert } from '../components/ui/Alert'
import { Badge, type BadgeTone } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { EmptyState } from '../components/ui/EmptyState'
import { Input } from '../components/ui/Input'
import { Modal } from '../components/ui/Modal'
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
    twitch_client_secret_clear: settings.twitch_client_secret_clear ?? false,
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
 * The form right after a save succeeded. `payload` is exactly what the server
 * accepted, so a field the user has not touched since the click snaps to its
 * saved shape, while anything typed mid-flight stays untouched (and keeps
 * `dirty` on) until it is saved too.
 * - channels: displayed trimmed/deduped, with one blank row kept as the shape
 *   of an intentionally empty list — saving zero channels clears `dirty`;
 * - max_tokens: replaced by the saved clamp only when the current input still
 *   clamps to it, so a half-typed value that would save differently stays;
 * - both write-only secrets: cleared only when unchanged since the click,
 *   because the server never echoes them back;
 * - twitch_client_secret_clear: dropped once the save carried it, kept when the
 *   click happened while that request was still in flight.
 */
function reconcileAfterSave(form: SettingsType, payload: SettingsType): SettingsType {
  const currentChannels = form.channels.map((c) => c.trim()).filter((c) => c !== '')
  const channelsSaved =
    currentChannels.length === payload.channels.length &&
    currentChannels.every((name, index) => name === payload.channels[index])

  return {
    ...form,
    channels: channelsSaved
      ? payload.channels.length > 0
        ? payload.channels
        : ['']
      : form.channels,
    max_tokens:
      clampMaxTokensValue(form.max_tokens) === payload.max_tokens
        ? payload.max_tokens
        : form.max_tokens,
    llm_api_key: form.llm_api_key === payload.llm_api_key ? '' : form.llm_api_key,
    twitch_client_secret:
      form.twitch_client_secret === payload.twitch_client_secret ? '' : form.twitch_client_secret,
    // The clear request is consumed by the save (the server erased the secret and
    // reset the flag), so the form flag goes false — except when it was clicked
    // while the request was in flight: that erase has not been sent yet.
    twitch_client_secret_clear: payload.twitch_client_secret_clear
      ? false
      : form.twitch_client_secret_clear,
    updated_at: payload.updated_at,
  }
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

/* ── Temporary mail inbox ─────────────────────────────────────────── */

const TEMP_INBOX_KEY = 'ttv-sm:temp-inbox'

interface StoredTempInbox {
  id: string
  email: string
  /** Мс эпоха: ящик живёт на стороне сервиса около часа. */
  expiresAtMs: number
}

/**
 * Читает сохранённый ящик и выбрасывает его вместе с копией в storage,
 * если запись битая или её срок уже истёк — UI никогда не работает по
 * мёртвому id.
 */
function loadStoredTempInbox(): StoredTempInbox | null {
  try {
    const raw = localStorage.getItem(TEMP_INBOX_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<StoredTempInbox>
    if (
      typeof parsed.id !== 'string' ||
      typeof parsed.email !== 'string' ||
      typeof parsed.expiresAtMs !== 'number'
    ) {
      throw new Error('stored inbox is malformed')
    }
    if (parsed.expiresAtMs <= Date.now()) throw new Error('stored inbox has expired')
    return { id: parsed.id, email: parsed.email, expiresAtMs: parsed.expiresAtMs }
  } catch {
    try {
      localStorage.removeItem(TEMP_INBOX_KEY)
    } catch {
      /* браузер отключил storage: чистить нечего */
    }
    return null
  }
}

/** Запоминает созданный ящик; запасная дата жизни — задокументированный час. */
function persistTempInbox(inbox: TempMailInbox): StoredTempInbox {
  const parsed = new Date(inbox.expires_at).getTime()
  const stored: StoredTempInbox = {
    id: inbox.id,
    email: inbox.email,
    expiresAtMs: Number.isFinite(parsed) ? parsed : Date.now() + 60 * 60 * 1000,
  }
  try {
    localStorage.setItem(TEMP_INBOX_KEY, JSON.stringify(stored))
  } catch {
    /* приватный режим: ящик работает хотя бы до конца визита */
  }
  return stored
}

function forgetTempInbox() {
  try {
    localStorage.removeItem(TEMP_INBOX_KEY)
  } catch {
    /* браузер отключил storage: состояние в памяти очищает вызывающий код */
  }
}

/**
 * Провайдерские метки времени → «дд.мм чч:мм» в локали пользователя.
 * Обнулённый timestamp провайдера (Guerrilla отдаёт «0001-01-01») и всё,
 * что раньше 2000 года, — не дата, а её отсутствие: показываем «—».
 */
const MAIL_MIN_VALID_MS = Date.UTC(2000, 0, 1)

function formatMailTime(iso: string): string {
  const date = new Date(iso)
  const time = date.getTime()
  if (!Number.isFinite(time) || time < MAIL_MIN_VALID_MS) return '—'
  return date.toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const MAIL_ROW_STYLE = {
  display: 'grid',
  gap: 2,
  justifyItems: 'start',
  maxWidth: '100%',
  textAlign: 'left',
  whiteSpace: 'normal',
} as const

/** Тело письма рендерится только текстом в <pre>: HTML провайдера сюда не попадает. */
const MAIL_BODY_STYLE = {
  margin: 0,
  padding: 12,
  background: 'var(--bg)',
  border: '1px solid var(--border)',
  borderRadius: 'var(--radius-md)',
  font: '13px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
  whiteSpace: 'pre-wrap',
  wordBreak: 'break-word',
  maxHeight: '50vh',
  overflowY: 'auto',
} as const

function TempMailCard() {
  const [inbox, setInbox] = useState<StoredTempInbox | null>(loadStoredTempInbox)
  const [alias, setAlias] = useState('')
  const [notice, setNotice] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [openedMail, setOpenedMail] = useState<TempMailMessageBody | null>(null)

  const {
    data: inboxView,
    isFetching: listFetching,
    isError: listFailed,
    error: listError,
    refetch: refetchList,
  } = useQuery({
    queryKey: ['temp-inbox', inbox?.id],
    queryFn: async () => (await api.getTempInbox(inbox!.id)).data,
    enabled: inbox !== null,
    // Провайдер — третья сторона: повтор только по кнопке пользователя.
    retry: false,
    refetchOnWindowFocus: false,
  })

  // «Нет такого ящика» от провайдера означает, что сохранённый id мёртв:
  // убираем его и показываем обычную форму создания с пояснением.
  const listMissing = listFailed && getErrorStatus(listError) === 404
  useEffect(() => {
    if (!listMissing || !inbox) return
    forgetTempInbox()
    setInbox(null)
    setNotice('Ящик истёк или был удалён на стороне сервиса. Создайте новый.')
  }, [listMissing, inbox])

  const createInbox = useMutation({
    mutationFn: async (value: string) =>
      (await api.createTempInbox(value.trim() || undefined)).data,
    onSuccess: (created) => {
      setActionError(null)
      setNotice(null)
      setAlias('')
      setInbox(persistTempInbox(created))
    },
    onError: (error) => {
      setNotice(null)
      setActionError(getTempMailErrorMessage(error))
    },
  })

  const deleteInbox = useMutation({
    mutationFn: (id: string) => api.deleteTempInbox(id),
    onSuccess: () => {
      forgetTempInbox()
      setActionError(null)
      setNotice(null)
      setInbox(null)
    },
    onError: (error) => setActionError(getTempMailErrorMessage(error)),
  })

  const copyEmail = async () => {
    if (!inbox) return
    try {
      await navigator.clipboard.writeText(inbox.email)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      setNotice('Не удалось скопировать адрес — выделите его и скопируйте вручную.')
    }
  }

  const openMail = async (mailID: string) => {
    if (!inbox) return
    setActionError(null)
    try {
      const { data } = await api.getTempMessage(inbox.id, mailID)
      setOpenedMail(data)
    } catch (error) {
      setActionError(getTempMailErrorMessage(error))
    }
  }

  // Список отдельен от самого ящика: «Писем нет» рендерится только после
  // успешной загрузки, а ошибка — как Alert с кнопкой повтора.
  const mailList: TempMailMessage[] = inboxView?.messages ?? []

  const expiryLabel = inbox
    ? new Date(inbox.expiresAtMs).toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit',
      })
    : null

  return (
    <Card title="Временная почта" subtitle="Одноразовый ящик для тестовых регистраций">
      <div style={{ display: 'grid', gap: 12 }}>
        {notice && <Alert variant="info">{notice}</Alert>}
        {actionError && <Alert variant="error">{actionError}</Alert>}

        {!inbox ? (
          <>
            <p style={HINT_STYLE}>
              Ящик живёт около часа и только принимает письма — отправить отсюда нельзя.
              Оставьте имя пустым, чтобы сервис выдал свой адрес.
            </p>
            <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end', flexWrap: 'wrap' }}>
              <div style={{ flex: '1 1 240px' }}>
                <Input
                  label="Имя ящика (необязательно)"
                  type="text"
                  placeholder="johnny.test"
                  value={alias}
                  maxLength={64}
                  onChange={(e) => setAlias(e.target.value)}
                />
              </div>
              <div>
                <Button loading={createInbox.isPending} onClick={() => createInbox.mutate(alias)}>
                  Создать ящик
                </Button>
              </div>
            </div>
          </>
        ) : (
          <>
            <div style={{ display: 'grid', gap: 6, justifyItems: 'start' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <span
                  style={{
                    padding: '6px 10px',
                    background: 'var(--bg)',
                    border: '1px solid var(--border)',
                    borderRadius: 'var(--radius-pill)',
                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                    fontSize: 13,
                    userSelect: 'all',
                    overflowWrap: 'anywhere',
                  }}
                >
                  {inboxView?.email || inbox.email}
                </span>
                <Button variant="secondary" size="sm" onClick={copyEmail}>
                  <Copy size={14} aria-hidden="true" />
                  {copied ? 'Скопировано' : 'Копировать'}
                </Button>
              </div>
              {expiryLabel && <span style={HINT_STYLE}>Действует до {expiryLabel}</span>}
            </div>

            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <Button
                variant="secondary"
                size="sm"
                loading={listFetching}
                onClick={() => refetchList()}
              >
                <RefreshCw size={14} aria-hidden="true" />
                Обновить
              </Button>
              <Button
                variant="danger"
                size="sm"
                loading={deleteInbox.isPending}
                onClick={() => deleteInbox.mutate(inbox.id)}
              >
                <Trash2 size={14} aria-hidden="true" />
                Удалить
              </Button>
            </div>

            {listFailed ? (
              <Alert variant="error" title="Не удалось загрузить письма">
                <div style={{ display: 'grid', gap: 8 }}>
                  <span>{getTempMailErrorMessage(listError)}</span>
                  <div style={{ justifySelf: 'start' }}>
                    <Button
                      variant="secondary"
                      size="sm"
                      loading={listFetching}
                      onClick={() => refetchList()}
                    >
                      Повторить
                    </Button>
                  </div>
                </div>
              </Alert>
            ) : !inboxView ? (
              <p style={HINT_STYLE}>Загрузка писем…</p>
            ) : mailList.length === 0 ? (
              <EmptyState
                title="Писем нет"
                description="Письма появятся здесь, как только на адрес кто-нибудь напишет."
                icon={Mail}
              />
            ) : (
              <div style={{ display: 'grid', gap: 8 }}>
                {mailList.map((mail) => (
                  <button
                    key={mail.id}
                    type="button"
                    className="btn btn--secondary"
                    style={MAIL_ROW_STYLE}
                    onClick={() => openMail(mail.id)}
                  >
                    <span style={{ fontWeight: 600 }}>{mail.subject || 'Без темы'}</span>
                    <span style={{ fontSize: 12, color: 'var(--muted)' }}>
                      {mail.from} • {formatMailTime(mail.received_at)}
                    </span>
                    {mail.excerpt && (
                      <span style={{ fontSize: 12, color: 'var(--muted)' }}>{mail.excerpt}</span>
                    )}
                  </button>
                ))}
              </div>
            )}
          </>
        )}
      </div>

      <Modal
        open={openedMail !== null}
        onClose={() => setOpenedMail(null)}
        title={openedMail?.subject || 'Письмо'}
        width={680}
      >
        {openedMail && (
          <div style={{ display: 'grid', gap: 8 }}>
            <p style={HINT_STYLE}>
              От: {openedMail.from} • {formatMailTime(openedMail.received_at)}
            </p>
            <pre style={MAIL_BODY_STYLE}>{openedMail.body || 'Тело письма пустое.'}</pre>
          </div>
        )}
      </Modal>
    </Card>
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
    twitch_client_secret_clear: false,
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
        ? !settingsFormEqual(formState, baseline) ||
          formState.llm_api_key !== '' ||
          // An erase request is an unsaved edit even though the secret input
          // itself already looks empty.
          formState.twitch_client_secret_clear
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
      // What we just sent is now the server truth: the baseline is rebased onto
      // its normalized shape (trim/drop empty channels, clamped token limit) so
      // `dirty` clears and background sync works again. Both write-only secrets
      // are blanked in the baseline — GET never returns them, so keeping the
      // just-saved one there would hold `dirty` on forever.
      const normalized = normalizeSettings(variables)
      setBaseline({
        ...normalized,
        max_tokens: clampMaxTokensValue(normalized.max_tokens),
        twitch_client_secret: '',
        // The erase was consumed by this save: the server reset the flag, and a
        // baseline carrying it would hold `dirty` on forever.
        twitch_client_secret_clear: false,
      })
      // The form is reconciled against the same payload: untouched fields snap to
      // the saved shape, edits made while the request was in flight stay put.
      setFormState((prev) => reconcileAfterSave(prev, variables))
      setSaveError(null)
      setSaveSuccess(true)
      window.setTimeout(() => setSaveSuccess(false), 4000)
    },
    onError: (error) => {
      // A failed save changes nothing on the server: form and baseline stay as
      // they are, so `dirty` keeps the unsaved edits visible and retryable.
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

  // An empty secret field only means "keep the stored secret", so the erase has
  // to travel on the transient flag; typing a new secret cancels it again.
  const clearTwitchSecret = () => {
    setFormState((prev) => ({
      ...prev,
      twitch_client_secret: '',
      twitch_client_secret_clear: true,
    }))
  }

  const updateTwitchSecret = (value: string) => {
    setFormState((prev) => ({
      ...prev,
      twitch_client_secret: value,
      twitch_client_secret_clear: false,
    }))
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
    // The form is left exactly as the user typed it: normalizing it here would
    // make a whitespace-only edit look saved even if this request fails.
    // onSuccess reconciles it against this payload once the server has it.
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
                formState.twitch_client_secret_clear
                  ? 'Секрет будет удалён из базы при сохранении — введите новый, чтобы отменить'
                  : formState.twitch_client_secret_set
                    ? 'Секрет сохранён — оставьте пустым, чтобы не менять'
                    : 'ваш_client_secret'
              }
              value={formState.twitch_client_secret}
              onChange={(e) => updateTwitchSecret(e.target.value)}
              autoComplete="new-password"
            />
            {formState.twitch_client_secret_set && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <Button variant="danger" size="sm" onClick={clearTwitchSecret}>
                  Очистить секрет
                </Button>
                {formState.twitch_client_secret_clear && (
                  <span style={HINT_STYLE}>Секрет будет удалён из базы при сохранении.</span>
                )}
              </div>
            )}
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

        {/* ── Temporary mail ───────────────────────────────────────── */}
        {/* In-app Guerrilla inbox: the external TempFastMail link is gone. */}
        <TempMailCard />

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

      {/* The audio test records what the backend is analysing right now, so it
          is the saved channel that matters: an unsaved edit in the form must
          not point the modal at a channel the bots have not joined. */}
      <AudioTestModal
        isOpen={audioModalOpen}
        onClose={() => setAudioModalOpen(false)}
        channel={settings.channels.map((c) => c.trim()).find(Boolean) ?? ''}
      />
    </>
  )
}
