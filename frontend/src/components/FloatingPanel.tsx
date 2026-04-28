import React, { createContext, useContext, useState, useRef, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import './FloatingPanel.css'

interface FloatingPanelContextType {
  isOpen: boolean
  setIsOpen: (open: boolean) => void
}

const FloatingPanelContext = createContext<FloatingPanelContextType | undefined>(undefined)

const useFloatingPanel = () => {
  const context = useContext(FloatingPanelContext)
  if (!context) {
    throw new Error('FloatingPanel components must be used within FloatingPanelRoot')
  }
  return context
}

export function FloatingPanelRoot({ children }: { children: React.ReactNode }) {
  const [isOpen, setIsOpen] = useState(false)

  return (
    <FloatingPanelContext.Provider value={{ isOpen, setIsOpen }}>
      {children}
    </FloatingPanelContext.Provider>
  )
}

export function FloatingPanelTrigger({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  const { setIsOpen } = useFloatingPanel()

  return (
    <button
      className={`floating-panel-trigger ${className}`}
      onClick={() => setIsOpen(true)}
      type="button"
    >
      {children}
    </button>
  )
}

export function FloatingPanelContent({ children }: { children: React.ReactNode }) {
  const { isOpen, setIsOpen } = useFloatingPanel()
  const contentRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        setIsOpen(false)
      }
    }
    window.addEventListener('keydown', handleEscape)
    return () => window.removeEventListener('keydown', handleEscape)
  }, [isOpen, setIsOpen])

  useEffect(() => {
    if (isOpen && contentRef.current) {
      const firstInput = contentRef.current.querySelector('input, textarea') as HTMLElement
      firstInput?.focus()
    }
  }, [isOpen])

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div
            className="floating-panel-backdrop"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={() => setIsOpen(false)}
          />
          <motion.div
            ref={contentRef}
            className="floating-panel-content"
            initial={{ opacity: 0, scale: 0.95, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 20 }}
            transition={{ duration: 0.2, ease: [0.2, 0.8, 0.2, 1] }}
          >
            {children}
          </motion.div>
        </>
      )}
    </AnimatePresence>
  )
}

export function FloatingPanelForm({
  children,
  onSubmit,
}: {
  children: React.ReactNode
  onSubmit: (e: React.FormEvent) => void
}) {
  const { setIsOpen } = useFloatingPanel()

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit(e)
    setIsOpen(false)
  }

  return (
    <form onSubmit={handleSubmit} className="floating-panel-form">
      {children}
    </form>
  )
}

export function FloatingPanelLabel({
  children,
  htmlFor,
}: {
  children: React.ReactNode
  htmlFor?: string
}) {
  return (
    <motion.label
      htmlFor={htmlFor}
      className="floating-panel-label"
      initial={{ opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.1 }}
    >
      {children}
    </motion.label>
  )
}

export function FloatingPanelTextarea({
  id,
  value,
  onChange,
  placeholder = 'Enter text...',
}: {
  id?: string
  value?: string
  onChange?: (e: React.ChangeEvent<HTMLTextAreaElement>) => void
  placeholder?: string
}) {
  return (
    <textarea
      id={id}
      className="floating-panel-textarea"
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      rows={4}
    />
  )
}

export function FloatingPanelInput({
  id,
  type = 'text',
  value,
  onChange,
  placeholder = '',
}: {
  id?: string
  type?: string
  value?: string
  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void
  placeholder?: string
}) {
  return (
    <input
      id={id}
      type={type}
      className="floating-panel-input"
      value={value}
      onChange={onChange}
      placeholder={placeholder}
    />
  )
}

export function FloatingPanelFooter({ children }: { children: React.ReactNode }) {
  return <div className="floating-panel-footer">{children}</div>
}

export function FloatingPanelCloseButton({ children = 'Cancel' }: { children?: React.ReactNode }) {
  const { setIsOpen } = useFloatingPanel()

  return (
    <button
      type="button"
      className="floating-panel-button floating-panel-button--secondary"
      onClick={() => setIsOpen(false)}
    >
      {children}
    </button>
  )
}

export function FloatingPanelSubmitButton({ children = 'Submit' }: { children?: React.ReactNode }) {
  return (
    <button type="submit" className="floating-panel-button floating-panel-button--primary">
      {children}
    </button>
  )
}
