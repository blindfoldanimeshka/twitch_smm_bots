import { Component, useState, type ErrorInfo, type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { WebSocketProvider } from './hooks/useWebSocket'
import { LiveDataBridge } from './components/LiveDataBridge'
import { Sidebar, type Page } from './components/layout/Sidebar'
import { NavigationProvider } from './components/layout/navigation'
import { Topbar } from './components/layout/Topbar'
import Dashboard from './pages/Dashboard'
import Stream from './pages/Stream'
import Bots from './pages/Bots'
import Settings from './pages/Settings'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
})

const PAGE_TITLES: Record<Page, string> = {
  dashboard: 'Дашборд',
  stream: 'Стрим',
  bots: 'Боты',
  settings: 'Настройки',
}

interface ErrorBoundaryState {
  hasError: boolean
  error?: Error
}

class ErrorBoundary extends Component<{ children: ReactNode }, ErrorBoundaryState> {
  constructor(props: { children: ReactNode }) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('ErrorBoundary caught:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="error-screen">
          <div className="error-screen__card">
            <h1 className="error-screen__title">Что-то пошло не так</h1>
            <p className="error-screen__message">{this.state.error?.message}</p>
            <button
              type="button"
              className="btn btn--primary error-screen__button"
              onClick={() => window.location.reload()}
            >
              Перезагрузить
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

function App() {
  const [currentPage, setCurrentPage] = useState<Page>('dashboard')

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <WebSocketProvider>
          {/* One owner of live invalidation for every page. */}
          <LiveDataBridge />
          <NavigationProvider value={setCurrentPage}>
            <div className="app-shell">
              <Sidebar currentPage={currentPage} onNavigate={setCurrentPage} />
              <div className="app-main">
                <Topbar title={PAGE_TITLES[currentPage]} onNavigate={setCurrentPage} />
                <main className="app-content">
                  {currentPage === 'dashboard' && <Dashboard />}
                  {currentPage === 'stream' && <Stream />}
                  {currentPage === 'bots' && <Bots />}
                  {currentPage === 'settings' && <Settings />}
                </main>
              </div>
            </div>
          </NavigationProvider>
        </WebSocketProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  )
}

export default App
