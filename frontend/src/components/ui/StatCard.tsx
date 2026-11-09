import type { ReactNode } from 'react'
import clsx from 'clsx'
import type { LucideIcon } from 'lucide-react'

type StatCardTone = 'neutral' | 'success' | 'warning' | 'error'

interface StatCardProps {
  title: string
  value: ReactNode
  hint?: ReactNode
  icon?: LucideIcon
  tone?: StatCardTone
  className?: string
}

export function StatCard({ title, value, hint, icon: Icon, tone = 'neutral', className }: StatCardProps) {
  return (
    <div className={clsx('stat-card', tone !== 'neutral' && `stat-card--${tone}`, className)}>
      <div className="stat-card__top">
        <span className="stat-card__title">{title}</span>
        {Icon && <Icon size={16} className="stat-card__icon" aria-hidden="true" />}
      </div>
      <span className="stat-card__value">{value}</span>
      {hint && <span className="stat-card__hint">{hint}</span>}
    </div>
  )
}
