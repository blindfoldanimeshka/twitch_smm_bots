import React from 'react'
import './BotFormModal.css'

interface BotFormModalProps {
  isOpen: boolean
  onClose: () => void
  children: React.ReactNode
  title: string
}

export function BotFormModal({ isOpen, onClose, children, title }: BotFormModalProps) {
  React.useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden'
      // Focus trap
      const modal = document.querySelector('.bot-modal')
      if (modal) {
        const focusableElements = modal.querySelectorAll(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        )
        const firstElement = focusableElements[0] as HTMLElement
        const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement

        const handleTab = (e: KeyboardEvent) => {
          if (e.key === 'Tab') {
            if (e.shiftKey) {
              if (document.activeElement === firstElement) {
                lastElement?.focus()
                e.preventDefault()
              }
            } else {
              if (document.activeElement === lastElement) {
                firstElement?.focus()
                e.preventDefault()
              }
            }
          }
        }

        modal.addEventListener('keydown', handleTab as any)
        firstElement?.focus()

        return () => {
          modal.removeEventListener('keydown', handleTab as any)
        }
      }
    } else {
      document.body.style.overflow = ''
    }
  }, [isOpen])

  React.useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose()
      }
    }
    window.addEventListener('keydown', handleEscape)
    return () => window.removeEventListener('keydown', handleEscape)
  }, [isOpen, onClose])

  if (!isOpen) return null

  return (
    <div className="bot-modal-overlay" onClick={onClose}>
      <div
        className="bot-modal"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
      >
        <div className="bot-modal__header">
          <h2 id="modal-title" className="bot-modal__title">
            {title}
          </h2>
          <button
            className="bot-modal__close"
            onClick={onClose}
            aria-label="Закрыть модальное окно"
            type="button"
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
              <path
                d="M18 6L6 18M6 6L18 18"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
              />
            </svg>
          </button>
        </div>
        <div className="bot-modal__content">{children}</div>
      </div>
    </div>
  )
}
