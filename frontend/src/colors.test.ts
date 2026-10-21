import { expect, it } from 'vitest'
import { alertColors, nextAlertColor } from './colors'

it('chooses the first unused palette color regardless of letter case', () => {
  expect(nextAlertColor([])).toBe(alertColors[0][1])
  expect(nextAlertColor([{ color: '#FA423E' }, { color: '#f2994a' }])).toBe('#e7c95c')
})

it('falls back to white when every palette color is already used', () => {
  expect(nextAlertColor(alertColors.map(([, color]) => ({ color })))).toBe('#ffffff')
})
