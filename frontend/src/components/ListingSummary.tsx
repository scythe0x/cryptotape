import type { Alert } from '../api'
import { useTranslation } from '../i18n'
import { exchangeLabel, listingEventLabelKey } from '../listings'

export function ListingSummary({ alert }: { alert: Alert }) {
  const { t } = useTranslation()
  if (alert.kind !== 'listing' || !alert.listing) return null
  return (
    <span
      className="listing-summary"
      title={`${t(listingEventLabelKey(alert.listing.marketType))} ${alert.listing.markets.join(', ')}`}
    >
      <strong>{exchangeLabel(alert.listing.exchange)}</strong>
      <small>{t(listingEventLabelKey(alert.listing.marketType))}</small>
      <span className="sr-only">{t('listing')}</span>
    </span>
  )
}
