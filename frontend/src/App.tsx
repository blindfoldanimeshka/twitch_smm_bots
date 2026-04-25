import React from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Navigation from './components/Navigation'
import { StatusBar } from './components/StatusBar'
import { BubbleBackground } from './components/BubbleBackground'
import Dashboard from './pages/Dashboard'
import Bots from './pages/Bots'
import Settings from './pages/Settings'
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

function App() {
  const [currentPage, setCurrentPage] = React.useState<Page>('dashboard')

  return (
    <QueryClientProvider client={queryClient}>
      <div className="app">
        <BubbleBackground interactive={true} />
        <Navigation currentPage={currentPage} onNavigate={setCurrentPage as (page: string) => void} />
        <StatusBar onNavigateToSettings={() => setCurrentPage('settings')} />
        <main className="page">
          {currentPage === 'dashboard' && <Dashboard />}
          {currentPage === 'bots' && <Bots />}
          {currentPage === 'settings' && <Settings />}
        </main>
      </div>
    </QueryClientProvider>
  )
}

export default App
