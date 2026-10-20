export function latencyState(pingMs: number | null): 'unknown' | 'good' | 'warning' | 'poor' {
  if (pingMs === null) return 'unknown'
  if (pingMs < 200) return 'good'
  return pingMs < 500 ? 'warning' : 'poor'
}

export function quoteAgeState(ageMs: number | null): 'unknown' | 'good' | 'warning' | 'poor' {
  if (ageMs === null) return 'unknown'
  if (ageMs > 10_000) return 'poor'
  return ageMs > 2_000 ? 'warning' : 'good'
}
