import { useState } from 'react'
import { desktop, defaultTableColumns, type Settings, type TableColumnID } from '../api'
import { useTranslation } from '../i18n'
import { ActiveItems } from './ActiveItems'

export function TableColumns({ settings, onError }: {
  settings: Settings
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
    if (saving) return
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
    <section className="table-columns-panel" data-saving={saving || undefined} aria-label={t('tableColumns')}>
      <ActiveItems
        items={defaultTableColumns.map((id) => ({ id, label: labels[id], content: labels[id] }))}
        value={settings.tableColumns}
        onChange={(value) => void save(value)}
        activeLabel={t('activeColumns')}
        inactiveLabel={t('inactiveColumns')}
      />
    </section>
  )
}
