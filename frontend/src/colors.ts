import type { Rule } from './api'

export const alertColors = [
  ['red', '#fa423e'],
  ['orange', '#f2994a'],
  ['yellow', '#e7c95c'],
  ['green', '#41c977'],
  ['cyan', '#46c6df'],
  ['blue', '#628cf0'],
  ['violet', '#a78bfa'],
  ['pink', '#e879b9'],
  ['neutral', '#c5c5c5'],
] as const

export function nextAlertColor(rules: readonly Pick<Rule, 'color'>[]): string {
  const used = new Set(rules.map((rule) => rule.color.toLowerCase()))
  return alertColors.find(([, color]) => !used.has(color))?.[1] ?? '#ffffff'
}
