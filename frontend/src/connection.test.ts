import { expect, it } from 'vitest'
import { latencyState } from './connection'

it.each([
  [null, 'unknown'], [0, 'good'], [199, 'good'], [200, 'warning'],
  [499, 'warning'], [500, 'poor'], [1500, 'poor'],
] as const)('colors ping %s as %s', (ping, state) => {
  expect(latencyState(ping)).toBe(state)
})
