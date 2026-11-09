import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { UserPlus } from 'lucide-react'
import { api, getErrorMessage } from '../api/client'
import type { Bot } from '../api/client'
import { PageHeader } from '../components/layout/PageHeader'
import { BotsTable, type BotTableAction } from '../components/BotsTable'
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
  const [pending, setPending] = useState<{ id: string; action: BotTableAction } | null>(null)

  const { data: bots = [] } = useQuery({
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

  const startBot = useMutation({
    mutationFn: (bot: Bot) => api.startBot(bot.id),
    onMutate: (bot) => setPending({ id: bot.id, action: 'start' }),
    onSettled: () => setPending(null),
    onSuccess: invalidateAfterAction,
  })

  const stopBot = useMutation({
    mutationFn: (bot: Bot) => api.stopBot(bot.id),
    onMutate: (bot) => setPending({ id: bot.id, action: 'stop' }),
    onSettled: () => setPending(null),
    onSuccess: invalidateAfterAction,
  })

  const actionError = deleteBot.error ?? startBot.error ?? stopBot.error
  const modalError = createBot.error ?? updateBot.error

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
        {actionError && <Alert variant="error">{getErrorMessage(actionError)}</Alert>}

        <Card title={`Список ботов (${bots.length})`}>
          <BotsTable
            bots={bots}
            pending={pending}
            onStart={(bot) => startBot.mutate(bot)}
            onStop={(bot) => stopBot.mutate(bot)}
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
        onClose={() => setDeletingBot(null)}
        title="Удалить бота?"
        width={440}
        footer={
          <>
            <Button variant="secondary" onClick={() => setDeletingBot(null)} disabled={deleteBot.isPending}>
              Отмена
            </Button>
            <Button
              variant="danger"
              loading={deleteBot.isPending}
              onClick={() => deletingBot && deleteBot.mutate(deletingBot.id)}
            >
              Удалить
            </Button>
          </>
        }
      >
        <p style={{ fontSize: 14 }}>
          Бот <strong>{deletingBot?.name}</strong> (@{deletingBot?.twitch_login}) будет удалён.
          Действие необратимо.
        </p>
      </Modal>

      <BotLogsDrawer bot={logsBot} onClose={() => setLogsBot(null)} />
    </>
  )
}
