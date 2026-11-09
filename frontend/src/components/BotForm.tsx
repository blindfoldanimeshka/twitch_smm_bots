import type { CSSProperties, FormEvent } from 'react'
import { Alert } from './ui/Alert'
import { BotFormField } from './BotFormField'
import type { BotFormData, TouchedFields, ValidationErrors } from '../hooks/useBotForm'

const PERSONALITY_OPTIONS = [
  { value: 'веселый_фанат', label: 'Веселый фанат' },
  { value: 'токсик', label: 'Токсик' },
  { value: 'нейтрал', label: 'Нейтрал' },
  { value: 'хайпер', label: 'Хайпер' },
]

interface BotFormProps {
  formData: BotFormData
  errors: ValidationErrors
  touchedFields: TouchedFields
  isEditing: boolean
  onChange: (name: keyof BotFormData, value: string | number) => void
  onBlur: (name: keyof BotFormData) => void
  onSubmit: (event: FormEvent) => void
}

const GRID_STYLE: CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
  gap: 12,
}

/** Add/edit bot form. Rendered inside BotModal; the footer buttons submit it by id. */
export function BotForm({
  formData,
  errors,
  touchedFields,
  isEditing,
  onChange,
  onBlur,
  onSubmit,
}: BotFormProps) {
  return (
    <form id="bot-form" onSubmit={onSubmit} noValidate>
      <div style={{ display: 'grid', gap: 12 }}>
        <div style={GRID_STYLE}>
          <BotFormField
            name="id"
            label="ID бота"
            value={formData.id}
            onChange={onChange}
            onBlur={() => onBlur('id')}
            error={errors.id}
            touched={touchedFields.id}
            required
            disabled={isEditing}
            placeholder="bot_001"
            hint={isEditing ? 'ID нельзя изменить' : 'Буквы, цифры, _ и -'}
          />
          <BotFormField
            name="name"
            label="Имя бота"
            value={formData.name}
            onChange={onChange}
            onBlur={() => onBlur('name')}
            error={errors.name}
            touched={touchedFields.name}
            required
            placeholder="Мой первый бот"
          />
        </div>

        <div style={GRID_STYLE}>
          <BotFormField
            name="twitch_login"
            label="Twitch логин"
            value={formData.twitch_login}
            onChange={onChange}
            onBlur={() => onBlur('twitch_login')}
            error={errors.twitch_login}
            touched={touchedFields.twitch_login}
            required
            placeholder="my_bot_account"
            hint="Логин аккаунта Twitch (4–25 символов)"
          />
          <BotFormField
            name="personality"
            label="Личность"
            type="select"
            value={formData.personality}
            onChange={onChange}
            options={PERSONALITY_OPTIONS}
            required
          />
        </div>

        <BotFormField
          name="password"
          label="OAuth токен"
          type="password"
          value={formData.password}
          onChange={onChange}
          onBlur={() => onBlur('password')}
          error={errors.password}
          touched={touchedFields.password}
          required={!isEditing}
          placeholder={isEditing ? 'оставьте пустым, чтобы не менять' : 'oauth:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'}
          hint={
            isEditing
              ? 'Введите новый токен, только если хотите его сменить'
              : 'OAuth токен аккаунта Twitch (префикс oauth: или без него)'
          }
        />

        <div style={GRID_STYLE}>
          <BotFormField
            name="min_delay_sec"
            label="Мин. задержка, сек"
            type="number"
            value={formData.min_delay_sec}
            onChange={onChange}
            onBlur={() => onBlur('min_delay_sec')}
            error={errors.min_delay_sec}
            touched={touchedFields.min_delay_sec}
            required
            min={10}
            max={300}
            step={5}
          />
          <BotFormField
            name="max_delay_sec"
            label="Макс. задержка, сек"
            type="number"
            value={formData.max_delay_sec}
            onChange={onChange}
            onBlur={() => onBlur('max_delay_sec')}
            error={errors.max_delay_sec}
            touched={touchedFields.max_delay_sec}
            required
            min={20}
            max={600}
            step={10}
          />
          <BotFormField
            name="rate_limit_per_min"
            label="Лимит, сообщ/мин"
            type="number"
            value={formData.rate_limit_per_min}
            onChange={onChange}
            onBlur={() => onBlur('rate_limit_per_min')}
            error={errors.rate_limit_per_min}
            touched={touchedFields.rate_limit_per_min}
            required
            min={1}
            max={20}
            step={1}
          />
        </div>

        <Alert variant="warning" title="Важно">
          Слишком частые сообщения могут привести к бану аккаунта. Рекомендуются
          задержки 45–180 сек и лимит 5–10 сообщений в минуту.
        </Alert>
      </div>
    </form>
  )
}
