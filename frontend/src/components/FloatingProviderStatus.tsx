import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Bot, Server, Zap, X } from 'lucide-react'
import { api, type LLMStatus } from '../api/client'

export function FloatingProviderStatus() {
  const [isOpen, setIsOpen] = useState(false)
  const [status, setStatus] = useState<LLMStatus | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    fetchStatus().catch(console.error)
    const interval = setInterval(() => fetchStatus().catch(console.error), 30000)
    return () => clearInterval(interval)
  }, [])

  const fetchStatus = async () => {
    setLoading(true)
    try {
      const response = await api.getLLMStatus()
      setStatus(response.data)
    } catch (err: any) {
      setStatus({
        provider: 'Unknown',
        available: false,
        error: err?.message || 'Failed to fetch status',
        model: 'N/A',
        url: 'N/A',
      })
    } finally {
      setLoading(false)
    }
  }

  const toggleOpen = () => {
    try {
      setIsOpen(prev => !prev)
    } catch (e) {
      console.error('Toggle error:', e)
    }
  }

  const truncateUrl = (url: string, maxLength: number = 35) => {
    if (url.length <= maxLength) return url
    return url.substring(0, maxLength) + '...'
  }

  const getProviderBadgeColor = (provider: string) => {
    switch (provider.toLowerCase()) {
      case 'lmstudio':
        return '#8b5cf6'
      case 'openai':
        return '#10a37f'
      default:
        return '#737373'
    }
  }

  return (
    <>
      <motion.button
        className="cursor-target"
        onClick={toggleOpen}
        style={{
          position: 'fixed',
          bottom: '32px',
          right: '32px',
          width: '56px',
          height: '56px',
          background: 'rgba(25, 25, 25, 0.8)',
          backdropFilter: 'blur(12px)',
          border: '1px solid #383838',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          cursor: 'pointer',
          zIndex: 999,
          transition: 'all 0.2s cubic-bezier(0.2, 0.8, 0.2, 1)',
        }}
        whileHover={{
          scale: 1.05,
          borderColor: '#8b5cf6',
          boxShadow: '0 0 20px rgba(139, 92, 246, 0.3)',
        }}
        whileTap={{ scale: 0.95 }}
      >
        <div style={{ position: 'relative' }}>
          <Bot size={24} color="#fafafa" />
          <motion.div
            animate={{
              scale: [1, 1.2, 1],
              opacity: [1, 0.6, 1],
            }}
            transition={{
              duration: 2,
              repeat: Infinity,
              ease: 'easeInOut',
            }}
            style={{
              position: 'absolute',
              top: '-2px',
              right: '-2px',
              width: '10px',
              height: '10px',
              borderRadius: '50%',
              background: status?.available ? '#10b981' : '#ef4444',
              boxShadow: status?.available
                ? '0 0 8px rgba(16, 185, 129, 0.6)'
                : '0 0 8px rgba(239, 68, 68, 0.6)',
            }}
          />
        </div>
      </motion.button>

      <AnimatePresence>
        {isOpen && (
          <>
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2 }}
              style={{
                position: 'fixed',
                inset: 0,
                background: 'rgba(10, 10, 10, 0.6)',
                backdropFilter: 'blur(4px)',
                zIndex: 1000,
              }}
              onClick={() => setIsOpen(false)}
            />

            <motion.div
              initial={{ opacity: 0, scale: 0.9, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.9, y: 20 }}
              transition={{ duration: 0.2, ease: [0.2, 0.8, 0.2, 1] }}
              style={{
                position: 'fixed',
                bottom: '100px',
                right: '32px',
                width: '380px',
                maxWidth: 'calc(100vw - 64px)',
                background: 'rgba(25, 25, 25, 0.95)',
                backdropFilter: 'blur(16px)',
                border: '1px solid #383838',
                zIndex: 1001,
                overflow: 'hidden',
              }}
            >
              <div
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  right: 0,
                  height: '2px',
                  background: `linear-gradient(90deg, transparent, ${getProviderBadgeColor(status?.provider || 'Unknown')}, transparent)`,
                }}
              >
                <motion.div
                  animate={{ x: ['-100%', '100%'] }}
                  transition={{
                    duration: 3,
                    repeat: Infinity,
                    ease: 'linear',
                  }}
                  style={{
                    width: '100%',
                    height: '100%',
                    background: `linear-gradient(90deg, transparent, ${getProviderBadgeColor(status?.provider || 'Unknown')}, transparent)`,
                  }}
                />
              </div>

              <div style={{ padding: '24px' }}>
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    marginBottom: '20px',
                  }}
                >
                  <h3
                    style={{
                      fontFamily: 'var(--font-display)',
                      fontSize: '0.9rem',
                      fontWeight: 700,
                      letterSpacing: '0.08em',
                      textTransform: 'uppercase',
                      color: '#fafafa',
                    }}
                  >
                    LLM Provider Status
                  </h3>
                  <button
                    onClick={() => setIsOpen(false)}
                    className="cursor-target"
                    style={{
                      background: 'transparent',
                      border: 'none',
                      cursor: 'pointer',
                      padding: '4px',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      transition: 'all 0.2s',
                    }}
                  >
                    <X size={20} color="#a1a1a1" />
                  </button>
                </div>

                {loading && !status ? (
                  <div
                    style={{
                      textAlign: 'center',
                      padding: '20px',
                      color: '#a1a1a1',
                      fontSize: '0.85rem',
                      fontFamily: 'var(--font-display)',
                      textTransform: 'uppercase',
                      letterSpacing: '0.05em',
                    }}
                  >
                    Loading...
                  </div>
                ) : status ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '12px',
                        padding: '12px',
                        background: '#0a0a0a',
                        border: '1px solid #383838',
                      }}
                    >
                      <Server size={20} color={getProviderBadgeColor(status.provider)} />
                      <div style={{ flex: 1 }}>
                        <div
                          style={{
                            fontSize: '0.75rem',
                            color: '#717171',
                            textTransform: 'uppercase',
                            letterSpacing: '0.05em',
                            marginBottom: '4px',
                          }}
                        >
                          Provider
                        </div>
                        <div
                          style={{
                            display: 'inline-block',
                            padding: '4px 10px',
                            background: getProviderBadgeColor(status.provider),
                            color: '#0a0a0a',
                            fontSize: '0.8rem',
                            fontWeight: 700,
                            textTransform: 'uppercase',
                            letterSpacing: '0.05em',
                          }}
                        >
                          {status.provider}
                        </div>
                      </div>
                    </div>

                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '12px',
                        padding: '12px',
                        background: '#0a0a0a',
                        border: '1px solid #383838',
                      }}
                    >
                      <Bot size={20} color="#a1a1a1" />
                      <div style={{ flex: 1 }}>
                        <div
                          style={{
                            fontSize: '0.75rem',
                            color: '#717171',
                            textTransform: 'uppercase',
                            letterSpacing: '0.05em',
                            marginBottom: '4px',
                          }}
                        >
                          Model
                        </div>
                        <div
                          style={{
                            fontSize: '0.85rem',
                            color: '#fafafa',
                            fontFamily: 'var(--font-mono)',
                          }}
                        >
                          {status.model || 'Auto-detect'}
                        </div>
                      </div>
                    </div>

                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '12px',
                        padding: '12px',
                        background: '#0a0a0a',
                        border: '1px solid #383838',
                      }}
                    >
                      <Zap
                        size={20}
                        color={status.available ? '#10b981' : '#ef4444'}
                      />
                      <div style={{ flex: 1 }}>
                        <div
                          style={{
                            fontSize: '0.75rem',
                            color: '#717171',
                            textTransform: 'uppercase',
                            letterSpacing: '0.05em',
                            marginBottom: '4px',
                          }}
                        >
                          Connection
                        </div>
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: '8px',
                          }}
                        >
                          <motion.div
                            animate={{
                              scale: [1, 1.2, 1],
                              opacity: [1, 0.6, 1],
                            }}
                            transition={{
                              duration: 2,
                              repeat: Infinity,
                              ease: 'easeInOut',
                            }}
                            style={{
                              width: '8px',
                              height: '8px',
                              borderRadius: '50%',
                              background: status.available ? '#10b981' : '#ef4444',
                              boxShadow: status.available
                                ? '0 0 8px rgba(16, 185, 129, 0.6)'
                                : '0 0 8px rgba(239, 68, 68, 0.6)',
                            }}
                          />
                          <span
                            style={{
                              fontSize: '0.85rem',
                              color: status.available ? '#10b981' : '#ef4444',
                              fontWeight: 600,
                              textTransform: 'uppercase',
                              letterSpacing: '0.05em',
                            }}
                          >
                            {status.available ? 'Connected' : 'Disconnected'}
                          </span>
                        </div>
                        {status.error && (
                          <div
                            style={{
                              fontSize: '0.75rem',
                              color: '#ef4444',
                              marginTop: '4px',
                            }}
                          >
                            {status.error}
                          </div>
                        )}
                      </div>
                    </div>

                    <div
                      style={{
                        padding: '12px',
                        background: '#0a0a0a',
                        border: '1px solid #383838',
                      }}
                    >
                      <div
                        style={{
                          fontSize: '0.75rem',
                          color: '#717171',
                          textTransform: 'uppercase',
                          letterSpacing: '0.05em',
                          marginBottom: '4px',
                        }}
                      >
                        API Endpoint
                      </div>
                      <div
                        style={{
                          fontSize: '0.8rem',
                          color: '#a1a1a1',
                          fontFamily: 'var(--font-mono)',
                          wordBreak: 'break-all',
                        }}
                      >
                        {truncateUrl(status.url)}
                      </div>
                    </div>

                    <button
                      onClick={fetchStatus}
                      disabled={loading}
                      className="cursor-target"
                      style={{
                        width: '100%',
                        padding: '12px',
                        background: 'transparent',
                        border: '1px solid #383838',
                        color: '#a1a1a1',
                        fontFamily: 'var(--font-display)',
                        fontSize: '0.85rem',
                        fontWeight: 700,
                        letterSpacing: '0.08em',
                        textTransform: 'uppercase',
                        cursor: loading ? 'not-allowed' : 'pointer',
                        transition: 'all 0.2s',
                        opacity: loading ? 0.5 : 1,
                      }}
                      onMouseEnter={(e) => {
                        if (!loading) {
                          e.currentTarget.style.background = '#262626'
                          e.currentTarget.style.borderColor = '#8b5cf6'
                          e.currentTarget.style.color = '#8b5cf6'
                        }
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.background = 'transparent'
                        e.currentTarget.style.borderColor = '#383838'
                        e.currentTarget.style.color = '#a1a1a1'
                      }}
                    >
                      {loading ? '⟳ Refreshing...' : '↻ Refresh Status'}
                    </button>
                  </div>
                ) : null}
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </>
  )
}
