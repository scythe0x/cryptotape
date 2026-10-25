import React, { lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import './style.css'

const params = new URLSearchParams(window.location.search)
const overlay = params.has('overlay')
const settings = params.has('settings')
const clock = params.has('clock')
document.documentElement.classList.toggle('clock-page', clock)
document.documentElement.classList.toggle('overlay-page', overlay)
document.documentElement.classList.toggle('settings-window-page', settings)

const View = lazy(clock
  ? () => import('./ClockWindow').then(({ ClockWindow }) => ({ default: ClockWindow }))
  : overlay
  ? () => import('./Overlay').then(({ Overlay }) => ({ default: Overlay }))
  : settings
    ? () => import('./SettingsWindow').then(({ SettingsWindow }) => ({ default: SettingsWindow }))
    : () => import('./App').then(({ App }) => ({ default: App })))

createRoot(document.getElementById('root')!).render(
  <React.StrictMode><Suspense fallback={null}><View /></Suspense></React.StrictMode>,
)
