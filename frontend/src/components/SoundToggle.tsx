import { useState } from 'react'
import { Volume2, VolumeX } from 'lucide-react'
import type { Settings } from '../api'
import { desktop } from '../api'
import { useTranslation } from '../i18n'
import { Button } from './ui'

export function SoundToggle({ settings, onError }: {
  settings: Settings | null
  onError: (message: string) => void
}) {
  const { t } = useTranslation()
  const [saving, setSaving] = useState(false)
  const muted = settings?.soundMuted ?? false
  const label = t(muted ? 'unmuteSounds' : 'muteSounds')

  async function toggle() {
    if (!settings) return
    setSaving(true)
    try {
      await desktop.setSoundMuted(!muted)
    } catch (error) {
      onError(String(error))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Button
      size="icon"
      variant="ghost"
      className={muted ? 'sound-muted' : 'sound-enabled'}
      aria-label={label}
      aria-pressed={muted}
      title={label}
      disabled={!settings || saving}
      onClick={() => void toggle()}
    >
      {muted ? <VolumeX size={15} /> : <Volume2 size={15} />}
    </Button>
  )
}
