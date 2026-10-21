import type { Rule } from '../api'
import { useTranslation } from '../i18n'
import { exchangeLabel, listingExchanges, listingMarketTypes, listingEventLabelKey } from '../listings'
import { ExchangeIcon } from './ExchangeIcon'
import { Checkbox } from './ui'

type Props = { draft: Rule; onChange: (rule: Rule) => void }

export function ListingFields({ draft, onChange }: Props) {
  const { t } = useTranslation()
  const exchanges = draft.listingExchanges ?? [...listingExchanges]
  const types = draft.listingTypes ?? ['spot']

  function toggleExchange(exchange: string) {
    const next = exchanges.includes(exchange)
      ? exchanges.filter((value) => value !== exchange)
      : [...exchanges, exchange]
    onChange({ ...draft, listingExchanges: next })
  }

  return (
    <>
      <fieldset className="listing-filter">
        <legend>{t('listingExchanges')}</legend>
        <div className="listing-exchange-grid">
          {listingExchanges.map((exchange) => (
            <button
              type="button"
              role="checkbox"
              aria-label={exchangeLabel(exchange)}
              aria-checked={exchanges.includes(exchange)}
              className={`exchange-card ${exchanges.includes(exchange) ? 'is-active' : ''}`}
              key={exchange}
              onClick={() => toggleExchange(exchange)}
            >
              <ExchangeIcon exchange={exchange} />
              <span>{exchangeLabel(exchange)}</span>
            </button>
          ))}
        </div>
      </fieldset>
      <fieldset className="listing-filter">
        <legend>{t('listingEventTypes')}</legend>
        <div className="listing-market-grid">
          {listingMarketTypes.map((kind) => (
            <label className="check-label" key={kind}>
              <Checkbox
                checked={types.includes(kind)}
                disabled={types.length === 1 && types.includes(kind)}
                onChange={() =>
                  onChange({
                    ...draft,
                    listingTypes: types.includes(kind)
                      ? types.filter((value) => value !== kind)
                      : [...types, kind],
                  })
                }
              />
              {t(listingEventLabelKey(kind))}
            </label>
          ))}
        </div>
      </fieldset>
    </>
  )
}
