import { Reorder, useDragControls } from 'motion/react'
import type { OpenExchange } from '../api'
import { useTranslation } from '../i18n'
import { exchangeLabel } from '../listings'
import { ExchangeIcon } from './ExchangeIcon'

type Props = { value: OpenExchange[]; onChange: (value: OpenExchange[]) => void }

export function ExchangePriority({ value, onChange }: Props) {
  const { t } = useTranslation()
  function move(index: number, direction: number) {
    const next = [...value]
    const target = index + direction
    const current = next[index]
    const neighbor = next[target]
    if (!current || !neighbor) return
    next[index] = neighbor
    next[target] = current
    onChange(next)
  }
  return (
    <fieldset className="exchange-priority">
      <legend>{t('openOnExchange')}</legend>
      <p className="exchange-priority-help">{t('exchangePriorityHelp')}</p>
      <Reorder.Group axis="x" values={value} onReorder={onChange} className="exchange-priority-list">
        {value.map((exchange, index) => (
          <PriorityItem key={exchange} exchange={exchange} position={index + 1} onMove={(direction) => move(index, direction)} />
        ))}
      </Reorder.Group>
    </fieldset>
  )
}

function PriorityItem({ exchange, position, onMove }: {
  exchange: OpenExchange
  position: number
  onMove: (direction: number) => void
}) {
  const { t } = useTranslation()
  const controls = useDragControls()
  return (
    <Reorder.Item value={exchange} dragControls={controls} dragListener={false} className="exchange-priority-item">
      <button
        type="button"
        className="exchange-priority-handle"
        aria-label={t('exchangePriorityItem', { exchange: exchangeLabel(exchange), position })}
        title={exchangeLabel(exchange)}
        onPointerDown={(event) => controls.start(event)}
        onKeyDown={(event) => {
          if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
          event.preventDefault()
          onMove(event.key === 'ArrowLeft' ? -1 : 1)
        }}
      >
        <ExchangeIcon exchange={exchange} />
      </button>
    </Reorder.Item>
  )
}
