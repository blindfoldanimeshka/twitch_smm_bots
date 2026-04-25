import React from 'react'
import { BotFormField } from './BotFormField'
import type { BotFormData, ValidationErrors, TouchedFields } from '../hooks/useBotForm'
import './BotForm.css'

interface BotFormProps {
  formData: BotFormData
  errors: ValidationErrors
  touchedFields: TouchedFields
  isValid: boolean
  isSubmitting: boolean
  isEditing: boolean
  showAdvanced: boolean
  onShowAdvancedToggle: () => void
  onChange: (name: keyof BotFormData, value: any) => void
  onBlur: (name: keyof BotFormData) => void
  onSubmit: (e: React.FormEvent) => void
  onCancel: () => void
}

const PERSONALITY_OPTIONS = [
  {
    value: 'веселый_фанат',
    label: '🎉 Веселый фанат',
    description: 'Позитивный, поддерживающий, использует эмодзи',
  },
  {
    value: 'токсик',
    label: '😈 Токсик',
    description: 'Провокационный, саркастичный, троллит',
  },
  {
    value: 'нейтрал',
    label: '😐 Нейтрал',
    description: 'Спокойный, сдержанный, без эмоций',
  },
  {
    value: 'хайпер',
    label: '🔥 Хайпер',
    description: 'Энергичный, восторженный, много капса',
  },
]

export function BotForm({
  formData,
  errors,
  touchedFields,
  isValid,
  isSubmitting,
  isEditing,
  showAdvanced,
  onShowAdvancedToggle,
  onChange,
  onBlur,
  onSubmit,
  onCancel,
}: BotFormProps) {
  return (
    <form onSubmit={onSubmit} className="bot-form">
      <div className="bot-form__section">
        <h3 className="bot-form__section-title">
          <span className="bot-form__section-icon">🤖</span>
          Основная информация
        </h3>

        <div className="bot-form__grid">
          <BotFormField
            label="ID бота"
            name="id"
            value={formData.id}
            onChange={(value) => onChange('id', value)}
            onBlur={() => onBlur('id')}
            error={errors.id}
            touched={touchedFields.id}
            required
            disabled={isEditing}
            placeholder="bot_001"
            hint={isEditing ? 'ID нельзя изменить' : 'Уникальный идентификатор (буквы, цифры, _, -)'}
            icon={
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <path
                  d="M7 7h10M7 12h10M7 17h6"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                />
              </svg>
            }
          />

          <BotFormField
            label="Twitch логин"
            name="twitch_login"
            value={formData.twitch_login}
            onChange={(value) => onChange('twitch_login', value)}
            onBlur={() => onBlur('twitch_login')}
            error={errors.twitch_login}
            touched={touchedFields.twitch_login}
            required
            placeholder="my_bot_account"
            hint="Логин аккаунта Twitch (4-25 символов)"
            icon={
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <path
                  d="M16 8v8M12 11v5M8 14v3M6 3l-3 3v12h6v3l3-3h4l8-8V3H6z"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            }
          />
        </div>

        <BotFormField
          label="Пароль"
          name="password"
          type="password"
          value={formData.password}
          onChange={(value) => onChange('password', value)}
          onBlur={() => onBlur('password')}
          error={errors.password}
          touched={touchedFields.password}
          required={!isEditing}
          placeholder={isEditing ? 'Оставьте пустым, чтобы не менять' : '••••••••'}
          hint={isEditing ? 'Введите новый пароль только если хотите изменить' : 'Минимум 6 символов'}
          icon={
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
              <rect x="5" y="11" width="14" height="10" rx="2" stroke="currentColor" strokeWidth="2" />
              <path d="M8 11V7a4 4 0 118 0v4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
          }
        />

        <div className="bot-form__grid">
          <BotFormField
            label="Имя бота"
            name="name"
            value={formData.name}
            onChange={(value) => onChange('name', value)}
            onBlur={() => onBlur('name')}
            error={errors.name}
            touched={touchedFields.name}
            required
            placeholder="Мой первый бот"
            hint="Отображаемое имя для удобства"
            icon={
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <path
                  d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2M12 11a4 4 0 100-8 4 4 0 000 8z"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                />
              </svg>
            }
          />

          <BotFormField
            label="Личность"
            name="personality"
            type="select"
            value={formData.personality}
            onChange={(value) => onChange('personality', value)}
            onBlur={() => onBlur('personality')}
            required
            options={PERSONALITY_OPTIONS}
            hint="Стиль общения бота в чате"
          />
        </div>
      </div>

      <button
        type="button"
        className="bot-form__advanced-toggle"
        onClick={onShowAdvancedToggle}
        aria-expanded={showAdvanced}
      >
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          style={{ transform: showAdvanced ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 0.2s' }}
        >
          <path d="M6 9l6 6 6-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>
        {showAdvanced ? 'Скрыть' : 'Показать'} расширенные настройки
      </button>

      {showAdvanced && (
        <div className="bot-form__section bot-form__section--advanced">
          <h3 className="bot-form__section-title">
            <span className="bot-form__section-icon">⚙️</span>
            Расширенные настройки
          </h3>

          <BotFormField
            label="Минимальная задержка (сек)"
            name="min_delay_sec"
            type="number"
            value={formData.min_delay_sec}
            onChange={(value) => onChange('min_delay_sec', value)}
            onBlur={() => onBlur('min_delay_sec')}
            error={errors.min_delay_sec}
            touched={touchedFields.min_delay_sec}
            required
            min={10}
            max={300}
            step={5}
            hint="Минимальное время между сообщениями"
          />

          <BotFormField
            label="Максимальная задержка (сек)"
            name="max_delay_sec"
            type="number"
            value={formData.max_delay_sec}
            onChange={(value) => onChange('max_delay_sec', value)}
            onBlur={() => onBlur('max_delay_sec')}
            error={errors.max_delay_sec}
            touched={touchedFields.max_delay_sec}
            required
            min={20}
            max={600}
            step={10}
            hint="Максимальное время между сообщениями"
          />

          <BotFormField
            label="Лимит сообщений в минуту"
            name="rate_limit_per_min"
            type="number"
            value={formData.rate_limit_per_min}
            onChange={(value) => onChange('rate_limit_per_min', value)}
            onBlur={() => onBlur('rate_limit_per_min')}
            error={errors.rate_limit_per_min}
            touched={touchedFields.rate_limit_per_min}
            required
            min={1}
            max={20}
            step={1}
            hint="Рекомендуется 5-10 для безопасности"
          />

          <div className="bot-form__warning">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
              <path
                d="M12 9v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
              />
            </svg>
            <div>
              <strong>Важно:</strong> Слишком частые сообщения могут привести к бану аккаунта.
              Рекомендуется использовать задержки 45-180 сек и лимит 5-10 сообщений/мин.
            </div>
          </div>
        </div>
      )}

      <div className="bot-form__actions">
        <button
          type="button"
          className="btn secondary"
          onClick={onCancel}
          disabled={isSubmitting}
        >
          Отмена
        </button>
        <button
          type="submit"
          className="btn primary"
          disabled={!isValid || isSubmitting}
        >
          {isSubmitting ? (
            <>
              <span className="btn__spinner" />
              {isEditing ? 'Сохранение...' : 'Создание...'}
            </>
          ) : (
            <>{isEditing ? 'Сохранить изменения' : 'Создать бота'}</>
          )}
        </button>
      </div>
    </form>
  )
}
