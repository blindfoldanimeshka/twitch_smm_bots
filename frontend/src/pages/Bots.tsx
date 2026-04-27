import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import { BotFormModal } from '../components/BotFormModal'
import { BotForm } from '../components/BotForm'
import { BotLogPanel } from '../components/BotLogPanel'
import { useBotForm } from '../hooks/useBotForm'
import type { BotFormData } from '../hooks/useBotForm'

export default function Bots() {
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [editingBot, setEditingBot] = useState<any | null>(null)
  const [expandedLogs, setExpandedLogs] = useState<Set<string>>(new Set())

  const botForm = useBotForm(editingBot)

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
    onError: (error: any) => {
      alert(`Ошибка обновления бота: ${error.response?.data?.error || error.message}`)
    },
  })

  const deleteBotMutation = useMutation({
    mutationFn: (id: string) => api.deleteBot(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bots'] })
    },
    onError: (error: any) => {
      alert(`Ошибка удаления бота: ${error.response?.data?.error || error.message}`)
    },
  })

  const resetForm = () => {
    botForm.reset()
    setShowForm(false)
    setEditingBot(null)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    console.log('📝 [FORM SUBMIT] Форма отправлена')
    console.log('📝 [FORM SUBMIT] Данные формы:', botForm.formData)
    console.log('📝 [FORM SUBMIT] Режим редактирования:', editingBot ? 'Да' : 'Нет')

    if (editingBot) {
      console.log('✏️ [UPDATE BOT] Обновление бота:', editingBot.id)
      updateBotMutation.mutate({ id: editingBot.id, data: botForm.formData })
    } else {
      console.log('➕ [CREATE BOT] Создание нового бота')
      createBotMutation.mutate(botForm.formData)
    }
  }

  const handleEdit = (bot: any) => {
    setEditingBot(bot)
    setShowForm(true)
  }

  const handleDelete = (id: string) => {
    if (confirm(`Удалить бота ${id}? Это действие необратимо.`)) {
      deleteBotMutation.mutate(id)
    }
  }

  const handleAddNew = () => {
    setEditingBot(null)
    botForm.reset()
    setShowForm(true)
  }

  const toggleLogs = (botId: string) => {
    setExpandedLogs(prev => {
      const newSet = new Set(prev)
      if (newSet.has(botId)) {
        newSet.delete(botId)
      } else {
        newSet.add(botId)
      }
      return newSet
    })
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
            onClick={handleAddNew}
          >
            + ДОБАВИТЬ БОТА
          </button>
        </div>

        <BotFormModal
          isOpen={showForm}
          onClose={resetForm}
          title={editingBot ? 'РЕДАКТИРОВАТЬ БОТА' : 'НОВЫЙ БОТ'}
        >
          <BotForm
            formData={botForm.formData}
            errors={botForm.errors}
            touchedFields={botForm.touchedFields}
            isValid={botForm.isValid}
            isSubmitting={createBotMutation.isPending || updateBotMutation.isPending}
            isEditing={!!editingBot}
            showAdvanced={botForm.showAdvanced}
            onShowAdvancedToggle={() => botForm.setShowAdvanced(!botForm.showAdvanced)}
            onChange={botForm.handleChange}
            onBlur={botForm.handleBlur}
            onSubmit={handleSubmit}
            onCancel={resetForm}
          />
        </BotFormModal>

        <div className="bot-list">
          {bots.map((bot: any) => (
            <div key={bot.id}>
              <div className={`bot-card ${bot.status === 'running' ? 'bot-card--running' : ''}`}>
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
                  <button
                    className={`btn ghost ${expandedLogs.has(bot.id) ? 'active' : ''}`}
                    onClick={() => toggleLogs(bot.id)}
                    style={{
                      background: expandedLogs.has(bot.id) ? 'var(--accent-primary)' : 'transparent',
                      color: expandedLogs.has(bot.id) ? 'var(--bg-surface)' : 'var(--text-primary)',
                    }}
                  >
                    📋 ЛОГИ
                  </button>
                </div>
              </div>

              {/* Log Panel - shown when expanded */}
              {expandedLogs.has(bot.id) && (
                <BotLogPanel botId={bot.id} maxHeight={300} />
              )}
            </div>
          ))}
        </div>

        {bots.length === 0 && (
          <div style={{ textAlign: 'center', padding: 'var(--space-12)', color: 'var(--text-muted)' }}>
            <p style={{ fontFamily: 'var(--font-display)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
              НЕТ БОТОВ. ДОБАВЬТЕ ПЕРВОГО БОТА ЧТОБЫ НАЧАТЬ.
            </p>
          </div>
        )}
      </div>
    </>
  )
}
