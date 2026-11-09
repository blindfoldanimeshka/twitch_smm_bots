import type { ChangeEvent } from 'react'
import { Input } from './ui/Input'
import { Select } from './ui/Select'
import type { BotFormData } from '../hooks/useBotForm'

export interface BotFormFieldOption {
  value: string
  label: string
}

interface BotFormFieldProps {
  name: keyof BotFormData
  label: string
  type?: 'text' | 'password' | 'number' | 'select'
  value: string | number
  onChange: (name: keyof BotFormData, value: string | number) => void
  onBlur?: () => void
  /** Shown only after the field was blurred (touched). */
  error?: string
  touched?: boolean
  hint?: string
  required?: boolean
  disabled?: boolean
  placeholder?: string
  min?: number
  max?: number
  step?: number
  options?: BotFormFieldOption[]
}

/** A single bot form field rendered with the shared Input/Select primitives. */
export function BotFormField({
  name,
  label,
  type = 'text',
  value,
  onChange,
  onBlur,
  error,
  touched,
  hint,
  required,
  disabled,
  placeholder,
  min,
  max,
  step,
  options,
}: BotFormFieldProps) {
  const showError = touched ? error : undefined

  const handleChange = (
    event: ChangeEvent<HTMLInputElement | HTMLSelectElement>,
  ) => {
    onChange(name, type === 'number' ? Number(event.target.value) : event.target.value)
  }

  return (
    <div style={{ display: 'grid', gap: 4 }}>
      {type === 'select' ? (
        <Select
          label={label}
          error={showError}
          options={options}
          value={String(value)}
          onChange={handleChange}
          onBlur={onBlur}
          required={required}
          disabled={disabled}
        />
      ) : (
        <Input
          label={label}
          type={type}
          value={value}
          onChange={handleChange}
          onBlur={onBlur}
          error={showError}
          required={required}
          disabled={disabled}
          placeholder={placeholder}
          min={min}
          max={max}
          step={step}
          autoComplete={type === 'password' ? 'new-password' : undefined}
        />
      )}
      {hint && (
        <span style={{ fontSize: 12, color: 'var(--muted)' }}>{hint}</span>
      )}
    </div>
  )
}
