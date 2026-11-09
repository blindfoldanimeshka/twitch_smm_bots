import type { ReactNode } from 'react'
import { Alert } from './Alert'
import { Button } from './Button'
import { EmptyState } from './EmptyState'

interface AsyncStateProps {
  /** Request in flight: renders `loadingText` instead of data or emptiness. */
  loading?: boolean
  /** Request failed: renders `errorText` as an error, never as an empty list. */
  error?: boolean
  /** Data loaded successfully and is genuinely empty. */
  isEmpty?: boolean
  loadingText?: ReactNode
  errorText?: ReactNode
  emptyTitle?: string
  emptyDescription?: ReactNode
  /** Shown as «Повторить» below the error alert. */
  onRetry?: () => void
  children?: ReactNode
}

/**
 * Separates the three states pages used to blend together: loading, failure and
 * real emptiness. Order matters — a failed request must not look like an endless
 * load, and an empty result must not look like a failure.
 */
export function AsyncState({
  loading = false,
  error = false,
  isEmpty = false,
  loadingText = 'Загружаем…',
  errorText = 'Не удалось загрузить данные.',
  emptyTitle = 'Пока пусто',
  emptyDescription,
  onRetry,
  children,
}: AsyncStateProps) {
  if (loading) {
    return (
      <p role="status" style={{ margin: 0, fontSize: 13, color: 'var(--muted)' }}>
        {loadingText}
      </p>
    )
  }

  if (error) {
    return (
      <div style={{ display: 'grid', gap: 12, justifyItems: 'start' }}>
        <Alert variant="error">{errorText}</Alert>
        {onRetry && (
          <Button variant="secondary" size="sm" onClick={onRetry}>
            Повторить
          </Button>
        )}
      </div>
    )
  }

  if (isEmpty) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />
  }

  return <>{children}</>
}
