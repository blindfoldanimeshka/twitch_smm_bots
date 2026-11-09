import { useId } from 'react'
import type { InputHTMLAttributes } from 'react'
import clsx from 'clsx'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
}

export function Input({ label, error, id, className, ...rest }: InputProps) {
  const autoId = useId()
  const inputId = id ?? autoId

  return (
    <div className="field">
      {label && (
        <label className="field__label" htmlFor={inputId}>
          {label}
        </label>
      )}
      <input id={inputId} className={clsx('input', error && 'input--error', className)} {...rest} />
      {error && <span className="field__error">{error}</span>}
    </div>
  )
}
