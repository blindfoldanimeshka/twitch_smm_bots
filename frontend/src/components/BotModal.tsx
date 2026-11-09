import type { FormEvent } from 'react'
import type { Bot } from '../api/client'
import { useBotForm } from '../hooks/useBotForm'
import type { BotFormData } from '../hooks/useBotForm'
import { Alert } from './ui/Alert'
import { Button } from './ui/Button'
import { Modal } from './ui/Modal'
import { BotForm } from './BotForm'

interface BotModalProps {
  open: boolean
  onClose: () => void
  /** Bot being edited, or null to create a new one. */
  editingBot: Bot | null
  isSubmitting: boolean
  /** Mutation error shown inline at the top of the form. */
  error: string | null
  onSubmit: (data: BotFormData) => void
}

/** Add/edit bot dialog. Owns the form state via `useBotForm`. */
export function BotModal({
  open,
  onClose,
  editingBot,
  isSubmitting,
  error,
  onSubmit,
}: BotModalProps) {
  const form = useBotForm(editingBot ?? undefined)

  const handleClose = () => {
    form.reset()
    onClose()
  }

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    if (!form.isValid) {
      return
    }
    onSubmit(form.formData)
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={editingBot ? 'Редактировать бота' : 'Новый бот'}
      width={640}
      footer={
        <>
          <Button variant="secondary" onClick={handleClose} disabled={isSubmitting}>
            Отмена
          </Button>
          <Button
            type="submit"
            form="bot-form"
            loading={isSubmitting}
            disabled={!form.isValid}
          >
            {editingBot ? 'Сохранить' : 'Создать бота'}
          </Button>
        </>
      }
    >
      <div style={{ display: 'grid', gap: 12 }}>
        {error && <Alert variant="error">{error}</Alert>}
        <BotForm
          formData={form.formData}
          errors={form.errors}
          touchedFields={form.touchedFields}
          isEditing={Boolean(editingBot)}
          onChange={form.handleChange}
          onBlur={form.handleBlur}
          onSubmit={handleSubmit}
        />
      </div>
    </Modal>
  )
}
