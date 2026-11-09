import type { ReactNode } from 'react'
import clsx from 'clsx'

export type BadgeTone = 'neutral' | 'success' | 'warning' | 'error' | 'accent'

interface BadgeProps {
  children: ReactNode
  tone?: BadgeTone
  className?: string
}

export function Badge({ children, tone = 'neutral', className }: BadgeProps) {
  return <span className={clsx('badge', `badge--${tone}`, className)}>{children}</span>
}
