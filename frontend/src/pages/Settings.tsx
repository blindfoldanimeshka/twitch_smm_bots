import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, type Settings as SettingsType } from '../api/client'

export default function Settings() {
  const queryClient = useQueryClient()
  const [channels, setChannels] = useState<string[]>([''])
  const [debugMode, setDebugMode] = useState(false)
  const [debugChannel, setDebugChannel] = useState('')
  const [positivePrompt, setPositivePrompt] = useState('')
  const [negativePrompt, setNegativePrompt] = useState('')
  const [maxTokens, setMaxTokens] = useState(60)
  const [audioEnabled, setAudioEnabled] = useState(false)
  const [rotationEnabled, setRotationEnabled] = useState(false)
  const [rotationInterval, setRotationInterval] = useState(60)

  const { data: settings, isLoading } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => {
      const res = await api.getSettings()
      return res.data
    },
  })

  useEffect(() => {
    if (settings) {
      setChannels(settings.channels.length > 0 ? settings.channels : [''])
      setDebugMode(settings.debug_mode)
      setDebugChannel(settings.debug_channel)
      setPositivePrompt(settings.positive_prompt)
      setNegativePrompt(settings.negative_prompt)
      setMaxTokens(settings.max_tokens)
      setAudioEnabled(settings.audio_enabled)
      setRotationEnabled(settings.rotation_enabled)
      setRotationInterval(settings.rotation_interval)
    }
  }, [settings])

  const updateSettingsMutation = useMutation({
    mutationFn: (data: SettingsType) => api.updateSettings(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      alert('Настройки сохранены!')
    },
    onError: () => {
      alert('Ошибка при сохранении настроек')
    },
  })

  const addChannel = () => {
    setChannels([...channels, ''])
  }

  const removeChannel = (index: number) => {
    setChannels(channels.filter((_, i) => i !== index))
  }

  const updateChannel = (index: number, value: string) => {
    const newChannels = [...channels]
    newChannels[index] = value
    setChannels(newChannels)
  }

  const handleSave = () => {
    const filteredChannels = channels.filter(ch => ch.trim() !== '')

    updateSettingsMutation.mutate({
      id: 'default',
      channels: filteredChannels,
      debug_mode: debugMode,
      debug_channel: debugChannel,
      positive_prompt: positivePrompt,
      negative_prompt: negativePrompt,
      max_tokens: maxTokens,
      audio_enabled: audioEnabled,
      rotation_enabled: rotationEnabled,
      rotation_interval: rotationInterval,
      updated_at: new Date().toISOString(),
    })
  }

  if (isLoading) {
    return (
      <div style={{ textAlign: 'center', padding: '3rem' }}>
        <p>Загрузка настроек...</p>
      </div>
    )
  }

  return (
    <>
      <header className="hero">
        <h1 className="hero__title">Настройки системы</h1>
        <p className="hero__lead">
          Глобальные настройки для всех ботов: целевые каналы, режим отладки, промпты для AI и дополнительные функции.
        </p>
      </header>

      <nav className="quick-tiles">
        <a className="quick-tile" href="#channels">
          <span className="quick-tile__label">Шаг 1</span>
          <span className="quick-tile__title">Целевые каналы</span>
          <p className="quick-tile__desc">Список стримеров, куда боты будут заходить</p>
        </a>
        <a className="quick-tile quick-tile--primary" href="#debug">
          <span className="quick-tile__label">Тестирование</span>
          <span className="quick-tile__title">Режим отладки</span>
          <p className="quick-tile__desc">Проверка ботов без отправки в реальный чат</p>
        </a>
        <a className="quick-tile" href="#prompts">
          <span className="quick-tile__label">AI</span>
          <span className="quick-tile__title">Промпты и поведение</span>
          <p className="quick-tile__desc">Настройка характера и ограничений нейросети</p>
        </a>
        <a className="quick-tile" href="#advanced">
          <span className="quick-tile__label">Дополнительно</span>
          <span className="quick-tile__title">Продвинутые функции</span>
          <p className="quick-tile__desc">Аудио-подвязка, ротация ботов между стримерами</p>
        </a>
      </nav>

      <section className="card" id="channels">
        <h2>Целевые каналы Twitch</h2>
        <p className="hint">Укажите список стримеров, в чаты которых будут заходить боты. Можно указать несколько каналов.</p>

        <div style={{ marginTop: '1.5rem' }}>
          {channels.map((channel, index) => (
            <div key={index} className="field" style={{ display: 'flex', gap: '0.5rem', alignItems: 'end' }}>
              <div style={{ flex: 1 }}>
                <label htmlFor={`channel-${index}`}>Канал {index + 1}</label>
                <input
                  id={`channel-${index}`}
                  type="text"
                  value={channel}
                  onChange={(e) => updateChannel(index, e.target.value)}
                  placeholder="streamer_name"
                />
              </div>
              {channels.length > 1 && (
                <button
                  type="button"
                  className="btn danger"
                  onClick={() => removeChannel(index)}
                  style={{ marginBottom: '0' }}
                >
                  Удалить
                </button>
              )}
            </div>
          ))}
          <button type="button" className="btn ghost" onClick={addChannel}>
            + Добавить канал
          </button>
        </div>
      </section>

      <section className="card" id="debug">
        <h2>Режим отладки</h2>
        <p className="hint">
          В режиме отладки боты подключаются к указанному стриму, но сообщения отображаются только в панели управления и не отправляются в реальный чат.
          Это позволяет проверить работу ботов без риска бана.
        </p>

        <div className="field" style={{ marginTop: '1.5rem' }}>
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={debugMode}
              onChange={(e) => setDebugMode(e.target.checked)}
            />
            <span>Включить режим отладки</span>
          </label>
        </div>

        {debugMode && (
          <div className="field">
            <label htmlFor="debug-channel">Тестовый канал</label>
            <input
              id="debug-channel"
              type="text"
              value={debugChannel}
              onChange={(e) => setDebugChannel(e.target.value)}
              placeholder="test_streamer"
            />
            <p className="hint">Укажите канал для тестирования (боты будут "подключаться", но не отправлять сообщения)</p>
          </div>
        )}

        {debugMode && (
          <div className="alert">
            <strong>Режим отладки активен!</strong> Все сообщения ботов будут отображаться только в панели управления.
          </div>
        )}
      </section>

      <section className="card" id="prompts">
        <h2>Промпты и поведение AI</h2>
        <p className="hint">
          Настройте глобальные промпты для всех ботов. Позитивный промпт указывает, что боты должны делать, негативный — чего избегать.
        </p>

        <div className="field-grid" style={{ marginTop: '1.5rem' }}>
          <div className="field">
            <label htmlFor="positive-prompt">Позитивный промпт</label>
            <textarea
              id="positive-prompt"
              value={positivePrompt}
              onChange={(e) => setPositivePrompt(e.target.value)}
              placeholder="Пиши короткие естественные сообщения, используй сленг, будь дружелюбным..."
              rows={5}
            />
            <p className="hint">Что боты ДОЛЖНЫ делать</p>
          </div>

          <div className="field">
            <label htmlFor="negative-prompt">Негативный промпт</label>
            <textarea
              id="negative-prompt"
              value={negativePrompt}
              onChange={(e) => setNegativePrompt(e.target.value)}
              placeholder="Не используй эмодзи, не оскорбляй, не спамь, не повторяйся..."
              rows={5}
            />
            <p className="hint">Чего боты НЕ ДОЛЖНЫ делать</p>
          </div>
        </div>

        <div className="field">
          <label htmlFor="max-tokens">Ограничение по символам (max tokens)</label>
          <input
            id="max-tokens"
            type="number"
            min="20"
            max="200"
            value={maxTokens}
            onChange={(e) => setMaxTokens(parseInt(e.target.value))}
          />
          <p className="hint">Максимальная длина генерируемого сообщения (по умолчанию 60)</p>
        </div>
      </section>

      <section className="card" id="advanced">
        <h2>Продвинутые функции</h2>

        <h3>Аудио/видео подвязка к стриму</h3>
        <p className="hint">
          Боты могут анализировать аудио со стрима (через транскрипцию) для понимания контекста и генерации релевантных сообщений.
          Требует дополнительную настройку API для транскрипции.
        </p>

        <div className="field">
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={audioEnabled}
              onChange={(e) => setAudioEnabled(e.target.checked)}
            />
            <span>Включить анализ аудио со стрима</span>
          </label>
        </div>

        {audioEnabled && (
          <div className="alert" style={{ marginTop: '1rem' }}>
            <strong>Включено:</strong> Backend будет периодически захватывать аудио стрима и отправлять фрагменты
            в локальный STT API (OpenAI-compatible endpoint). Укажите переменные `AUDIO_STT_URL`, `AUDIO_STT_MODEL`
            и убедитесь, что доступны `streamlink` и `ffmpeg`.
          </div>
        )}

        <h3 style={{ marginTop: '2rem' }}>Ротация ботов между стримерами</h3>
        <p className="hint">
          Автоматическое переключение ботов между разными каналами по расписанию. Помогает избежать банов и создать видимость активности на нескольких стримах.
        </p>

        <div className="field">
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={rotationEnabled}
              onChange={(e) => setRotationEnabled(e.target.checked)}
            />
            <span>Включить ротацию ботов</span>
          </label>
        </div>

        {rotationEnabled && (
          <div className="field">
            <label htmlFor="rotation-interval">Интервал ротации (минуты)</label>
            <input
              id="rotation-interval"
              type="number"
              min="10"
              max="1440"
              value={rotationInterval}
              onChange={(e) => setRotationInterval(parseInt(e.target.value))}
            />
            <p className="hint">Как часто боты будут переключаться между каналами</p>
          </div>
        )}
      </section>

      <div className="form-actions">
        <button className="btn primary" onClick={handleSave}>
          Сохранить все настройки
        </button>
        <button className="btn ghost" onClick={() => window.location.reload()}>
          Сбросить изменения
        </button>
      </div>
    </>
  )
}
