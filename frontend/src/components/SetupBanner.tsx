import type { ReactNode } from 'react'
import clsx from 'clsx'
import { Button } from './ui/Button'
import { useAppNavigate } from './layout/navigation'
import type { Page } from './layout/Sidebar'

type SetupBannerTone = 'info' | 'warning'

interface SetupBannerProps {
  /** `info` — calm neutral hint, `warning` — calm amber hint (still not an error). */
  tone?: SetupBannerTone
  title: string
  description?: ReactNode
  /** Sidebar page the action button leads to; omit to render no button. */
  actionPage?: Page
  actionLabel?: string
  className?: string
}

/**
 * A calm configuration hint, deliberately not styled as an error: missing
 * channel or API key is a setup state, not a failure.
 */
export function SetupBanner({
  tone = 'info',
  title,
  description,
  actionPage,
  actionLabel = 'Открыть настройки',
  className,
}: SetupBannerProps) {
  const navigate = useAppNavigate()

  return (
    <div className={clsx('alert', tone === 'warning' ? 'alert--warning' : 'alert--info', className)} role="status">
      <span className="alert__title">{title}</span>
      {description && <div>{description}</div>}
      {actionPage && (
        <div>
          <Button variant="secondary" size="sm" onClick={() => navigate(actionPage)}>
            {actionLabel}
          </Button>
        </div>
      )}
    </div>
  )
}
