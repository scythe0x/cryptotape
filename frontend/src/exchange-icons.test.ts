import { expect, it } from 'vitest'
import { exchangeIcons } from './exchange-icons'
import { listingExchanges } from './listings'

it('bundles an icon for every supported listing and opening exchange', () => {
  for (const exchange of ['binance', ...listingExchanges]) {
    expect(exchangeIcons.get(exchange), exchange).toBeTruthy()
  }
})
