import { useState } from 'react'
import * as Popover from '@radix-ui/react-popover'
import { Columns3 } from 'lucide-react'
import { desktop, defaultTableColumns, type Settings, type TableColumnID } from '../api'
import { useTranslation } from '../i18n'
import { ActiveItems } from './ActiveItems'
import { Button } from './ui'
import './alerts-table.css'

export function TableColumns({ settings, onError }: {
  settings: Settings | null
  onError: (message: string) => void
}) {
  const { t } = useTranslation()
  const [saving, setSaving] = useState(false)
  const labels: Record<TableColumnID, string> = {
    time: t('time'),
    ticker: t('ticker'),
    event: t('alertEvent'),
    terminals: t('terminal'),
  }

  async function save(value: TableColumnID[]) {
    if (saving || !settings) return
    setSaving(true)
    try {
      await desktop.setTableColumns(value)
    } catch (error) {
      onError(String(error))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <Button
          size="icon"
          variant="ghost"
          aria-label={t('tableColumns')}
          title={t('tableColumns')}
          disabled={!settings}
        >
          <Columns3 size={15} />
        </Button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          className="table-columns-popover"
          align="end"
          sideOffset={8}
          data-saving={saving || undefined}
          aria-label={t('tableColumns')}
        >
          <h2 className="table-columns-title">{t('tableColumns')}</h2>
          <ActiveItems
            items={defaultTableColumns.map((id) => ({ id, label: labels[id], content: labels[id] }))}
            value={settings?.tableColumns ?? defaultTableColumns}
            onChange={(value) => void save(value)}
            activeLabel={t('activeColumns')}
            inactiveLabel={t('inactiveColumns')}
          />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
