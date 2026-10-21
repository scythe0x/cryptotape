import i18n, { type TranslationKey } from './index'

const messages: Record<string, TranslationKey> = {
  'invalid proxy profile': 'invalidProxyProfile',
  'unknown proxy profile': 'unknownProxyProfile',
  'proxy connection failed': 'proxyConnectionFailed',
  'binance connection failed': 'binanceDisconnected',
  initializing: 'initializing',
  'loading symbols and turnover': 'loadingMarket',
  'opening market streams': 'openingStreams',
  'waiting for rule windows': 'waitingWindows',
  'signals live': 'signalsLive',
  'invalid alert rule': 'invalidRule',
  'unsupported interface language': 'unsupportedLanguage',
  'invalid settings': 'invalidSettings',
  'no enabled exchanges': 'noEnabledExchanges',
  'alert not found': 'alertNotFound',
  'invalid table columns': 'invalidTableColumns',
  'invalid Tiger port or link group': 'invalidTigerSettings',
  'proxy must be ip:port': 'invalidProxy',
  'invalid Binance symbol': 'invalidSymbol',
  'sound must be mp3 or wav': 'soundFormats',
  'supported formats: mp3, wav': 'soundFormats',
  'sound must be smaller than 2 MB': 'soundSize',
  'invalid sound file': 'invalidSound',
  'sound name is empty': 'soundNameEmpty',
  'invalid volume': 'invalidVolume',
  'unknown sound': 'unknownSound',
  'invalid wav file': 'invalidWav',
  'unsupported wav format': 'unsupportedWav',
  'sound is too long': 'soundTooLong',
  'cannot read sound': 'cannotReadSound',
  'alert is older than 10 seconds': 'staleAlert',
  'unknown terminal': 'unknownTerminal',
  'choose a terminal': 'chooseTerminal',
  'invalid terminal selection': 'invalidTerminals',
  'invalid Binance clock': 'invalidClock',
  'binance returned no server time': 'noServerTime',
  'no trading USDT futures returned by Binance': 'noMarkets',
  'Tiger endpoint changed': 'tigerChanged',
  'Tiger client closed': 'tigerClosed',
  'testing Binance route…': 'testingRoute',
  'Binance connection is working': 'routeWorking',
  'use of closed network connection': 'socketClosed',
  'connection refused': 'connectionRefused',
  'context deadline exceeded': 'deadlineExceeded',
  'context canceled': 'contextCanceled',
}
const prefixes: [string, TranslationKey][] = [
  ['book stream', 'bookStream'],
  ['volume stream', 'volumeStream'],
  ['market refresh', 'marketRefresh'],
  ['clock sync', 'clockSync'],
  ['symbol refresh', 'symbolRefresh'],
  ['alert storage', 'alertStorage'],
  ['stale book stream', 'staleBook'],
  ['stale volume stream', 'staleVolume'],
  ['invalid Tiger response', 'invalidTigerResponse'],
  ['alert sync', 'alertSync'],
  ['terminal', 'terminal'],
  ['connection failed', 'connectionFailed'],
  ['failed to read', 'failedRead'],
  ['failed to write', 'failedWrite'],
]

// translate application messages while retaining external diagnostic details and addresses.
export function localizeDiagnostic(raw: string): string {
  const message = raw.replace(/^Error:\s*/, '')
  if (message.includes('\n')) return message.split('\n').map(localizeDiagnostic).join('\n')
  for (const terminal of ['vataga', 'tiger']) {
    if (message.startsWith(`${terminal}: `))
      return `${terminal === 'vataga' ? 'Vataga' : 'Tiger'}: ${localizeDiagnostic(message.slice(terminal.length + 2))}`
  }
  if (message.startsWith('failed to ')) {
    if (message.endsWith('EOF') || message.endsWith('use of closed network connection'))
      return i18n.t('socketClosed')
    if (message.endsWith('context deadline exceeded')) return i18n.t('deadlineExceeded')
  }
  const key = messages[message]
  if (key) return i18n.t(key)
  for (const [prefix, translation] of prefixes) {
    if (message.startsWith(`${prefix}: `))
      return `${i18n.t(translation)}: ${localizeDiagnostic(message.slice(prefix.length + 2))}`
  }
  const status = message.match(/^terminal returned (status )?(.+)$/)
  if (status) return i18n.t(status[1] ? 'terminalStatus' : 'terminalResponse', { status: status[2] })
  let result = message
  for (const phrase of [
    'use of closed network connection',
    'connection refused',
    'context deadline exceeded',
    'context canceled',
  ]) {
    result = result.replaceAll(phrase, i18n.t(messages[phrase]))
  }
  return result
}
