import type { ReactNode } from 'react'
import clsx from 'clsx'

export type AlertVariant = 'info' | 'success' | 'warning' | 'error'

interface AlertProps {
  children: ReactNode
  variant?: AlertVariant
  title?: ReactNode
  className?: string
}

export function Alert({ children, variant = 'info', title, className }: AlertProps) {
  return (
    <div
      className={clsx('alert', `alert--${variant}`, className)}
      role={variant === 'error' ? 'alert' : 'status'}
    >
      {title && <span className="alert__title">{title}</span>}
      <div>{children}</div>
    </div>
  )
}
