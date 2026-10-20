import React from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { Overlay } from './Overlay'
import './style.css'

const overlay = new URLSearchParams(window.location.search).has('overlay')
document.documentElement.classList.toggle('overlay-page', overlay)

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>{overlay ? <Overlay /> : <App />}</React.StrictMode>,
)
