import React, { lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import './style.css'

const overlay = new URLSearchParams(window.location.search).has('overlay')
document.documentElement.classList.toggle('overlay-page', overlay)

const View = lazy(overlay
  ? () => import('./Overlay').then(({ Overlay }) => ({ default: Overlay }))
  : () => import('./App').then(({ App }) => ({ default: App })))

createRoot(document.getElementById('root')!).render(
  <React.StrictMode><Suspense fallback={null}><View /></Suspense></React.StrictMode>,
)
