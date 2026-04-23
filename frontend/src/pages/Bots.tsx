import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'

interface BotFormData {
  id: string
  twitch_login: string
  password: string
  name: string
  personality: string
  min_delay_sec: number
  max_delay_sec: number
  rate_limit_per_min: number
}

export default function Bots() {
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [editingBot, setEditingBot] = useState<string | null>(null)
  const [formData, setFormData] = useState<BotFormData>({
    id: '',
    twitch_login: '',
    password: '',
    name: '',
    personality: 'нейтрал',
    min_delay_sec: 45,
    max_delay_sec: 180,
    rate_limit_per_min: 10,
  })

  const { data: bots = [] } = useQuery({
    queryKey: ['bots'],
    queryFn: async () => {
      const res = await api.getBots()
      return res.data
    },
  })

  const createBotMutation = useMutation({
    mutationFn: (data: BotFormData) => api.createBot(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] })
      resetForm()
    },
  })

  const updateBotMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<BotFormData> }) =>
      api.updateBot(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] })
      resetForm()
    },
  })

  const deleteBotMutation = useMutation({
    mutationFn: (id: string) => api.deleteBot(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] })
    },
  })

  const resetForm = () => {
    setFormData({
      id: '',
      twitch_login: '',
      password: '',
      name: '',
      personality: 'нейтрал',
      min_delay_sec: 45,
      max_delay_sec: 180,
      rate_limit_per_min: 10,
    })
    setShowForm(false)
    setEditingBot(null)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (editingBot) {
      updateBotMutation.mutate({ id: editingBot, data: formData })
    } else {
      createBotMutation.mutate(formData)
    }
  }

  const handleEdit = (bot: any) => {
    setFormData({
      id: bot.id,
      twitch_login: bot.twitch_login,
      password: '',
      name: bot.name,
      personality: bot.personality,
      min_delay_sec: bot.min_delay_sec,
      max_delay_sec: bot.max_delay_sec,
      rate_limit_per_min: bot.rate_limit_per_min,
    })
    setEditingBot(bot.id)
    setShowForm(true)
  }

  const handleDelete = (id: string) => {
    if (confirm(`Удалить бота ${id}? Это действие необратимо.`)) {
      deleteBotMutation.mutate(id)
    }
  }

  return (
    <>
      <header className="hero">
        <h1 className="hero__title">Управление аккаунтами ботов</h1>
        <p className="hero__lead">
          Добавляйте, редактируйте и удаляйте аккаунты ботов. Для каждого бота укажите Twitch логин, OAuth токен и настройки поведения.
        </p>
      </header>

      <div className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
          <h2>Список ботов ({bots.length})</h2>
          <button
            className="btn primary"
            onClick={() => setShowForm(!showForm)}
          >
            {showForm ? 'Отменить' : '+ Добавить бота'}
          </button>
        </div>

        {showForm && (
          <form onSubmit={handleSubmit} style={{ marginBottom: '2rem', padding: '1.5rem', background: 'var(--color-bg)', borderRadius: 'var(--radius)' }}>
            <h3>{editingBot ? 'Редактировать бота' : 'Новый бот'}</h3>

            <div className="field-grid">
              <div className="field">
                <label htmlFor="bot-id">ID бота *</label>
                <input
                  id="bot-id"
                  type="text"
                  value={formData.id}
                  onChange={(e) => setFormData({ ...formData, id: e.target.value })}
                  disabled={!!editingBot}
                  required
                  placeholder="bot1"
                />
                <p className="hint">Уникальный идентификатор (только латиница, цифры, подчеркивание)</p>
              </div>

              <div className="field">
                <label htmlFor="bot-name">Имя бота *</label>
                <input
                  id="bot-name"
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  required
                  placeholder="Alex_Gamer"
                />
                <p className="hint">Отображаемое имя</p>
              </div>

              <div className="field">
                <label htmlFor="bot-login">Twitch логин *</label>
                <input
                  id="bot-login"
                  type="text"
                  value={formData.twitch_login}
                  onChange={(e) => setFormData({ ...formData, twitch_login: e.target.value })}
                  required
                  placeholder="your_bot_account"
                />
                <p className="hint">Логин Twitch аккаунта</p>
              </div>

              <div className="field">
                <label htmlFor="bot-token">Пароль *</label>
                <input
                  id="bot-token"
                  type="password"
                  value={formData.password}
                  onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                  required
                  placeholder="Пароль от Twitch аккаунта"
                />
                <p className="hint">Пароль от вашего Twitch аккаунта</p>
              </div>

              <div className="field">
                <label htmlFor="bot-personality">Личность</label>
                <select
                  id="bot-personality"
                  value={formData.personality}
                  onChange={(e) => setFormData({ ...formData, personality: e.target.value })}
                >
                  <option value="веселый_фанат">Веселый фанат</option>
                  <option value="токсик">Токсик</option>
                  <option value="нейтрал">Нейтрал</option>
                  <option value="хайпер">Хайпер</option>
                </select>
              </div>

              <div className="field">
                <label htmlFor="bot-rate">Лимит сообщений/мин</label>
                <input
                  id="bot-rate"
                  type="number"
                  min="1"
                  max="20"
                  value={formData.rate_limit_per_min}
                  onChange={(e) => setFormData({ ...formData, rate_limit_per_min: parseInt(e.target.value) })}
                />
                <p className="hint">Максимум сообщений в минуту</p>
              </div>

              <div className="field">
                <label htmlFor="bot-min-delay">Мин. задержка (сек)</label>
                <input
                  id="bot-min-delay"
                  type="number"
                  min="10"
                  max="300"
                  value={formData.min_delay_sec}
                  onChange={(e) => setFormData({ ...formData, min_delay_sec: parseInt(e.target.value) })}
                />
              </div>

              <div className="field">
                <label htmlFor="bot-max-delay">Макс. задержка (сек)</label>
                <input
                  id="bot-max-delay"
                  type="number"
                  min="10"
                  max="600"
                  value={formData.max_delay_sec}
                  onChange={(e) => setFormData({ ...formData, max_delay_sec: parseInt(e.target.value) })}
                />
              </div>
            </div>

            <div className="form-actions">
              <button type="submit" className="btn primary">
                {editingBot ? 'Сохранить изменения' : 'Создать бота'}
              </button>
              <button type="button" className="btn ghost" onClick={resetForm}>
                Отменить
              </button>
            </div>
          </form>
        )}

        <div className="bot-list">
          {bots.map((bot: any) => (
            <div key={bot.id} className={`bot-card ${bot.status === 'running' ? 'bot-card--running' : ''}`}>
              <div className="bot-card__header">
                <div>
                  <div className="bot-card__title">{bot.name}</div>
                  <div style={{ fontSize: '0.875rem', color: 'var(--color-text-muted)' }}>@{bot.twitch_login}</div>
                </div>
                <span className={`bot-card__status bot-card__status--${bot.status}`}>
                  {bot.status === 'running' ? 'Работает' : 'Остановлен'}
                </span>
              </div>

              <div className="bot-card__info">
                <div>ID: <code>{bot.id}</code></div>
                <div>Личность: {bot.personality}</div>
                <div>Задержка: {bot.min_delay_sec}-{bot.max_delay_sec} сек</div>
                <div>Лимит: {bot.rate_limit_per_min} сообщ/мин</div>
              </div>

              <div className="bot-card__actions">
                <button
                  className="btn ghost"
                  onClick={() => handleEdit(bot)}
                >
                  Редактировать
                </button>
                <button
                  className="btn danger"
                  onClick={() => handleDelete(bot.id)}
                >
                  Удалить
                </button>
              </div>
            </div>
          ))}
        </div>

        {bots.length === 0 && !showForm && (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>
            <p>Нет ботов. Добавьте первого бота для начала работы.</p>
          </div>
        )}
      </div>
    </>
  )
}
