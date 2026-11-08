import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, type Settings as SettingsType, type LLMStatus } from '../api/client'
import { GridBeam } from '../components/ui/grid-beam'
import { SettingsLogPanel } from '../components/SettingsLogPanel'
import { Switch } from '../components/ui/switch'
import { AudioTestModal } from '../components/AudioTestModal'

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
    updated_at: '',
  })
  const [testingConnection, setTestingConnection] = useState(false)
  const [llmStatus, setLlmStatus] = useState<LLMStatus | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [saveError, setSaveError] = useState(false)
  const [audioModalOpen, setAudioModalOpen] = useState(false)

  const { data: settings, isLoading } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => {
      const res = await api.getSettings()
      return res.data
    },
  })

  useEffect(() => {
    if (settings) {
      setFormState({
        ...settings,
        channels: settings.channels.length > 0 ? settings.channels : [''],
        llm_provider: settings.llm_provider || 'openrouter',
        llm_api_url: settings.llm_api_url || 'https://openrouter.ai/api/v1',
        llm_model: settings.llm_model || 'openrouter/free',
        llm_paid_model: settings.llm_paid_model ?? 'z-ai/glm-5.3-flash',
        llm_paid_enabled: settings.llm_paid_enabled ?? true,
        llm_paid_daily_cap_usd: settings.llm_paid_daily_cap_usd ?? 1,
        llm_paid_enabled_effective: settings.llm_paid_enabled_effective ?? settings.llm_paid_enabled ?? true,
        llm_paid_daily_cap_usd_effective: settings.llm_paid_daily_cap_usd_effective ?? settings.llm_paid_daily_cap_usd ?? 1,
        llm_api_key: '',
        llm_api_key_set: settings.llm_api_key_set ?? false,
        llm_api_key_source: settings.llm_api_key_source ?? 'none',
        twitch_client_id: settings.twitch_client_id || '',
        twitch_client_secret: settings.twitch_client_secret || '',
      })
    }
  }, [settings])

  useEffect(() => {
    let cancelled = false
    api.getLLMStatus()
      .then((response) => {
        if (!cancelled) {
          setLlmStatus(response.data)
        }
      })
      .catch(() => {
        // Status panel simply stays hidden until the connection test succeeds.
      })
    return () => {
      cancelled = true
    }
  }, [])

  const updateSettingsMutation = useMutation({
    mutationFn: (data: SettingsType) => api.updateSettings(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      setSaveSuccess(true)
      setSaveError(false)
      setTimeout(() => setSaveSuccess(false), 3000)
    },
    onError: () => {
      setSaveError(true)
      setSaveSuccess(false)
      setTimeout(() => setSaveError(false), 3000)
    },
  })

  const addChannel = () => {
    setFormState(prev => ({ ...prev, channels: [...prev.channels, ''] }))
  }

  const removeChannel = (index: number) => {
    setFormState(prev => ({ ...prev, channels: prev.channels.filter((_, i) => i !== index) }))
  }

  const updateChannel = (index: number, value: string) => {
    setFormState(prev => {
      const newChannels = [...prev.channels]
      newChannels[index] = value
      return { ...prev, channels: newChannels }
    })
  }

  const testLLMConnection = async () => {
    setTestingConnection(true)
    try {
      const response = await api.getLLMStatus()
      const status = response.data
      setLlmStatus(status)
      if (status.available) {
        const freeModel = status.free_model || status.model || 'N/A'
        let message = '✓ Подключение успешно! Провайдер ' + status.provider + ', бесплатная модель ' + freeModel + '.'
        if (status.generation_available === false) {
          message += '\n\n⚠ Генерация недоступна: ' + (status.last_error || status.error || 'причина не указана')
        }
        alert(message)
      } else {
        alert('✗ Подключение не удалось: ' + (status.error || status.last_error || 'Неизвестная ошибка'))
      }
    } catch (error) {
      alert('✗ Ошибка при проверке подключения')
    } finally {
      setTestingConnection(false)
    }
  }

  const apiKeySourceLabel =
    formState.llm_api_key_source === 'env'
      ? 'Ключ из окружения'
      : formState.llm_api_key_source === 'database'
        ? 'Ключ сохранён в БД'
        : 'Ключ не задан'

  const paidEnabledOverridden =
    formState.llm_paid_enabled_effective !== formState.llm_paid_enabled
  const paidCapOverridden =
    formState.llm_paid_daily_cap_usd_effective !== formState.llm_paid_daily_cap_usd

  const cooldownUntil = (() => {
    const raw = llmStatus?.free_cooldown_until
    if (!raw) return null
    const timestamp = new Date(raw).getTime()
    if (!Number.isFinite(timestamp) || timestamp <= Date.now()) return null
    return new Date(timestamp)
  })()

  const saveLLMSettings = () => {
    updateSettingsMutation.mutate({
      ...formState,
      updated_at: new Date().toISOString(),
    })
  }

  const testAudioTranscription = async () => {
    setAudioModalOpen(true)
  }

  const handleSave = () => {
    const filteredChannels = formState.channels.filter(ch => ch.trim() !== '')
    updateSettingsMutation.mutate({
      ...formState,
      channels: filteredChannels,
      updated_at: new Date().toISOString(),
    })
  }

  if (isLoading) {
    return (
      <div style={{ textAlign: 'center', padding: 'var(--space-12)' }}>
        <p style={{ fontFamily: 'var(--font-display)', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--text-secondary)' }}>
          ЗАГРУЗКА НАСТРОЕК...
        </p>
      </div>
    )
  }

  return (
    <>
      <header className="hero">
        <h1 className="hero__title">НАСТРОЙКИ СИСТЕМЫ</h1>
        <p className="hero__lead">
          Глобальные настройки для всех ботов: целевые каналы, режим отладки, промпты для AI и дополнительные функции.
        </p>
      </header>

      <nav className="quick-tiles">
        <a className="quick-tile cursor-target" href="#channels">
          <span className="quick-tile__label">ШАГ 1</span>
          <span className="quick-tile__title">ЦЕЛЕВЫЕ КАНАЛЫ</span>
          <p className="quick-tile__desc">Список стримеров, куда боты будут заходить</p>
        </a>
        <a className="quick-tile quick-tile--primary cursor-target" href="#debug">
          <span className="quick-tile__label">ТЕСТИРОВАНИЕ</span>
          <span className="quick-tile__title">РЕЖИМ ОТЛАДКИ</span>
          <p className="quick-tile__desc">Проверка ботов без отправки в реальный чат</p>
        </a>
        <a className="quick-tile cursor-target" href="#llm">
          <span className="quick-tile__label">LLM</span>
          <span className="quick-tile__title">ПРОВАЙДЕР AI</span>
          <p className="quick-tile__desc">Настройка подключения к языковой модели</p>
        </a>
        <a className="quick-tile cursor-target" href="#prompts">
          <span className="quick-tile__label">AI</span>
          <span className="quick-tile__title">ПРОМПТЫ И ПОВЕДЕНИЕ</span>
          <p className="quick-tile__desc">Настройка характера и ограничений нейросети</p>
        </a>
        <a className="quick-tile cursor-target" href="#advanced">
          <span className="quick-tile__label">ДОПОЛНИТЕЛЬНО</span>
          <span className="quick-tile__title">ПРОДВИНУТЫЕ ФУНКЦИИ</span>
          <p className="quick-tile__desc">Аудио-подвязка, ротация ботов между стримерами</p>
        </a>
      </nav>

      <section className="card" id="channels">
        <h2>ЦЕЛЕВЫЕ КАНАЛЫ TWITCH</h2>
        <p className="hint">Укажите список стримеров, в чаты которых будут заходить боты. Можно указать несколько каналов.</p>

        {formState.debug_mode && formState.debug_channel && (
          <div className="alert" style={{ marginTop: 'var(--space-4)', background: 'rgba(255, 165, 0, 0.1)', borderColor: 'rgba(255, 165, 0, 0.3)' }}>
            ⚠️ <strong>ВНИМАНИЕ:</strong> Режим отладки активен с тестовым каналом <strong>"{formState.debug_channel}"</strong>. Этот канал переопределяет список ниже.
          </div>
        )}

        <div style={{ marginTop: 'var(--space-6)' }}>
          {formState.channels.map((channel, index) => (
            <div key={index} className="field" style={{ display: 'flex', gap: 'var(--space-2)', alignItems: 'end' }}>
              <div style={{ flex: 1 }}>
                <label htmlFor={`channel-${index}`}>КАНАЛ {index + 1}</label>
                <input
                  id={`channel-${index}`}
                  type="text"
                  value={channel}
                  onChange={(e) => updateChannel(index, e.target.value)}
                  placeholder="streamer_name"
                />
              </div>
              {formState.channels.length > 1 && (
                <button
                  type="button"
                  className="btn danger cursor-target"
                  onClick={() => removeChannel(index)}
                  style={{ marginBottom: '0' }}
                >
                  ✕ УДАЛИТЬ
                </button>
              )}
            </div>
          ))}
          <button type="button" className="btn ghost cursor-target" onClick={addChannel}>
            + ДОБАВИТЬ КАНАЛ
          </button>
        </div>
      </section>

      <section className="card" id="debug">
        <h2>РЕЖИМ ОТЛАДКИ</h2>
        <p className="hint">
          В режиме отладки боты подключаются к указанному стриму, но сообщения отображаются только в панели управления и не отправляются в реальный чат.
          Это позволяет проверить работу ботов без риска бана.
        </p>

        <div className="field" style={{ marginTop: 'var(--space-6)' }}>
          <label className="switch-row">
            <Switch
              checked={formState.debug_mode}
              onCheckedChange={(checked) => setFormState(prev => ({ ...prev, debug_mode: checked }))}
            />
            <span style={{ color: formState.debug_mode ? 'var(--text-primary)' : 'var(--text-secondary)' }}>
              ВКЛЮЧИТЬ РЕЖИМ ОТЛАДКИ
            </span>
          </label>
        </div>

        {formState.debug_mode && (
          <div className="field">
            <label htmlFor="debug-channel">ТЕСТОВЫЙ КАНАЛ</label>
            <input
              id="debug-channel"
              type="text"
              value={formState.debug_channel}
              onChange={(e) => setFormState(prev => ({ ...prev, debug_channel: e.target.value }))}
              placeholder="test_streamer"
            />
            <p className="hint">Укажите канал для тестирования (боты будут "подключаться", но не отправлять сообщения)</p>
          </div>
        )}

        {formState.debug_mode && (
          <div className="alert">
            <strong>РЕЖИМ ОТЛАДКИ АКТИВЕН!</strong> Все сообщения ботов будут отображаться только в панели управления.
            {formState.debug_channel && (
              <div style={{ marginTop: '8px', paddingTop: '8px', borderTop: '1px solid rgba(255,255,255,0.1)' }}>
                ⚠️ Тестовый канал <strong>"{formState.debug_channel}"</strong> переопределяет основной список каналов
              </div>
            )}
          </div>
        )}
      </section>

      <GridBeam
        rows={3}
        cols={4}
        colorVariant="neon"
        theme="dark"
        active
        borderRadius={8}
        className="card"
        id="llm"
      >
        <div style={{ padding: 'var(--space-6)' }}>
          <h2>НАСТРОЙКИ LLM ПРОВАЙДЕРА</h2>
          <p className="hint">
            Подключение к OpenRouter для генерации сообщений ботов. Сначала используется бесплатная модель,
            платный fallback включается только при необходимости.
          </p>

        <div className="field" style={{ marginTop: 'var(--space-6)' }}>
          <label htmlFor="llm-provider">ПРОВАЙДЕР</label>
          <select
            id="llm-provider"
            value={formState.llm_provider}
            onChange={(e) => setFormState(prev => ({ ...prev, llm_provider: e.target.value }))}
          >
            <option value="openrouter">OpenRouter</option>
          </select>
        </div>

        <div className="field">
          <label htmlFor="llm-api-url">API URL</label>
          <input
            id="llm-api-url"
            type="text"
            value={formState.llm_api_url}
            onChange={(e) => setFormState(prev => ({ ...prev, llm_api_url: e.target.value }))}
            placeholder="https://openrouter.ai/api/v1"
          />
          <p className="hint">Адрес API endpoint провайдера</p>
        </div>

        <div className="field">
          <label htmlFor="llm-model">БЕСПЛАТНАЯ МОДЕЛЬ</label>
          <input
            id="llm-model"
            type="text"
            value={formState.llm_model}
            onChange={(e) => setFormState(prev => ({ ...prev, llm_model: e.target.value }))}
            placeholder="openrouter/free"
          />
          <p className="hint">Используется в первую очередь</p>
        </div>

        <div className="field">
          <label htmlFor="llm-paid-model">ПЛАТНАЯ МОДЕЛЬ (FALLBACK)</label>
          <input
            id="llm-paid-model"
            type="text"
            value={formState.llm_paid_model}
            onChange={(e) => setFormState(prev => ({ ...prev, llm_paid_model: e.target.value }))}
            placeholder="z-ai/glm-5.3-flash"
          />
          <p className="hint">Включается, когда бесплатная модель недоступна или исчерпана</p>
        </div>

        <div className="field">
          <label className="switch-row">
            <Switch
              checked={formState.llm_paid_enabled}
              disabled={paidEnabledOverridden}
              onCheckedChange={(checked) => setFormState(prev => ({ ...prev, llm_paid_enabled: checked }))}
            />
            <span style={{ color: formState.llm_paid_enabled ? 'var(--text-primary)' : 'var(--text-secondary)' }}>
              ВКЛЮЧИТЬ ПЛАТНЫЙ FALLBACK
            </span>
          </label>
          {paidEnabledOverridden && (
            <p className="hint">Переопределено окружением</p>
          )}
        </div>

        <div className="field">
          <label htmlFor="llm-paid-daily-cap">ДНЕВНОЙ ЛИМИТ РАСХОДОВ, $</label>
          <input
            id="llm-paid-daily-cap"
            type="number"
            min="0"
            step="0.01"
            value={formState.llm_paid_daily_cap_usd}
            disabled={paidCapOverridden}
            readOnly={paidCapOverridden}
            onChange={(e) => setFormState(prev => ({ ...prev, llm_paid_daily_cap_usd: parseFloat(e.target.value) || 0 }))}
          />
          <p className="hint">0 или меньше — без лимита</p>
          {paidCapOverridden && (
            <p className="hint">Переопределено окружением</p>
          )}
        </div>

        <div className="field">
          <label htmlFor="llm-api-key">API KEY</label>
          <input
            id="llm-api-key"
            type="password"
            value={formState.llm_api_key}
            onChange={(e) => setFormState(prev => ({ ...prev, llm_api_key: e.target.value }))}
            placeholder={formState.llm_api_key_set ? '•••••••• (already set)' : 'sk-or-v1-...'}
          />
          <p className="hint">
            <span
              style={{
                display: 'inline-block',
                padding: '2px 8px',
                marginRight: '8px',
                borderRadius: 'var(--radius-sm)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                fontSize: '0.7rem',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
              }}
            >
              {apiKeySourceLabel}
            </span>
            Оставьте пустым, чтобы сохранить текущий ключ. Ключ берётся из OPENROUTER_API_KEY или базы данных;
            переменная окружения имеет приоритет.
          </p>
        </div>

        <div className="alert" style={{ marginTop: 'var(--space-4)' }}>
          <strong>OpenRouter:</strong> Сначала используется бесплатная модель «openrouter/free».
          Если бесплатные запросы исчерпаны или недоступны, включается платный fallback
          «{formState.llm_paid_model}», расход которого ограничен дневным лимитом.
          <br/>
          Ключ берётся из переменной окружения <code>OPENROUTER_API_KEY</code> или из базы данных.
        </div>

        <div style={{ marginTop: 'var(--space-6)', display: 'flex', gap: 'var(--space-4)', flexWrap: 'wrap' }}>
          <button
            type="button"
            className="btn ghost cursor-target"
            onClick={testLLMConnection}
            disabled={testingConnection}
          >
            {testingConnection ? '⟳ ПРОВЕРКА...' : '🔌 ПРОВЕРИТЬ ПОДКЛЮЧЕНИЕ'}
          </button>
          <button
            type="button"
            className="btn ghost cursor-target"
            onClick={testAudioTranscription}
          >
            🎤 ТЕСТ ТРАНСКРИПЦИИ
          </button>
          <button
            type="button"
            className="btn primary cursor-target"
            onClick={saveLLMSettings}
            disabled={updateSettingsMutation.isPending}
          >
            {updateSettingsMutation.isPending ? '⟳ СОХРАНЕНИЕ...' : '💾 СОХРАНИТЬ НАСТРОЙКИ LLM'}
          </button>
        </div>

        {llmStatus && (
          <div
            style={{
              marginTop: 'var(--space-4)',
              padding: 'var(--space-4)',
              border: '1px solid var(--border-default)',
              borderRadius: 'var(--radius-sm)',
              background: 'var(--bg-surface)',
              display: 'flex',
              flexWrap: 'wrap',
              gap: 'var(--space-4)',
              fontFamily: 'var(--font-mono)',
              fontSize: '0.75rem',
              color: 'var(--text-secondary)',
            }}
          >
            <span>
              {llmStatus.available ? 'Подключение: OK' : 'Подключение: ошибка'}
              {' • '}
              {llmStatus.provider || 'Unknown'}
              {' • '}
              {llmStatus.free_model || llmStatus.model || 'N/A'}
            </span>
            <span>
              Генерация: {llmStatus.generation_available ? 'доступна' : 'недоступна'}
            </span>
            <span>
              Расход сегодня: ${Number(llmStatus.paid_spent_today_usd ?? 0).toFixed(2)}
            </span>
            <span>
              Бесплатных запросов осталось: {llmStatus.free_requests_remaining ?? 0}
              {' из '}
              {llmStatus.free_requests_limit ?? 0}
            </span>
            <span>
              Платный fallback: {llmStatus.paid_enabled ? 'включён' : 'выключен'}
              {' • лимит $'}
              {Number(llmStatus.paid_daily_cap_usd ?? 0).toFixed(2)}
              {'/день'}
            </span>
            {cooldownUntil && (
              <span style={{ color: 'var(--state-warning)' }}>
                Бесплатная модель на паузе до{' '}
                {cooldownUntil.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}
              </span>
            )}
            {!llmStatus.generation_available && (llmStatus.last_error || llmStatus.error) && (
              <span style={{ color: 'var(--state-warning)' }}>
                Причина: {llmStatus.last_error || llmStatus.error}
              </span>
            )}
          </div>
        )}

        </div>
      </GridBeam>

      <section className="card" id="twitch-api">
        <h2>TWITCH API (ОПЦИОНАЛЬНО)</h2>
        <p className="hint">
          Настройте Twitch API для получения информации о категории/игре стрима. Это позволит ботам генерировать более релевантные сообщения.
          Получить Client ID и Secret можно на <a href="https://dev.twitch.tv/console/apps" target="_blank" rel="noopener noreferrer" style={{ color: 'var(--accent-purple)' }}>dev.twitch.tv/console/apps</a>
        </p>

        <div className="field" style={{ marginTop: 'var(--space-6)' }}>
          <label htmlFor="twitch-client-id">TWITCH CLIENT ID</label>
          <input
            id="twitch-client-id"
            type="text"
            value={formState.twitch_client_id}
            onChange={(e) => setFormState(prev => ({ ...prev, twitch_client_id: e.target.value }))}
            placeholder="ваш_client_id"
          />
          <p className="hint">Client ID из Twitch Developer Console</p>
        </div>

        <div className="field">
          <label htmlFor="twitch-client-secret">TWITCH CLIENT SECRET</label>
          <input
            id="twitch-client-secret"
            type="password"
            value={formState.twitch_client_secret}
            onChange={(e) => setFormState(prev => ({ ...prev, twitch_client_secret: e.target.value }))}
            placeholder="ваш_client_secret"
          />
          <p className="hint">Client Secret из Twitch Developer Console (показывается только один раз при создании)</p>
        </div>

        <div className="alert" style={{ marginTop: 'var(--space-4)' }}>
          <strong>Что это дает:</strong> Боты смогут получать информацию о текущей игре/категории стрима и генерировать более релевантные сообщения.
          Если не настроить, боты будут работать без этой информации.
        </div>
      </section>

      <GridBeam
        rows={2}
        cols={3}
        colorVariant="neon"
        theme="dark"
        active
        borderRadius={8}
        className="card"
        id="prompts"
      >
        <div style={{ padding: 'var(--space-6)' }}>
          <h2>ПРОМПТЫ И ПОВЕДЕНИЕ AI</h2>
          <p className="hint">
            Настройте глобальные промпты для всех ботов. Позитивный промпт указывает, что боты должны делать, негативный — чего избегать.
          </p>

        <div className="field-grid" style={{ marginTop: 'var(--space-6)' }}>
          <div className="field">
            <label htmlFor="positive-prompt">ПОЗИТИВНЫЙ ПРОМПТ</label>
            <textarea
              id="positive-prompt"
              value={formState.positive_prompt}
              onChange={(e) => setFormState(prev => ({ ...prev, positive_prompt: e.target.value }))}
              placeholder="Пиши короткие естественные сообщения, используй сленг, будь дружелюбным..."
              rows={5}
            />
            <p className="hint">Что боты ДОЛЖНЫ делать</p>
          </div>

          <div className="field">
            <label htmlFor="negative-prompt">НЕГАТИВНЫЙ ПРОМПТ</label>
            <textarea
              id="negative-prompt"
              value={formState.negative_prompt}
              onChange={(e) => setFormState(prev => ({ ...prev, negative_prompt: e.target.value }))}
              placeholder="Не используй эмодзи, не оскорбляй, не спамь, не повторяйся..."
              rows={5}
            />
            <p className="hint">Чего боты НЕ ДОЛЖНЫ делать</p>
          </div>
        </div>

        <div className="field">
          <label htmlFor="max-tokens">ОГРАНИЧЕНИЕ ПО СИМВОЛАМ (MAX TOKENS)</label>
          <input
            id="max-tokens"
            type="number"
            min="20"
            max="200"
            value={formState.max_tokens}
            onChange={(e) => setFormState(prev => ({ ...prev, max_tokens: parseInt(e.target.value) }))}
          />
          <p className="hint">Максимальная длина генерируемого сообщения (по умолчанию 60)</p>
        </div>
        </div>
      </GridBeam>

      <GridBeam
        rows={3}
        cols={2}
        colorVariant="neon"
        theme="dark"
        active
        borderRadius={8}
        className="card"
        id="advanced"
      >
        <div style={{ padding: 'var(--space-6)' }}>
          <h2>ПРОДВИНУТЫЕ ФУНКЦИИ</h2>

        <h3 style={{ marginTop: 'var(--space-8)' }}>АУДИО/ВИДЕО ПОДВЯЗКА К СТРИМУ</h3>
        <p className="hint">
          Боты могут анализировать аудио со стрима (через транскрипцию) для понимания контекста и генерации релевантных сообщений.
          Требует дополнительную настройку API для транскрипции.
        </p>

        <div className="field">
          <label className="switch-row">
            <Switch
              checked={formState.audio_enabled}
              onCheckedChange={(checked) => setFormState(prev => ({ ...prev, audio_enabled: checked }))}
            />
            <span style={{ color: formState.audio_enabled ? 'var(--text-primary)' : 'var(--text-secondary)' }}>
              ВКЛЮЧИТЬ АНАЛИЗ АУДИО СО СТРИМА
            </span>
          </label>
        </div>

        {formState.audio_enabled && (
          <div className="alert" style={{ marginTop: 'var(--space-4)' }}>
            <strong>ВКЛЮЧЕНО:</strong> Backend будет периодически захватывать аудио стрима и отправлять фрагменты
            в локальный STT API (OpenAI-compatible endpoint). Укажите переменные `AUDIO_STT_URL`, `AUDIO_STT_MODEL`
            и убедитесь, что доступны `streamlink` и `ffmpeg`.
          </div>
        )}

        <h3 style={{ marginTop: 'var(--space-8)' }}>РОТАЦИЯ БОТОВ МЕЖДУ СТРИМЕРАМИ</h3>
        <p className="hint">
          Автоматическое переключение ботов между разными каналами по расписанию. Помогает избежать банов и создать видимость активности на нескольких стримах.
        </p>

        <div className="field">
          <label className="switch-row">
            <Switch
              checked={formState.rotation_enabled}
              onCheckedChange={(checked) => setFormState(prev => ({ ...prev, rotation_enabled: checked }))}
            />
            <span style={{ color: formState.rotation_enabled ? 'var(--text-primary)' : 'var(--text-secondary)' }}>
              ВКЛЮЧИТЬ РОТАЦИЮ БОТОВ
            </span>
          </label>
        </div>

        {formState.rotation_enabled && (
          <div className="field">
            <label htmlFor="rotation-interval">ИНТЕРВАЛ РОТАЦИИ (МИНУТЫ)</label>
            <input
              id="rotation-interval"
              type="number"
              min="10"
              max="1440"
              value={formState.rotation_interval}
              onChange={(e) => setFormState(prev => ({ ...prev, rotation_interval: parseInt(e.target.value) }))}
            />
            <p className="hint">Как часто боты будут переключаться между каналами</p>
          </div>
        )}
        </div>
      </GridBeam>

      <div className="form-actions">
        <button
          className="btn primary"
          onClick={handleSave}
          disabled={updateSettingsMutation.isPending}
        >
          {updateSettingsMutation.isPending ? '⟳ СОХРАНЕНИЕ...' : '💾 СОХРАНИТЬ ВСЕ НАСТРОЙКИ'}
        </button>
        <button className="btn ghost" onClick={() => window.location.reload()}>
          ↻ СБРОСИТЬ ИЗМЕНЕНИЯ
        </button>
      </div>

      {/* Settings Log Panel */}
      <SettingsLogPanel maxHeight={250} collapsed={true} />

      {/* Toast notifications */}
      {saveSuccess && (
        <div style={{
          position: 'fixed',
          bottom: 'var(--space-8)',
          right: 'var(--space-8)',
          background: 'var(--bg-surface)',
          border: '2px solid var(--state-success)',
          color: 'var(--text-primary)',
          padding: 'var(--space-4) var(--space-6)',
          borderRadius: 'var(--radius-sm)',
          boxShadow: 'var(--neon-glow)',
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--space-3)',
          zIndex: 1000,
          animation: 'slideIn 0.3s ease-out',
          fontFamily: 'var(--font-display)'
        }}>
          <span style={{ fontSize: '1.5rem', color: 'var(--state-success)' }}>✓</span>
          <div>
            <div style={{ fontWeight: '700', marginBottom: 'var(--space-1)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
              НАСТРОЙКИ СОХРАНЕНЫ!
            </div>
            <div style={{ fontSize: '0.75rem', opacity: 0.9, color: 'var(--text-secondary)' }}>
              Изменения применены успешно
            </div>
          </div>
        </div>
      )}

      {saveError && (
        <div style={{
          position: 'fixed',
          bottom: 'var(--space-8)',
          right: 'var(--space-8)',
          background: 'var(--bg-surface)',
          border: '2px solid var(--state-error)',
          color: 'var(--text-primary)',
          padding: 'var(--space-4) var(--space-6)',
          borderRadius: 'var(--radius-sm)',
          boxShadow: '0 0 8px rgba(255,77,109,0.5)',
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--space-3)',
          zIndex: 1000,
          animation: 'slideIn 0.3s ease-out',
          fontFamily: 'var(--font-display)'
        }}>
          <span style={{ fontSize: '1.5rem', color: 'var(--state-error)' }}>✗</span>
          <div>
            <div style={{ fontWeight: '700', marginBottom: 'var(--space-1)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
              ОШИБКА СОХРАНЕНИЯ
            </div>
            <div style={{ fontSize: '0.75rem', opacity: 0.9, color: 'var(--text-secondary)' }}>
              Попробуйте еще раз
            </div>
          </div>
        </div>
      )}

      <AudioTestModal
        isOpen={audioModalOpen}
        onClose={() => setAudioModalOpen(false)}
        channel={formState.debug_channel}
      />

      <style>{`
        @keyframes slideIn {
          from {
            transform: translateX(100%);
            opacity: 0;
          }
          to {
            transform: translateX(0);
            opacity: 1;
          }
        }
      `}</style>
    </>
  )
}
