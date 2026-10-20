export function latencyState(pingMs: number | null): 'unknown' | 'good' | 'warning' | 'poor' {
  if (pingMs === null) return 'unknown'
  if (pingMs < 200) return 'good'
  return pingMs < 500 ? 'warning' : 'poor'
}
