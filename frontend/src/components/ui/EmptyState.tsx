import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'

interface EmptyStateProps {
  title: string
  description?: ReactNode
  icon?: LucideIcon
  action?: ReactNode
  className?: string
}

export function EmptyState({ title, description, icon: Icon, action, className }: EmptyStateProps) {
  return (
    <div className={className ? `empty-state ${className}` : 'empty-state'}>
      {Icon && <Icon size={28} className="empty-state__icon" aria-hidden="true" />}
      <span className="empty-state__title">{title}</span>
      {description && <p className="empty-state__description">{description}</p>}
      {action}
    </div>
  )
}
