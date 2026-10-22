import React, { lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import './style.css'
import { LicenseBoundary } from './LicenseBoundary'

const params = new URLSearchParams(window.location.search)
const activation = params.has('activation')
const overlay = params.has('overlay')
const settings = params.has('settings')
const clock = params.has('clock')
document.documentElement.classList.toggle('clock-page', clock)
document.documentElement.classList.toggle('activation-page', activation)
document.documentElement.classList.toggle('overlay-page', overlay)
document.documentElement.classList.toggle('settings-window-page', settings)

const View = lazy(activation
  ? () => import('./Activation').then(({ Activation }) => ({ default: Activation }))
  : clock
  ? () => import('./ClockWindow').then(({ ClockWindow }) => ({ default: ClockWindow }))
  : overlay
  ? () => import('./Overlay').then(({ Overlay }) => ({ default: Overlay }))
  : settings
    ? () => import('./SettingsWindow').then(({ SettingsWindow }) => ({ default: SettingsWindow }))
    : () => import('./App').then(({ App }) => ({ default: App })))

createRoot(document.getElementById('root')!).render(
  <React.StrictMode><Suspense fallback={null}>{activation ? <View /> : <LicenseBoundary><View /></LicenseBoundary>}</Suspense></React.StrictMode>,
)
