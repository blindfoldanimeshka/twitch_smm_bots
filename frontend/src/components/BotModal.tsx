import { useEffect, useState } from 'react'
import type { CSSProperties, FormEvent } from 'react'
import { api, getTokenErrorMessage } from '../api/client'
import type { Bot, TokenStatus } from '../api/client'
import { useBotForm } from '../hooks/useBotForm'
import type { BotFormData } from '../hooks/useBotForm'
import { Alert } from './ui/Alert'
import { Badge } from './ui/Badge'
import { Button } from './ui/Button'
import { Input } from './ui/Input'
import { Modal } from './ui/Modal'
import { BotForm } from './BotForm'

interface BotModalProps {
  open: boolean
  onClose: () => void
  /** Bot being edited, or null to create a new one. */
  editingBot: Bot | null
  isSubmitting: boolean
  /** Mutation error shown inline at the top of the form. */
  error: string | null
  onSubmit: (data: BotFormData) => void
  /** Called after a successful token import so the bot list can refresh. */
  onTokenChanged?: () => void
}

const TWO_COL_GRID: CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
  gap: 12,
}

// "05 нояб., 14:30" — compact enough for a status line, unambiguous enough
// for a token that may expire days from now.
function formatExpiry(iso: string): string | null {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) {
    return null
  }
  return date.toLocaleString('ru-RU', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * "Токен Twitch" section of the edit dialog: import a fresh access (+ optional
 * refresh) token via the dedicated token endpoint. Token values live only in
 * this component's local state — they never enter the bot settings form or
 * its update payload — and are cleared after a successful import.
 */
function BotTokenPanel({ bot, onSaved }: { bot: Bot; onSaved: () => void }) {
  const [accessToken, setAccessToken] = useState('')
  const [refreshToken, setRefreshToken] = useState('')
  const [status, setStatus] = useState<TokenStatus | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [importing, setImporting] = useState(false)

  // Validate the stored token live when the panel opens. If the request fails,
  // fall back silently to the metadata already present on the bot row.
  useEffect(() => {
    let cancelled = false
    api
      .getBotToken(bot.id)
      .then((response) => {
        if (!cancelled) {
          setStatus(response.data)
        }
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [bot.id])

  // Latest known status: live GET (or import result) first, list row as the
  // initial guess.
  const shown: TokenStatus = status ?? {
    valid: bot.token_valid,
    login: bot.twitch_login,
    expires_at: bot.token_expires_at,
    has_refresh: bot.token_has_refresh,
    scopes: bot.token_scopes,
  }

  const expiryText = shown.expires_at ? formatExpiry(shown.expires_at) : null

  const handleImport = async () => {
    const access = accessToken.trim()
    if (!access) {
      return
    }
    setImporting(true)
    setError(null)
    try {
      const response = await api.importBotToken(bot.id, access, refreshToken.trim() || undefined)
      setStatus(response.data)
      // Secrets must not linger in the dialog once stored.
      setAccessToken('')
      setRefreshToken('')
      onSaved()
    } catch (importError) {
      setError(getTokenErrorMessage(importError))
    } finally {
      setImporting(false)
    }
  }

  // Importing access-only over a bot that has a refresh token deletes the
  // stored refresh token — warn beforehand, calmly.
  const losingRefresh = shown.has_refresh && refreshToken.trim() === ''

  return (
    <section
      aria-label="Токен Twitch"
      style={{
        display: 'grid',
        gap: 10,
        borderTop: '1px solid var(--border)',
        paddingTop: 14,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap' }}>
        <span style={{ fontWeight: 600, fontSize: 14 }}>Токен Twitch</span>
        {!shown.valid && !shown.expires_at ? (
          <Badge tone="neutral">токен не импортирован</Badge>
        ) : shown.valid ? (
          <Badge tone="success">
            {expiryText ? `действует до ${expiryText}` : 'действует'}
          </Badge>
        ) : (
          <Badge tone="error">{expiryText ? `истёк ${expiryText}` : 'истёк'}</Badge>
        )}
        {shown.valid && shown.has_refresh && (
          <Badge tone="accent">обновляется автоматически</Badge>
        )}
      </div>

      <div style={TWO_COL_GRID}>
        <Input
          label="Access-токен"
          type="password"
          autoComplete="new-password"
          placeholder="oauth:…"
          value={accessToken}
          onChange={(event) => setAccessToken(event.target.value)}
          disabled={importing}
        />
        <Input
          label="Refresh-токен (необязательно)"
          type="password"
          autoComplete="new-password"
          placeholder="без него автообновление отключится"
          value={refreshToken}
          onChange={(event) => setRefreshToken(event.target.value)}
          disabled={importing}
        />
      </div>

      {losingRefresh && (
        <Alert variant="warning">
          Автообновление будет отключено — сохранённый refresh-токен будет удалён
        </Alert>
      )}

      {error && <Alert variant="error">{error}</Alert>}

      <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
        <Button
          variant="secondary"
          onClick={handleImport}
          loading={importing}
          disabled={accessToken.trim() === ''}
        >
          Проверить и сохранить
        </Button>
        <span style={{ fontSize: 12, color: 'var(--muted)' }}>
          Токен проверяется на стороне Twitch и сохраняется только в зашифрованном виде
        </span>
      </div>
    </section>
  )
}

/** Add/edit bot dialog. Owns the form state via `useBotForm`. */
export function BotModal({
  open,
  onClose,
  editingBot,
  isSubmitting,
  error,
  onSubmit,
  onTokenChanged,
}: BotModalProps) {
  const form = useBotForm(editingBot ?? undefined)

  const handleClose = () => {
    form.reset()
    onClose()
  }

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    if (!form.isValid) {
      return
    }
    onSubmit(form.formData)
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={editingBot ? 'Редактировать бота' : 'Новый бот'}
      width={640}
      footer={
        <>
          <Button variant="secondary" onClick={handleClose} disabled={isSubmitting}>
            Отмена
          </Button>
          <Button
            type="submit"
            form="bot-form"
            loading={isSubmitting}
            disabled={!form.isValid}
          >
            {editingBot ? 'Сохранить' : 'Создать бота'}
          </Button>
        </>
      }
    >
      <div style={{ display: 'grid', gap: 12 }}>
        {error && <Alert variant="error">{error}</Alert>}
        <BotForm
          formData={form.formData}
          errors={form.errors}
          touchedFields={form.touchedFields}
          isEditing={Boolean(editingBot)}
          onChange={form.handleChange}
          onBlur={form.handleBlur}
          onSubmit={handleSubmit}
        />
        {/* key per bot: the panel (token inputs, status) remounts when the
            dialog switches bots, so no stale token state survives. */}
        {editingBot && (
          <BotTokenPanel
            key={editingBot.id}
            bot={editingBot}
            onSaved={onTokenChanged ?? (() => {})}
          />
        )}
      </div>
    </Modal>
  )
}
