import { defaultOpenExchanges, type OpenExchange } from '../api'
import { useTranslation } from '../i18n'
import { exchangeLabel } from '../listings'
import { ExchangeIcon } from './ExchangeIcon'
import { ActiveItems } from './ActiveItems'

type Props = { value: OpenExchange[]; onChange: (value: OpenExchange[]) => void }

export function ExchangePriority({ value, onChange }: Props) {
  const { t } = useTranslation()
  return (
    <fieldset className="exchange-priority">
      <legend>{t('openOnExchange')}</legend>
      <ActiveItems
        items={defaultOpenExchanges.map((exchange) => ({
          id: exchange,
          label: exchangeLabel(exchange),
          content: <ExchangeIcon exchange={exchange} />,
        }))}
        value={value}
        onChange={onChange}
        activeLabel={t('activeExchanges')}
        inactiveLabel={t('inactiveExchanges')}
      />
    </fieldset>
  )
}
