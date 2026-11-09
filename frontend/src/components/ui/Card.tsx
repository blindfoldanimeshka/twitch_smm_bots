import type { ReactNode } from 'react'
import clsx from 'clsx'

interface CardProps {
  children?: ReactNode
  title?: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
  className?: string
}

export function Card({ children, title, subtitle, actions, className }: CardProps) {
  const hasHeader = Boolean(title || subtitle || actions)

  return (
    <section className={clsx('card', className)}>
      {hasHeader && (
        <header className="card__header">
          <div>
            {title && <h3 className="card__title">{title}</h3>}
            {subtitle && <p className="card__subtitle">{subtitle}</p>}
          </div>
          {actions && <div className="card__actions">{actions}</div>}
        </header>
      )}
      <div className="card__body">{children}</div>
    </section>
  )
}
