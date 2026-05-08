import { useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { createPortal } from 'react-dom'
import { api } from '../api/client'
import type { Bot } from '../api/client'

interface LaunchBotModalProps {
  isOpen: boolean
  onClose: () => void
  bot: Bot
  onStart: (id: string) => void
}

export function LaunchBotModal({ isOpen, onClose, bot, onStart }: LaunchBotModalProps) {
  const [selectedOption, setSelectedOption] = useState<'without-llm' | 'with-llm' | null>(null)
  const [isStarting, setIsStarting] = useState(false)

  const handleLaunch = async () => {
    if (!selectedOption) return
    setIsStarting(true)
    const disableLlm = selectedOption === 'without-llm'
    try {
      await api.startBot(bot.id, disableLlm)
      onStart(bot.id)
    } catch (error) {
      alert(`Ошибка запуска: ${error}`)
    } finally {
      setIsStarting(false)
      onClose()
    }
  }

  if (!isOpen) return null

  return createPortal(
    <AnimatePresence mode="wait">
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            onClick={onClose}
            className="fixed inset-0 z-[10000] bg-black/70 backdrop-blur-sm cursor-pointer"
            initial={{ opacity: 0, backdropFilter: 'blur(0px)' }}
            animate={{
              opacity: 1,
              backdropFilter: 'blur(8px)',
              transition: { duration: 0.3, ease: [0.16, 1, 0.3, 1] }
            }}
            exit={{
              opacity: 0,
              backdropFilter: 'blur(0px)',
              transition: { duration: 0.2, ease: [0.16, 1, 0.3, 1] }
            }}
          />

          {/* Modal Container */}
          <div className="fixed inset-0 z-[10001] flex items-center justify-center pointer-events-none p-6">
            <motion.div
              onClick={(e) => e.stopPropagation()}
              className="pointer-events-auto cursor-default"
              initial={{
                opacity: 0,
                scale: 0.92,
                y: 40,
                rotateX: 8
              }}
              animate={{
                opacity: 1,
                scale: 1,
                y: 0,
                rotateX: 0,
                transition: {
                  duration: 0.4,
                  ease: [0.16, 1, 0.3, 1]
                }
              }}
              exit={{
                opacity: 0,
                scale: 0.95,
                y: 20,
                transition: {
                  duration: 0.25,
                  ease: [0.16, 1, 0.3, 1]
                }
              }}
              style={{
                willChange: 'transform, opacity',
                perspective: '1200px',
                transformStyle: 'preserve-3d',
                backgroundColor: 'rgba(15, 15, 20, 0.95)',
                backdropFilter: 'blur(10px)',
                border: '1px solid rgba(115, 115, 115, 0.3)',
                borderRadius: '16px',
                padding: '24px',
                maxWidth: '680px',
                width: '100%',
                maxHeight: '80vh',
                display: 'flex',
                flexDirection: 'column',
                boxShadow: '0 8px 32px rgba(115, 115, 115, 0.2), 0 0 0 1px rgba(115, 115, 115, 0.1) inset'
              }}
            >
              {/* Header */}
              <motion.div
                style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}
                initial={{ opacity: 0, y: -10 }}
                animate={{
                  opacity: 1,
                  y: 0,
                  transition: { delay: 0.1, duration: 0.3, ease: [0.16, 1, 0.3, 1] }
                }}
              >
                <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '1.5rem', color: '#8b5cf6', margin: 0 }}>
                  &#9654; ЗАПУСК БОТА
                </h2>
                <motion.button
                  onClick={onClose}
                  className="cursor-pointer"
                  style={{
                    background: 'rgba(115, 115, 115, 0.2)',
                    border: '1px solid rgba(115, 115, 115, 0.4)',
                    borderRadius: '8px',
                    padding: '8px 16px',
                    color: '#fff',
                    fontFamily: 'var(--font-display)',
                    fontSize: '0.9rem',
                    cursor: 'pointer',
                    transition: 'all 0.2s'
                  }}
                  whileHover={{
                    background: 'rgba(115, 115, 115, 0.3)',
                    scale: 1.05
                  }}
                  whileTap={{ scale: 0.95 }}
                >
                  &#10005; ЗАКРЫТЬ
                </motion.button>
              </motion.div>

              {/* Bot Info */}
              <motion.div
                style={{ marginBottom: '20px', padding: '12px 16px', background: 'rgba(115, 115, 115, 0.1)', border: '1px solid rgba(115, 115, 115, 0.2)', borderRadius: '8px' }}
                initial={{ opacity: 0, y: -5 }}
                animate={{
                  opacity: 1,
                  y: 0,
                  transition: { delay: 0.15, duration: 0.3, ease: [0.16, 1, 0.3, 1] }
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div>
                    <div style={{ fontFamily: 'var(--font-display)', fontSize: '1.1rem', color: 'var(--text-primary)', fontWeight: 700 }}>
                      {bot.name}
                    </div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                      @{bot.twitch_login} &bull; {bot.personality} &bull; {bot.min_delay_sec}-{bot.max_delay_sec}с
                    </div>
                  </div>
                  <div style={{
                    padding: '4px 12px',
                    borderRadius: 'var(--radius-sm)',
                    fontSize: '0.75rem',
                    fontFamily: 'var(--font-display)',
                    textTransform: 'uppercase' as any,
                    letterSpacing: '0.08em',
                    background: 'rgba(115, 115, 115, 0.2)',
                    color: 'var(--text-secondary)'
                  }}>
                    ID: {bot.id}
                  </div>
                </div>
              </motion.div>

              {/* Options Title */}
              <motion.div
                style={{ fontFamily: 'var(--font-display)', fontSize: '0.875rem', color: 'var(--text-secondary)', textTransform: 'uppercase' as any, letterSpacing: '0.08em', marginBottom: '12px' }}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1, transition: { delay: 0.2 } }}
              >
                ВЫБЕРИТЕ РЕЖИМ ЗАПУСКА:
              </motion.div>

              {/* Launch Options */}
              <motion.div
                style={{ display: 'flex', flexDirection: 'column', gap: '12px', marginBottom: '24px' }}
                initial={{ opacity: 0, y: 10 }}
                animate={{
                  opacity: 1,
                  y: 0,
                  transition: { delay: 0.25, duration: 0.3, ease: [0.16, 1, 0.3, 1] }
                }}
              >
                {/* Option 1: Without LLM */}
                <motion.div
                  onClick={() => setSelectedOption('without-llm')}
                  className={`launch-option cursor-target ${selectedOption === 'without-llm' ? 'launch-option--selected' : ''}`}
                  whileHover={{ scale: 1.01, x: 4 }}
                  whileTap={{ scale: 0.99 }}
                  style={{
                    padding: '16px',
                    borderRadius: '12px',
                    border: `1px solid ${selectedOption === 'without-llm' ? 'rgba(139, 92, 246, 0.6)' : 'rgba(115, 115, 115, 0.2)'}`,
                    background: selectedOption === 'without-llm' ? 'rgba(139, 92, 246, 0.1)' : 'rgba(25, 25, 25, 0.5)',
                    cursor: 'pointer',
                    transition: 'all 0.2s ease',
                    display: 'flex',
                    gap: '16px',
                    alignItems: 'flex-start'
                  }}
                >
                  <div style={{
                    width: '48px',
                    height: '48px',
                    borderRadius: '12px',
                    background: selectedOption === 'without-llm' ? 'rgba(139, 92, 246, 0.2)' : 'rgba(115, 115, 115, 0.15)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '1.5rem',
                    flexShrink: 0,
                    transition: 'all 0.2s ease'
                  }}>
                    &#9673;
                  </div>
                  <div style={{ flex: 1 }}>
                    <div style={{
                      fontFamily: 'var(--font-display)',
                      fontSize: '1rem',
                      fontWeight: 700,
                      color: selectedOption === 'without-llm' ? '#8b5cf6' : 'var(--text-primary)',
                      textTransform: 'uppercase' as any,
                      letterSpacing: '0.04em',
                      marginBottom: '4px',
                      transition: 'color 0.2s ease'
                    }}>
                      ЗАПУСТИТЬ БЕЗ LLM
                    </div>
                    <div style={{
                      fontSize: '0.875rem',
                      color: 'var(--text-secondary)',
                      lineHeight: 1.6
                    }}>
                      Только просмотр и мониторинг. Бот подключится к чату, но не будет генерировать сообщения через LLM.
                      STT и генерация ответов отключены.
                    </div>
                    <div style={{
                      marginTop: '8px',
                      fontSize: '0.75rem',
                      color: 'var(--text-muted)',
                      fontFamily: 'var(--font-mono)'
                    }}>
                      &#9888; LLM: ВЫКЛ &#10004; STT: ВЫКЛ &#10004; ТОЛЬКО ЛОГИ
                    </div>
                  </div>
                  {selectedOption === 'without-llm' && (
                    <motion.div
                      initial={{ scale: 0 }}
                      animate={{ scale: 1 }}
                      style={{
                        width: '24px',
                        height: '24px',
                        borderRadius: '50%',
                        background: '#8b5cf6',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        color: '#0a0a0a',
                        fontSize: '0.75rem',
                        flexShrink: 0,
                        alignSelf: 'center'
                      }}
                    >
                      &#10004;
                    </motion.div>
                  )}
                </motion.div>

                {/* Option 2: With LLM */}
                <motion.div
                  onClick={() => setSelectedOption('with-llm')}
                  className={`launch-option cursor-target ${selectedOption === 'with-llm' ? 'launch-option--selected' : ''}`}
                  whileHover={{ scale: 1.01, x: 4 }}
                  whileTap={{ scale: 0.99 }}
                  style={{
                    padding: '16px',
                    borderRadius: '12px',
                    border: `1px solid ${selectedOption === 'with-llm' ? 'rgba(139, 92, 246, 0.6)' : 'rgba(115, 115, 115, 0.2)'}`,
                    background: selectedOption === 'with-llm' ? 'rgba(139, 92, 246, 0.1)' : 'rgba(25, 25, 25, 0.5)',
                    cursor: 'pointer',
                    transition: 'all 0.2s ease',
                    display: 'flex',
                    gap: '16px',
                    alignItems: 'flex-start'
                  }}
                >
                  <div style={{
                    width: '48px',
                    height: '48px',
                    borderRadius: '12px',
                    background: selectedOption === 'with-llm' ? 'rgba(139, 92, 246, 0.2)' : 'rgba(115, 115, 115, 0.15)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '1.5rem',
                    flexShrink: 0,
                    transition: 'all 0.2s ease'
                  }}>
                    &#9889;
                  </div>
                  <div style={{ flex: 1 }}>
                    <div style={{
                      fontFamily: 'var(--font-display)',
                      fontSize: '1rem',
                      fontWeight: 700,
                      color: selectedOption === 'with-llm' ? '#8b5cf6' : 'var(--text-primary)',
                      textTransform: 'uppercase' as any,
                      letterSpacing: '0.04em',
                      marginBottom: '4px',
                      transition: 'color 0.2s ease'
                    }}>
                      ЗАПУСТИТЬ С LLM
                    </div>
                    <div style={{
                      fontSize: '0.875rem',
                      color: 'var(--text-secondary)',
                      lineHeight: 1.6
                    }}>
                      Полный функционал. Бот будет генерировать сообщения через LLM, использовать STT для
                      анализа аудио-стрима и отправлять сообщения в чат.
                    </div>
                    <div style={{
                      marginTop: '8px',
                      fontSize: '0.75rem',
                      color: 'var(--text-muted)',
                      fontFamily: 'var(--font-mono)'
                    }}>
                      &#9889; LLM: ВКЛ &#10004; STT: ВКЛ &#10004; ПОЛНЫЙ ФУНКЦИОНАЛ
                    </div>
                  </div>
                  {selectedOption === 'with-llm' && (
                    <motion.div
                      initial={{ scale: 0 }}
                      animate={{ scale: 1 }}
                      style={{
                        width: '24px',
                        height: '24px',
                        borderRadius: '50%',
                        background: '#8b5cf6',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        color: '#0a0a0a',
                        fontSize: '0.75rem',
                        flexShrink: 0,
                        alignSelf: 'center'
                      }}
                    >
                      &#10004;
                    </motion.div>
                  )}
                </motion.div>
              </motion.div>

              {/* Action Buttons */}
              <motion.div
                style={{ display: 'flex', gap: '12px', justifyContent: 'flex-end' }}
                initial={{ opacity: 0, y: 10 }}
                animate={{
                  opacity: 1,
                  y: 0,
                  transition: { delay: 0.3, duration: 0.3, ease: [0.16, 1, 0.3, 1] }
                }}
              >
                <motion.button
                  onClick={onClose}
                  className="cursor-pointer"
                  style={{
                    background: 'transparent',
                    border: '1px solid var(--border-default)',
                    borderRadius: 'var(--radius-sm)',
                    padding: '12px 24px',
                    color: 'var(--text-primary)',
                    fontFamily: 'var(--font-display)',
                    fontSize: '0.875rem',
                    textTransform: 'uppercase' as any,
                    letterSpacing: '0.08em',
                    fontWeight: 700,
                    cursor: 'pointer',
                    transition: 'all 0.2s ease'
                  }}
                  whileHover={{ background: 'var(--bg-elevated)', scale: 1.02 }}
                  whileTap={{ scale: 0.98 }}
                >
                  ОТМЕНА
                </motion.button>
                <motion.button
                  onClick={handleLaunch}
                  disabled={!selectedOption || isStarting}
                  className="cursor-pointer"
                  style={{
                    background: !selectedOption || isStarting ? 'rgba(115, 115, 115, 0.2)' : 'rgba(139, 92, 246, 0.3)',
                    border: `1px solid ${!selectedOption || isStarting ? 'var(--border-default)' : 'rgba(139, 92, 246, 0.4)'}`,
                    borderRadius: 'var(--radius-sm)',
                    padding: '12px 24px',
                    color: !selectedOption || isStarting ? 'var(--text-muted)' : '#fff',
                    fontFamily: 'var(--font-display)',
                    fontSize: '0.875rem',
                    textTransform: 'uppercase' as any,
                    letterSpacing: '0.08em',
                    fontWeight: 700,
                    cursor: !selectedOption || isStarting ? 'not-allowed' : 'pointer',
                    opacity: !selectedOption || isStarting ? 0.5 : 1,
                    transition: 'all 0.2s ease'
                  }}
                  whileHover={!selectedOption || isStarting ? {} : {
                    background: 'rgba(139, 92, 246, 0.4)',
                    scale: 1.02
                  }}
                  whileTap={!selectedOption || isStarting ? {} : { scale: 0.98 }}
                >
                  {isStarting ? '⟳ ЗАПУСК...' : '▶ ЗАПУСТИТЬ БОТА'}
                </motion.button>
              </motion.div>
            </motion.div>
          </div>
        </>
      )}
    </AnimatePresence>,
    document.body
  )
}
