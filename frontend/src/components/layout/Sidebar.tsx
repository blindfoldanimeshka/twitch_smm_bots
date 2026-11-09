import clsx from 'clsx'
import { Bot, LayoutDashboard, MonitorPlay, Settings, type LucideIcon } from 'lucide-react'

export type Page = 'dashboard' | 'stream' | 'bots' | 'settings'

interface NavItem {
  page: Page
  label: string
  icon: LucideIcon
}

export const NAV_ITEMS: NavItem[] = [
  { page: 'dashboard', label: 'Дашборд', icon: LayoutDashboard },
  { page: 'stream', label: 'Стрим', icon: MonitorPlay },
  { page: 'bots', label: 'Боты', icon: Bot },
  { page: 'settings', label: 'Настройки', icon: Settings },
]

interface SidebarProps {
  currentPage: Page
  onNavigate: (page: Page) => void
}

export function Sidebar({ currentPage, onNavigate }: SidebarProps) {
  return (
    <aside className="sidebar">
      <div className="sidebar__brand">
        <span className="sidebar__brand-mark" aria-hidden="true">
          TS
        </span>
        <span className="sidebar__brand-name">Twitch SMM</span>
      </div>

      <nav className="sidebar__nav" aria-label="Основная навигация">
        {NAV_ITEMS.map(({ page, label, icon: Icon }) => (
          <button
            key={page}
            type="button"
            className={clsx('sidebar__nav-item', currentPage === page && 'sidebar__nav-item--active')}
            onClick={() => onNavigate(page)}
            aria-current={currentPage === page ? 'page' : undefined}
          >
            <Icon size={18} strokeWidth={2} aria-hidden="true" />
            <span>{label}</span>
          </button>
        ))}
      </nav>

      <div className="sidebar__footer">
        <span className="sidebar__version">версия v{__APP_VERSION__}</span>
      </div>
    </aside>
  )
}
