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
    mutationFn: async (data: BotFormData) => {
      console.log('🚀 [CREATE BOT] Отправка запроса:', data)
      try {
        const response = await api.createBot(data)
        console.log('✅ [CREATE BOT] Успешный ответ:', response)
        return response
      } catch (error: any) {
        console.error('❌ [CREATE BOT] Ошибка:', error)
        console.error('❌ [CREATE BOT] Детали ошибки:', {
          message: error.message,
          response: error.response?.data,
          status: error.response?.status,
        })
        throw error
      }
    },
    onSuccess: (data) => {
      console.log('✅ [CREATE BOT] onSuccess вызван, данные:', data)
      queryClient.invalidateQueries({ queryKey: ['bots'] })
      resetForm()
    },
    onError: (error: any) => {
      console.error('❌ [CREATE BOT] onError вызван:', error)
      alert(`Ошибка создания бота: ${error.response?.data?.error || error.message}`)
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
    console.log('📝 [FORM SUBMIT] Форма отправлена')
    console.log('📝 [FORM SUBMIT] Данные формы:', formData)
    console.log('📝 [FORM SUBMIT] Режим редактирования:', editingBot ? 'Да' : 'Нет')

    if (editingBot) {
      console.log('✏️ [UPDATE BOT] Обновление бота:', editingBot)
      updateBotMutation.mutate({ id: editingBot, data: formData })
    } else {
      console.log('➕ [CREATE BOT] Создание нового бота')
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
        <h1 className="hero__title">УПРАВЛЕНИЕ АККАУНТАМИ</h1>
        <p className="hero__lead">
          Добавляйте, редактируйте и удаляйте аккаунты ботов. Для каждого бота укажите Twitch логин, пароль и настройки поведения.
        </p>
      </header>

      <div className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-6)' }}>
          <h2>СПИСОК БОТОВ ({bots.length})</h2>
          <button
            className="btn primary"
            onClick={() => setShowForm(!showForm)}
          >
            {showForm ? '✕ ОТМЕНИТЬ' : '+ ДОБАВИТЬ БОТА'}
          </button>
        </div>

        {showForm && (
          <form onSubmit={handleSubmit} style={{ marginBottom: 'var(--space-8)', padding: 'var(--space-6)', background: 'var(--bg-base)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-default)' }}>
            <h3>{editingBot ? 'РЕДАКТИРОВАТЬ БОТА' : 'НОВЫЙ БОТ'}</h3>

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
                {editingBot ? '💾 СОХРАНИТЬ' : '+ СОЗДАТЬ БОТА'}
              </button>
              <button type="button" className="btn ghost" onClick={resetForm}>
                ✕ ОТМЕНИТЬ
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
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>@{bot.twitch_login}</div>
                </div>
                <span className={`bot-card__status bot-card__status--${bot.status}`}>
                  {bot.status === 'running' ? '● РАБОТАЕТ' : '○ ОСТАНОВЛЕН'}
                </span>
              </div>

              <div className="bot-card__info">
                <div>ID: <code>{bot.id}</code></div>
                <div>ЛИЧНОСТЬ: {bot.personality}</div>
                <div>ЗАДЕРЖКА: {bot.min_delay_sec}-{bot.max_delay_sec} сек</div>
                <div>ЛИМИТ: {bot.rate_limit_per_min} сообщ/мин</div>
              </div>

              <div className="bot-card__actions">
                <button
                  className="btn ghost"
                  onClick={() => handleEdit(bot)}
                >
                  ✎ РЕДАКТИРОВАТЬ
                </button>
                <button
                  className="btn danger"
                  onClick={() => handleDelete(bot.id)}
                >
                  ✕ УДАЛИТЬ
                </button>
              </div>
            </div>
          ))}
        </div>

        {bots.length === 0 && !showForm && (
          <div style={{ textAlign: 'center', padding: 'var(--space-12)', color: 'var(--text-muted)' }}>
            <p style={{ fontFamily: 'var(--font-display)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
              НЕТ БОТОВ. ДОБАВЬТЕ ПЕРВОГО БОТА ДЛЯ НАЧАЛА РАБОТЫ.
            </p>
          </div>
        )}
      </div>
    </>
  )
}
