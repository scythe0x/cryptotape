import type { ProxyProfile } from './api'

export function nextProxyName(proxies: ProxyProfile[]): string {
  const names = new Set(proxies.map((proxy) => proxy.name))
  let number = 1
  while (names.has(`proxy-${number}`)) number++
  return `proxy-${number}`
}

// parse a pasted address and optional labeled credentials without sending them anywhere.
export function parseProxy(text: string): Partial<ProxyProfile> | null {
  const lines = text
    .trim()
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
  const first = lines[0]
  if (!first) return null
  const explicit = /^(https?|socks5h?):\/\//i.test(first)
  try {
    const url = new URL(explicit ? first : `http://${first.replace(/\s*:\s*/g, ':')}`)
    if (
      !['http:', 'https:', 'socks5:', 'socks5h:'].includes(url.protocol) ||
      (url.pathname !== '/' && url.pathname !== '') ||
      url.search ||
      url.hash
    )
      return null
    const port = Number(
      url.port ||
        (explicit ? (url.protocol === 'https:' ? 443 : url.protocol.startsWith('socks5') ? 1080 : 80) : 0),
    )
    if (!port || port > 65535) return null
    const host = url.hostname.replace(/^\[|\]$/g, '')
    const result: Partial<ProxyProfile> = { host, port }
    if (url.username || url.password) {
      result.login = decodeURIComponent(url.username)
      result.password = decodeURIComponent(url.password)
    }
    for (const line of lines.slice(1)) {
      const match = line.match(/^(логин|login|username|пароль|password)\s*:\s*(.*)$/i)
      if (match) result[/^(пароль|password)$/i.test(match[1]) ? 'password' : 'login'] = match[2]
    }
    return result
  } catch {
    return null
  }
}
