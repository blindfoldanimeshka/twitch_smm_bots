import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { UserPlus, X } from 'lucide-react'
import { api, getErrorMessage } from '../api/client'
import type { Bot } from '../api/client'
import { PageHeader } from '../components/layout/PageHeader'
import { BotsTable } from '../components/BotsTable'
import { BotModal } from '../components/BotModal'
import { BotLogsDrawer } from '../components/BotLogsDrawer'
import { Alert } from '../components/ui/Alert'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Modal } from '../components/ui/Modal'
import type { BotFormData } from '../hooks/useBotForm'

export default function Bots() {
  const queryClient = useQueryClient()

  const [modalOpen, setModalOpen] = useState(false)
  const [editingBot, setEditingBot] = useState<Bot | null>(null)
  const [deletingBot, setDeletingBot] = useState<Bot | null>(null)
  const [logsBot, setLogsBot] = useState<Bot | null>(null)
  const [pendingIds, setPendingIds] = useState<ReadonlySet<string>>(() => new Set())

  const {
    data: bots = [],
    isError: botsLoadFailed,
    refetch: refetchBots,
  } = useQuery({
    queryKey: ['bots'],
    queryFn: async () => (await api.getBots()).data,
  })

  const invalidateAfterAction = () => {
    queryClient.invalidateQueries({ queryKey: ['bots'] })
    queryClient.invalidateQueries({ queryKey: ['stats'] })
  }

  const closeModal = () => {
    setModalOpen(false)
    setEditingBot(null)
    createBot.reset()
    updateBot.reset()
  }

  const createBot = useMutation({
    mutationFn: (data: BotFormData) => api.createBot(data),
    onSuccess: () => {
      invalidateAfterAction()
      closeModal()
    },
  })

  const updateBot = useMutation({
    mutationFn: ({ id, data }: { id: string; data: BotFormData }) => {
      const { password, ...fields } = data
      // An empty OAuth token on edit means "keep the current one": omit it.
      const payload = (password ? { ...fields, password } : fields) as Partial<Bot>
      return api.updateBot(id, payload)
    },
    onSuccess: () => {
      invalidateAfterAction()
      closeModal()
    },
  })

  const deleteBot = useMutation({
    mutationFn: (id: string) => api.deleteBot(id),
    onSuccess: () => {
      invalidateAfterAction()
      setDeletingBot(null)
    },
  })

  // A Set keeps each row's spinner independent when several actions are in flight.
  const markPending = (bot: Bot) =>
    setPendingIds((prev) => new Set(prev).add(bot.id))

  const unmarkPending = (bot: Bot) =>
    setPendingIds((prev) => {
      if (!prev.has(bot.id)) {
        return prev
      }
      const next = new Set(prev)
      next.delete(bot.id)
      return next
    })

  const startBot = useMutation({
    mutationFn: (bot: Bot) => api.startBot(bot.id),
    onMutate: markPending,
    onSettled: (_data, _error, bot) => unmarkPending(bot),
    onSuccess: invalidateAfterAction,
  })

  const stopBot = useMutation({
    mutationFn: (bot: Bot) => api.stopBot(bot.id),
    onMutate: markPending,
    onSettled: (_data, _error, bot) => unmarkPending(bot),
    onSuccess: invalidateAfterAction,
  })

  // Delete errors render inside the delete modal; start/stop errors as a
  // dismissible page-level alert.
  const actionError = startBot.error ?? stopBot.error
  const modalError = createBot.error ?? updateBot.error

  const clearActionErrors = () => {
    startBot.reset()
    stopBot.reset()
  }

  const handleStart = (bot: Bot) => {
    clearActionErrors()
    startBot.mutate(bot)
  }

  const handleStop = (bot: Bot) => {
    clearActionErrors()
    stopBot.mutate(bot)
  }

  const handleDeleteClose = () => {
    deleteBot.reset()
    setDeletingBot(null)
  }

  const handleDeleteConfirm = () => {
    if (deletingBot) {
      deleteBot.reset()
      deleteBot.mutate(deletingBot.id)
    }
  }

  const handleSubmit = (data: BotFormData) => {
    if (editingBot) {
      updateBot.mutate({ id: editingBot.id, data })
    } else {
      createBot.mutate(data)
    }
  }

  const openCreate = () => {
    setEditingBot(null)
    setModalOpen(true)
  }

  const openEdit = (bot: Bot) => {
    setEditingBot(bot)
    setModalOpen(true)
  }

  const submitPending = createBot.isPending || updateBot.isPending

  return (
    <>
      <PageHeader
        title="Боты"
        description="Аккаунты ботов: запуск, настройки поведения и логи работы"
        actions={
          <Button variant="accent" onClick={openCreate}>
            <UserPlus size={16} aria-hidden="true" />
            Добавить бота
          </Button>
        }
      />

      <div style={{ display: 'grid', gap: 16 }}>
        {actionError && (
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
            <div style={{ flex: 1 }}>
              <Alert variant="error">{getErrorMessage(actionError)}</Alert>
            </div>
            <button
              type="button"
              className="modal__close"
              aria-label="Закрыть"
              onClick={clearActionErrors}
            >
              <X size={18} aria-hidden="true" />
            </button>
          </div>
        )}

        {/* A failed list load must not look like "no bots yet": show the
            error with a retry instead of the empty state. */}
        {botsLoadFailed ? (
          <div style={{ display: 'grid', gap: 12 }}>
            <Alert variant="error">Не удалось загрузить список ботов.</Alert>
            <div>
              <Button variant="secondary" size="sm" onClick={() => refetchBots()}>
                Повторить
              </Button>
            </div>
          </div>
        ) : (
          <Card title={`Список ботов (${bots.length})`}>
            <BotsTable
              bots={bots}
              pendingIds={pendingIds}
              onStart={handleStart}
              onStop={handleStop}
              onEdit={openEdit}
              onDelete={setDeletingBot}
              onShowLogs={setLogsBot}
              emptyAction={
                <Button variant="accent" onClick={openCreate}>
                  Добавить бота
                </Button>
              }
            />
          </Card>
        )}
      </div>

      <BotModal
        open={modalOpen}
        onClose={closeModal}
        editingBot={editingBot}
        isSubmitting={submitPending}
        error={modalError ? getErrorMessage(modalError) : null}
        onSubmit={handleSubmit}
      />

      <Modal
        open={deletingBot !== null}
        onClose={handleDeleteClose}
        title="Удалить бота?"
        width={440}
        footer={
          <>
            <Button variant="secondary" onClick={handleDeleteClose} disabled={deleteBot.isPending}>
              Отмена
            </Button>
            <Button variant="danger" loading={deleteBot.isPending} onClick={handleDeleteConfirm}>
              Удалить
            </Button>
          </>
        }
      >
        <div style={{ display: 'grid', gap: 12 }}>
          {deleteBot.error && (
            <Alert variant="error">{getErrorMessage(deleteBot.error)}</Alert>
          )}
          <p style={{ fontSize: 14 }}>
            Бот <strong>{deletingBot?.name}</strong> (@{deletingBot?.twitch_login}) будет удалён.
            Действие необратимо.
          </p>
        </div>
      </Modal>

      <BotLogsDrawer bot={logsBot} onClose={() => setLogsBot(null)} />
    </>
  )
}
