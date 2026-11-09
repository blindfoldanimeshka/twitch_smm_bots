import type { ReactNode } from 'react'
import clsx from 'clsx'
import { Button } from './ui/Button'

// Navigation without a router: the current page lives in App.tsx state and is
// only reachable through the sidebar buttons, so a banner CTA activates the
// matching nav item. The sidebar renders on every page and viewport, so this
// is deterministic; when it is missing we stay silent instead of throwing.
const PAGE_LABELS = { bots: 'Боты', settings: 'Настройки' } as const

export type BannerPage = keyof typeof PAGE_LABELS

/** Activates the sidebar item of the given page (see comment above). */
export function navigateToPage(page: BannerPage): void {
  const label = PAGE_LABELS[page]
  const items = document.querySelectorAll<HTMLButtonElement>('.sidebar__nav-item')
  for (const item of items) {
    if (item.textContent?.includes(label)) {
      item.click()
      return
    }
  }
}

type SetupBannerTone = 'info' | 'warning'

interface SetupBannerProps {
  /** `info` — calm neutral hint, `warning` — calm amber hint (still not an error). */
  tone?: SetupBannerTone
  title: string
  description?: ReactNode
  /** Sidebar page the action button leads to; omit to render no button. */
  actionPage?: BannerPage
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
  return (
    <div className={clsx('alert', tone === 'warning' ? 'alert--warning' : 'alert--info', className)} role="status">
      <span className="alert__title">{title}</span>
      {description && <div>{description}</div>}
      {actionPage && (
        <div>
          <Button variant="secondary" size="sm" onClick={() => navigateToPage(actionPage)}>
            {actionLabel}
          </Button>
        </div>
      )}
    </div>
  )
}
