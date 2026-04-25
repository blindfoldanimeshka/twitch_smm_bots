import React, { useState } from 'react'
import './BotFormField.css'

interface BotFormFieldProps {
  label: string
  name: string
  type?: 'text' | 'password' | 'number' | 'select'
  value: string | number
  onChange: (value: any) => void
  onBlur?: () => void
  error?: string
  touched?: boolean
  required?: boolean
  disabled?: boolean
  placeholder?: string
  hint?: string
  min?: number
  max?: number
  step?: number
  options?: { value: string; label: string; description?: string }[]
  icon?: React.ReactNode
}

export function BotFormField({
  label,
  name,
  type = 'text',
  value,
  onChange,
  onBlur,
  error,
  touched,
  required,
  disabled,
  placeholder,
  hint,
  min,
  max,
  step,
  options,
  icon,
}: BotFormFieldProps) {
  const [showPassword, setShowPassword] = useState(false)
  const [isFocused, setIsFocused] = useState(false)
  const hasError = touched && error
  const isValid = touched && !error && value

  const inputId = `bot-field-${name}`

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const newValue = type === 'number' ? Number(e.target.value) : e.target.value
    onChange(newValue)
  }

  const renderInput = () => {
    if (type === 'select' && options) {
      return (
        <select
          id={inputId}
          className="bot-field__input"
          value={value}
          onChange={handleInputChange}
          onBlur={() => {
            setIsFocused(false)
            onBlur?.()
          }}
          onFocus={() => setIsFocused(true)}
          disabled={disabled}
          required={required}
          aria-invalid={hasError ? 'true' : 'false'}
          aria-describedby={hasError ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined}
        >
          {options.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      )
    }

    return (
      <input
        id={inputId}
        className="bot-field__input"
        type={type === 'password' && showPassword ? 'text' : type}
        value={value}
        onChange={handleInputChange}
        onBlur={() => {
          setIsFocused(false)
          onBlur?.()
        }}
        onFocus={() => setIsFocused(true)}
        disabled={disabled}
        required={required}
        placeholder={placeholder}
        min={min}
        max={max}
        step={step}
        aria-invalid={hasError ? 'true' : 'false'}
        aria-describedby={hasError ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined}
      />
    )
  }

  return (
    <div
      className={`bot-field ${hasError ? 'bot-field--error' : ''} ${
        isValid ? 'bot-field--valid' : ''
      } ${isFocused ? 'bot-field--focused' : ''} ${disabled ? 'bot-field--disabled' : ''}`}
    >
      <label htmlFor={inputId} className="bot-field__label">
        {label}
        {required && <span className="bot-field__required" aria-label="обязательное поле">*</span>}
      </label>

      <div className="bot-field__input-wrapper">
        {icon && <div className="bot-field__icon">{icon}</div>}
        {renderInput()}

        {type === 'password' && (
          <button
            type="button"
            className="bot-field__toggle-password"
            onClick={() => setShowPassword(!showPassword)}
            aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
            tabIndex={-1}
          >
            {showPassword ? (
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <path
                  d="M3 3L21 21M10.5 10.677A2 2 0 1013.323 13.5M7.362 7.561C5.68 8.74 4.279 10.42 3 12c1.889 2.991 5.282 6 9 6 1.55 0 3.043-.523 4.395-1.35M12 6C15.718 6 19.111 9.01 21 12a15.66 15.66 0 01-2.119 2.798"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                />
              </svg>
            ) : (
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <path
                  d="M12 5C8.24 5 4.82 7.58 3 12c1.82 4.42 5.24 7 9 7s7.18-2.58 9-7c-1.82-4.42-5.24-7-9-7zm0 11.5a4.5 4.5 0 110-9 4.5 4.5 0 010 9zm0-7a2.5 2.5 0 100 5 2.5 2.5 0 000-5z"
                  fill="currentColor"
                />
              </svg>
            )}
          </button>
        )}

        {isValid && (
          <div className="bot-field__status bot-field__status--valid" aria-hidden="true">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
              <path
                d="M20 6L9 17L4 12"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </div>
        )}

        {hasError && (
          <div className="bot-field__status bot-field__status--error" aria-hidden="true">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="2" />
              <path d="M12 8V12M12 16H12.01" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </div>
        )}
      </div>

      {hasError && (
        <div id={`${inputId}-error`} className="bot-field__error" role="alert">
          {error}
        </div>
      )}

      {hint && !hasError && (
        <div id={`${inputId}-hint`} className="bot-field__hint">
          {hint}
        </div>
      )}

      {type === 'number' && (min !== undefined || max !== undefined) && (
        <div className="bot-field__range">
          <input
            type="range"
            min={min}
            max={max}
            step={step}
            value={value}
            onChange={handleInputChange}
            className="bot-field__slider"
            disabled={disabled}
            aria-label={`${label} слайдер`}
          />
          <div className="bot-field__range-labels">
            {min !== undefined && <span>{min}</span>}
            {max !== undefined && <span>{max}</span>}
          </div>
        </div>
      )}
    </div>
  )
}
