import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import { motion, AnimatePresence } from 'framer-motion'

export function DebugModeNotification() {
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => {
      const res = await api.getSettings()
      return res.data
    },
    refetchInterval: 5000,
  })

  const isDebugMode = settings?.debug_mode || false

  return (
    <AnimatePresence>
      {isDebugMode && (
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -20 }}
          transition={{ duration: 0.3 }}
          className="fixed top-20 left-1/2 transform -translate-x-1/2 z-50"
        >
          <div className="bg-gradient-to-r from-yellow-500/20 to-orange-500/20 backdrop-blur-md border-2 border-yellow-500/50 rounded-lg px-6 py-3 shadow-2xl">
            <div className="flex items-center gap-3">
              <div className="flex items-center justify-center w-8 h-8 bg-yellow-500/30 rounded-full animate-pulse">
                <svg
                  className="w-5 h-5 text-yellow-300"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
              </div>
              <div className="text-left">
                <div className="text-yellow-300 font-bold text-sm uppercase tracking-wide">
                  РЕЖИМ ОТЛАДКИ АКТИВЕН!
                </div>
                <div className="text-yellow-200/80 text-xs mt-0.5">
                  Все сообщения ботов будут отображаться только в панели управления.
                </div>
              </div>
            </div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
