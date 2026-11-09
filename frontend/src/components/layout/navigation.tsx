import { createContext, useContext, type ReactNode } from 'react'
import type { Page } from './Sidebar'

type Navigate = (page: Page) => void

// The current page is App.tsx state; components that need to switch pages get
// the setter from here instead of poking the sidebar DOM.
const NavigationContext = createContext<Navigate | null>(null)

export function NavigationProvider({ value, children }: { value: Navigate; children: ReactNode }) {
  return <NavigationContext.Provider value={value}>{children}</NavigationContext.Provider>
}

/**
 * Returns the app-level navigate function. Throws when there is no provider:
 * a silently dead button would be worse than a visible error, and every
 * screen lives inside the shell.
 * The provider and its hook are one feature; splitting them into two files
 * would only add indirection.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function useAppNavigate(): Navigate {
  const navigate = useContext(NavigationContext)
  if (!navigate) {
    throw new Error('useAppNavigate: нет NavigationProvider — компонент вне оболочки приложения')
  }
  return navigate
}
