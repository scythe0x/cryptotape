import { useRef, useState } from 'react'
import { Play } from 'lucide-react'
import { desktop } from '../api'
import { useTranslation, soundLabel } from '../i18n'
import { Button } from './ui'
import { Select } from './Select'

type Props = {
  sound: string
  volume: number
  sounds: string[]
  onChange: (sound: string) => void
  onVolume: (volume: number) => void
  onError: (error: string) => void
}

export function SoundField({ sound, volume, sounds, onChange, onVolume, onError }: Props) {
  const { t } = useTranslation()
  const picker = useRef<HTMLInputElement>(null)
  const [importing, setImporting] = useState(false)
  const [imported, setImported] = useState<string[]>([])

  async function importSound(file: File | undefined) {
    if (!file) return
    setImporting(true)
    try {
      const extension = file.name.slice(file.name.lastIndexOf('.')).toLowerCase()
      if (!['.mp3', '.wav'].includes(extension)) throw new Error('supported formats: mp3, wav')
      if (file.size > 2 * 1024 * 1024) throw new Error('sound must be smaller than 2 MB')
      const encoded = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result).split(',')[1] ?? '')
        reader.onerror = () => reject(reader.error ?? new Error('cannot read sound'))
        reader.readAsDataURL(file)
      })
      const id = await desktop.importSound(file.name.slice(0, -extension.length), extension, encoded)
      setImported((current) => [...current, id])
      onChange(id)
      onError('')
    } catch (error) {
      onError(String(error))
    } finally {
      setImporting(false)
      if (picker.current) picker.current.value = ''
    }
  }

  return (
    <div className="sound-field">
      <label htmlFor="rule-sound">{t('sound')}</label>
      <div className="input-row">
        <Select
          id="rule-sound"
          value={sound}
          disabled={importing}
          onValueChange={(value) => {
            if (value === '__import') picker.current?.click()
            else onChange(value)
          }}
          options={[
            ...[...new Set(['none', ...sounds, ...imported])].map((id) => ({ value: id, label: soundLabel(id) })),
            { value: '__import', label: t('importSound') },
          ]}
        />
        <Button
          type="button"
          size="icon"
          title={t('previewSound')}
          aria-label={t('previewSound')}
          disabled={sound === 'none'}
          onClick={() =>
            void desktop.previewSound(sound, volume).catch((error: unknown) => onError(String(error)))
          }
        >
          <Play size={13} />
        </Button>
      </div>
      <input
        type="file"
        ref={picker}
        accept=".mp3,.wav,audio/mpeg,audio/wav"
        hidden
        onChange={(event) => void importSound(event.target.files?.[0])}
      />
      {sound !== 'none' && (
        <label className="volume-row">
          {t('volume')}
          <input
            aria-label={t('soundVolume')}
            type="range"
            min="0"
            max="100"
            step="1"
            value={Math.round(volume * 100)}
            onChange={(event) => onVolume(Number(event.target.value) / 100)}
          />
          <output>{Math.round(volume * 100)}%</output>
        </label>
      )}
    </div>
  )
}
