import type { Alert } from '../api'
import { useTranslation } from '../i18n'
import { exchangeLabel, listingEventLabelKey } from '../listings'

export function ListingSummary({ alert }: { alert: Alert }) {
  const { t } = useTranslation()
  if (alert.kind !== 'listing' || !alert.listing) return null
  return <span className="listing-event">{t(listingEventLabelKey(alert.listing.marketType))} {exchangeLabel(alert.listing.exchange)}</span>
}
