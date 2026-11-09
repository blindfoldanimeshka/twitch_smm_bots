import { useId } from 'react'
import type { SelectHTMLAttributes } from 'react'
import clsx from 'clsx'

export interface SelectOption {
  value: string
  label: string
}

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string
  error?: string
  /** Convenience list; `children` are rendered instead when provided. */
  options?: SelectOption[]
}

export function Select({ label, error, options, id, className, children, ...rest }: SelectProps) {
  const autoId = useId()
  const selectId = id ?? autoId

  return (
    <div className="field">
      {label && (
        <label className="field__label" htmlFor={selectId}>
          {label}
        </label>
      )}
      <select id={selectId} className={clsx('select', className)} {...rest}>
        {options ? options.map((o) => <option key={o.value} value={o.value}>{o.label}</option>) : children}
      </select>
      {error && <span className="field__error">{error}</span>}
    </div>
  )
}
