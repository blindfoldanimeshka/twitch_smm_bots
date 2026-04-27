import React from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StatusBar } from './components/StatusBar'
import Dashboard from './pages/Dashboard'
import Bots from './pages/Bots'
import Settings from './pages/Settings'
import { BGPattern } from './components/ui/bg-pattern'
import { Dock } from './components/ui/dock'
import TargetCursor from './components/ui/TargetCursor'
import { PixelSnow } from './components/ui/PixelSnow'
import { FloatingProviderStatus } from './components/FloatingProviderStatus'
import { AnimatePresence, motion } from 'framer-motion'
import './styles/main.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
})

type Page = 'dashboard' | 'bots' | 'settings'

interface ErrorBoundaryState {
  hasError: boolean
  error?: Error
}

class ErrorBoundary extends React.Component<{ children: React.ReactNode }, ErrorBoundaryState> {
  constructor(props: { children: React.ReactNode }) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('ErrorBoundary caught:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex items-center justify-center min-h-screen bg-[#0a0a0a] text-white">
          <div className="text-center p-8">
            <h1 className="text-2xl font-bold mb-4">Something went wrong</h1>
            <p className="text-gray-400 mb-4">{this.state.error?.message}</p>
            <button 
              onClick={() => window.location.reload()}
              className="px-4 py-2 bg-purple-600 rounded hover:bg-purple-700"
            >
              Reload Page
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

function App() {
  const [currentPage, setCurrentPage] = React.useState<Page>('dashboard')

  const pageVariants = {
    initial: {
      opacity: 0,
      y: 20,
      scale: 0.98
    },
    animate: {
      opacity: 1,
      y: 0,
      scale: 1,
      transition: {
        duration: 0.4,
        ease: [0.22, 1, 0.36, 1] as any
      }
    },
    exit: {
      opacity: 0,
      y: -20,
      scale: 0.98,
      transition: {
        duration: 0.3,
        ease: [0.22, 1, 0.36, 1] as any
      }
    }
  }

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
      <TargetCursor
        targetSelector=".cursor-target"
        spinDuration={2}
        hideDefaultCursor={true}
        parallaxOn={true}
      />
      <div className="dark app relative min-h-screen pb-32">
        <PixelSnow
          color="#8b5cf6"
          flakeSize={0.008}
          minFlakeSize={1.0}
          pixelResolution={150}
          speed={0.8}
          density={0.15}
          direction={135}
          brightness={0.6}
          variant="round"
          className="fixed inset-0 z-0"
          style={{ pointerEvents: 'none' }}
        />
        <BGPattern
          variant="dots"
          mask="none"
          size={20}
          fill="rgba(115,115,115,0.15)"
          className="fixed inset-0 z-0"
        />
        <StatusBar onNavigateToSettings={() => setCurrentPage('settings')} />
        <main className="page relative z-10">
          <AnimatePresence mode="wait">
            {currentPage === 'dashboard' && (
              <motion.div
                key="dashboard"
                variants={pageVariants}
                initial="initial"
                animate="animate"
                exit="exit"
              >
                <Dashboard />
              </motion.div>
            )}
            {currentPage === 'bots' && (
              <motion.div
                key="bots"
                variants={pageVariants}
                initial="initial"
                animate="animate"
                exit="exit"
              >
                <Bots />
              </motion.div>
            )}
            {currentPage === 'settings' && (
              <motion.div
                key="settings"
                variants={pageVariants}
                initial="initial"
                animate="animate"
                exit="exit"
              >
                <Settings />
              </motion.div>
            )}
          </AnimatePresence>
        </main>
        <Dock currentPage={currentPage} onNavigate={setCurrentPage as (page: string) => void} />
        <FloatingProviderStatus />
      </div>
    </QueryClientProvider>
    </ErrorBoundary>
  )
}

export default App
