// classified listing sources: https://newlistings.pro/docs/v2/filter-values
export const listingExchanges = [
  'aevo',
  'asterdex',
  'bingx',
  'bitget',
  'bithumb',
  'bitstamp',
  'bybit',
  'coinbase',
  'coinbaseinternational',
  'coinex',
  'coinone',
  'cryptocom',
  'gate',
  'htx',
  'hyperliquid',
  'korbit',
  'kraken',
  'kucoin',
  'lighter',
  'mexc',
  'okx',
  'robinhood',
  'upbit',
] as const

export const listingMarketTypes = [
  'spot',
  'futures',
  'pre-market',
  'roadmap',
  'soon-spot',
  'assets',
] as const

const listingEventLabels = {
  spot: 'listingEventSpot',
  futures: 'listingEventFutures',
  'pre-market': 'listingEventPreMarket',
  roadmap: 'listingEventRoadmap',
  'soon-spot': 'listingEventSoonSpot',
  assets: 'listingEventAssets',
} as const

export function listingEventLabelKey(kind: string) {
  const known = listingMarketTypes.find((value) => value === kind)
  return known ? listingEventLabels[known] : 'listing'
}

export function exchangeLabel(exchange: string): string {
  const labels: Record<string, string> = {
    binance: 'Binance',
    asterdex: 'Aster',
    bingx: 'BingX',
    coinex: 'CoinEx',
    kucoin: 'KuCoin',
    upbit: 'Upbit',
    bithumb: 'Bithumb',
    coinbase: 'Coinbase',
    coinbaseinternational: 'Coinbase Intl.',
    cryptocom: 'Crypto.com',
    htx: 'HTX',
    okx: 'OKX',
    mexc: 'MEXC',
    gate: 'Gate',
  }
  return labels[exchange] ?? exchange.charAt(0).toUpperCase() + exchange.slice(1)
}
