import { expect, it } from 'vitest'
import { latencyState, quoteAgeState } from './connection'

it.each([
  [null, 'unknown'], [0, 'good'], [199, 'good'], [200, 'warning'],
  [499, 'warning'], [500, 'poor'], [1500, 'poor'],
] as const)('colors ping %s as %s', (ping, state) => {
  expect(latencyState(ping)).toBe(state)
})

it.each([[null, 'unknown'], [2000, 'good'], [2001, 'warning'], [10000, 'warning'], [10001, 'poor']] as const)(
  'colors quote age %s as %s', (age, state) => expect(quoteAgeState(age)).toBe(state),
)
