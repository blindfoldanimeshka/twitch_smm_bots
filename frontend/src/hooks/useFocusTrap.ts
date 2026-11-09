import { useEffect, useRef } from 'react'
import type { RefObject } from 'react'

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), input, select, textarea, [tabindex]:not([tabindex="-1"])'

/**
 * Module-level stack of ids of currently active traps. Dialogs can stack
 * (e.g. a modal opened while a drawer is open); only the topmost trap is
 * allowed to handle Tab or Escape, so the dialogs do not fight over focus
 * or close together.
 */
const activeTraps: number[] = []
let nextTrapId = 1

/**
 * Keeps keyboard focus inside `ref` while `active` is true: cycles Tab and
 * Shift+Tab between the focusable elements, focuses the first focusable
 * element when the trap opens, and restores focus to the element that was
 * active before opening when the trap is deactivated.
 *
 * `onEscape` is invoked on Escape only while this trap is the topmost one
 * in the stack, so stacked dialogs close one at a time, innermost first.
 */
export function useFocusTrap(
  ref: RefObject<HTMLElement | null>,
  active: boolean,
  onEscape?: () => void,
) {
  const onEscapeRef = useRef<(() => void) | undefined>(onEscape)
  useEffect(() => {
    onEscapeRef.current = onEscape
  }, [onEscape])

  useEffect(() => {
    const container = ref.current
    if (!active || !container) {
      return
    }

    const trapId = nextTrapId++
    activeTraps.push(trapId)
    const isTopmost = () => activeTraps[activeTraps.length - 1] === trapId

    const previouslyFocused = document.activeElement as HTMLElement | null

    const getFocusable = (): HTMLElement[] =>
      Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))

    const focusables = getFocusable()
    if (focusables.length > 0) {
      focusables[0].focus()
    } else {
      container.focus()
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (!isTopmost()) {
        return
      }
      if (event.key === 'Escape') {
        onEscapeRef.current?.()
        return
      }
      if (event.key !== 'Tab') {
        return
      }
      const items = getFocusable()
      if (items.length === 0) {
        event.preventDefault()
        container.focus()
        return
      }
      const first = items[0]
      const last = items[items.length - 1]
      const current = document.activeElement
      const inside = current instanceof HTMLElement && container.contains(current)
      if (event.shiftKey) {
        if (!inside || current === first) {
          event.preventDefault()
          last.focus()
        }
      } else if (!inside || current === last) {
        event.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      const index = activeTraps.indexOf(trapId)
      if (index !== -1) {
        activeTraps.splice(index, 1)
      }
      if (previouslyFocused && document.contains(previouslyFocused)) {
        previouslyFocused.focus()
      }
    }
  }, [ref, active])
}
