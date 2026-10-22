export const tableColumnIDs = ['time', 'ticker', 'event', 'terminals'] as const
export type TableColumnID = typeof tableColumnIDs[number]

export const minimumTableRowHeight = 44

export const tableColumnDefinitions = {
  time: { minimum: 66, preferred: 66 },
  ticker: { minimum: 90, preferred: 120 },
  event: { minimum: 84, preferred: null },
  terminals: { minimum: 72, preferred: 72 },
} satisfies Record<TableColumnID, { minimum: number; preferred: number | null }>

export function tableMinimumWidth(columns: readonly TableColumnID[]): number {
  return columns.reduce((width, column) => width + tableColumnDefinitions[column].minimum, 0)
}

export function tableGridTemplate(columns: readonly TableColumnID[]): string {
  return columns.map((column) => {
    const { minimum, preferred } = tableColumnDefinitions[column]
    if (preferred === null) return `minmax(${minimum}px, 1fr)`
    if (minimum === preferred) return `${minimum}px`
    return `minmax(${minimum}px, ${preferred}px)`
  }).join(' ')
}
