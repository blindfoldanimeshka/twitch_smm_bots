import { useState } from 'react'
import { ArrowRight, Check } from 'lucide-react'
import { AnimatePresence, motion } from 'framer-motion'
import { cn } from '@/lib/utils'

interface InteractiveHoverButtonProps {
  text?: string
  loadingText?: string
  successText?: string
  classes?: string
  onClick?: () => void | Promise<void>
}

export default function InteractiveHoverButton({
  text = 'Button',
  loadingText = 'Processing...',
  successText = 'Complete!',
  classes,
  onClick,
  ...props
}: InteractiveHoverButtonProps) {
  const [status, setStatus] = useState<'idle' | 'loading' | 'success'>('idle')

  const isIdle = status === 'idle'

  const handleClick = async () => {
    if (status !== 'idle') return

    setStatus('loading')

    try {
      if (onClick) {
        await onClick()
      } else {
        // Simulate async process (for demo purpose only)
        await new Promise(resolve => setTimeout(resolve, 2000))
      }

      setStatus('success')
      setTimeout(() => {
        setStatus('idle')
      }, 3000) // Reset after success
    } catch (error) {
      setStatus('idle')
    }
  }

  return (
    <motion.button
      className={cn(
        'cursor-target group bg-background relative flex min-w-40 items-center justify-center overflow-hidden border p-2 px-6 font-semibold',
        status === 'loading' && 'px-2',
        classes
      )}
      onClick={handleClick}
      layout
      transition={{ type: 'spring', stiffness: 400, damping: 30 }}
      {...props}
    >
      <AnimatePresence mode='popLayout' initial={false}>
        <motion.div
          key='idle'
          className='flex items-center gap-2'
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -10 }}
        >
          <div
            className={cn(
              'bg-primary h-2 w-2 transition-all duration-500 group-hover:scale-[40]',
              !isIdle && 'scale-[40]'
            )}
          />
          <span
            className={cn(
              'inline-block transition-all duration-500 group-hover:translate-x-20 group-hover:opacity-0',
              !isIdle && 'translate-x-20 opacity-0'
            )}
          >
            {text}
          </span>
          <div
            className={cn(
              'text-primary-foreground absolute top-0 left-0 z-10 flex h-full w-full -translate-x-16 items-center justify-center gap-2 opacity-0 transition-all duration-500 group-hover:translate-x-0 group-hover:opacity-100',
              !isIdle && 'translate-x-0 opacity-100'
            )}
          >
            {status === 'idle' ? (
              <>
                <span>{text}</span>
                <ArrowRight className='h-4 w-4' />
              </>
            ) : status === 'loading' ? (
              <>
                <div className='border-primary border-t-background h-4 w-4 animate-spin rounded-full border-2' />
                <span className='text-background'>{loadingText}</span>
              </>
            ) : (
              <>
                <Check className='h-4 w-4' />
                <span>{successText}</span>
              </>
            )}
          </div>
        </motion.div>
      </AnimatePresence>
    </motion.button>
  )
}
