import { useId } from 'react'
import type { InputHTMLAttributes } from 'react'
import clsx from 'clsx'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
}

export function Input({
  label,
  error,
  id,
  className,
  'aria-describedby': describedBy,
  'aria-invalid': ariaInvalid,
  ...rest
}: InputProps) {
  const autoId = useId()
  const inputId = id ?? autoId
  const errorId = `${autoId}-error`

  return (
    <div className="field">
      {label && (
        <label className="field__label" htmlFor={inputId}>
          {label}
        </label>
      )}
      <input
        id={inputId}
        className={clsx('input', error && 'input--error', className)}
        {...rest}
        aria-invalid={error && ariaInvalid === undefined ? true : ariaInvalid}
        aria-describedby={error ? [describedBy, errorId].filter(Boolean).join(' ') : describedBy}
      />
      {error && <span id={errorId} className="field__error">{error}</span>}
    </div>
  )
}
