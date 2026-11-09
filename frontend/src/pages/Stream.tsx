import { PageHeader } from '../components/layout/PageHeader'
import { Card } from '../components/ui/Card'
import { EmptyState } from '../components/ui/EmptyState'
import { MonitorPlay } from 'lucide-react'

// Minimal placeholder: the full Stream page is implemented in Task 9.
export default function Stream() {
  return (
    <>
      <PageHeader title="Стрим" description="Статус трансляции, зрители и чат" />
      <Card title="Трансляция">
        <EmptyState
          icon={MonitorPlay}
          title="Статус стрима появится здесь"
          description="Этот раздел сейчас в разработке."
        />
      </Card>
    </>
  )
}
