import { useEffect } from 'react'
import type { Alert } from '../api'

type Props = {
  id: number
  seconds: Alert['bannerSeconds']
  onClose: (id: number) => void
}

export function BannerTimer({ id, seconds, onClose }: Props) {
  useEffect(() => {
    if (seconds <= 0) return
    const timer = window.setTimeout(() => onClose(id), seconds * 1000)
    return () => window.clearTimeout(timer)
  }, [id, seconds, onClose])

  return null
}
