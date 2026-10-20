export function Ticker({ symbol }: { symbol: string }) {
  return symbol.endsWith('USDT')
    ? <>{symbol.slice(0, -4)}<span className="ticker-quote">USDT</span></>
    : <>{symbol}</>
}
