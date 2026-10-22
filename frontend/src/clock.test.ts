import { describe, expect, it } from 'vitest'
import { candleWarning, clockDisplay, clockTimezones } from './clock'

const utc = (time: string) => Date.parse(`2026-10-21T${time}Z`)

describe('candle warnings', () => {
  it('switches every second in the last five seconds and resets at the exact boundary', () => {
    for (const [time, yellow] of [
      ['12:04:54.999', false], ['12:04:55.000', true], ['12:04:55.999', true],
      ['12:04:56.000', false], ['12:04:57.000', true], ['12:04:58.000', false],
      ['12:04:59.999', true], ['12:05:00.000', false], ['12:05:01.000', false],
      ['12:05:55.000', false],
    ] as const) expect(candleWarning(utc(time)).yellow, time).toBe(yellow)
  })

  it.each([
    ['12:59:44.999', ''], ['12:59:45.000', '1 HOUR'], ['12:59:59.999', '1 HOUR'],
    ['13:00:00.000', ''], ['15:59:45.000', '4 HOURS'], ['19:59:59.999', '4 HOURS'],
    ['23:59:44.999', ''], ['23:59:45.000', '1 DAY'], ['23:59:59.999', '1 DAY'],
    ['00:00:00.000', ''], ['07:59:45.000', '4 HOURS'],
  ])('uses UTC boundaries and the largest timeframe at %s', (time, label) => {
    expect(candleWarning(utc(time)).label).toBe(label)
  })

  it('keeps flashing independently of the large timeframe label', () => {
    expect(candleWarning(utc('23:59:55'))).toEqual({ yellow: true, label: '1 DAY' })
    expect(candleWarning(utc('23:59:56'))).toEqual({ yellow: false, label: '1 DAY' })
  })
})

it('changes only displayed time, including fractional offsets and date rollover', () => {
  expect(clockDisplay(utc('23:59:55'), 'UTC300')).toEqual({ time: '04:59:55', zone: 'UTC+5' })
  expect(clockDisplay(utc('23:59:55'), 'UTC345')).toEqual({ time: '05:44:55', zone: 'UTC+5:45' })
  expect(clockDisplay(utc('00:00:00'), 'UTC-210')).toEqual({ time: '20:30:00', zone: 'UTC−3:30' })
  expect(clockDisplay(utc('23:59:55'), 'UTC0')).toEqual({ time: '23:59:55', zone: 'UTC' })
  const system = clockDisplay(utc('12:00:00'), 'system')
  const date = new Date(utc('12:00:00'))
  expect(system.time).toBe([date.getHours(), date.getMinutes(), date.getSeconds()].map((part) => String(part).padStart(2, '0')).join(':'))
  expect(clockTimezones[0].value).toBe('UTC-720')
  expect(clockTimezones.at(-1)?.value).toBe('UTC840')
})

