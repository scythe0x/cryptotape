import { useEffect, useState } from 'react'
import { Check } from 'lucide-react'
import './controls.css'
import type { ListingSource } from '../api'
import { desktop, events } from '../api'
import { useTranslation } from '../i18n'
import { Button, Input } from './ui'

const freeEndpoint = 'wss://ws.newlistings.pro/v2/full'
const keyBotURL = 'https://t.me/NLF_websocket_bot'

export function ListingSourceSettings() {
  const { t } = useTranslation()
  const [source, setSource] = useState<ListingSource | null>(null)
  const [key, setKey] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let active = true
    void desktop
      .listingSource()
      .then((value) => {
        if (active) setSource(value)
      })
      .catch((cause: unknown) => {
        if (active) setError(String(cause))
      })
    const stop = events.listingHealth((health) =>
      setSource((current) => (current ? { ...current, health } : current)),
    )
    return () => {
      active = false
      stop()
    }
  }, [])

  async function save(clear = false) {
    setBusy(true)
    setError('')
    try {
      if (clear) await desktop.clearListingKey()
      else await desktop.saveListingSource(freeEndpoint, key)
      setKey('')
      setSource(await desktop.listingSource())
    } catch (cause) {
      setError(String(cause))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form
      className="form-stack"
      onSubmit={(event) => {
        event.preventDefault()
        void save()
      }}
    >
      <div className="listing-key-field">
        <div className="listing-key-heading">
          <label htmlFor="listing-key">{t('listingKey')}</label>
          {source?.keyConfigured && <span className="listing-key-saved"><Check size={12} />{t('keySaved')}</span>}
        </div>
        <Input
          id="listing-key"
          type="password"
          autoComplete="new-password"
          spellCheck={false}
          value={key}
          onChange={(event) => setKey(event.target.value)}
          placeholder={source?.keyConfigured ? '*'.repeat(50) : undefined}
          aria-describedby="listing-key-help"
        />
        <p className="muted" id="listing-key-help">
          {t('listingKeyHelp')}{' '}
          <a
            href={keyBotURL}
            onClick={(event) => {
              event.preventDefault()
              void desktop.openListingURL(keyBotURL).catch((cause: unknown) => setError(String(cause)))
            }}
          >
            @NLF_websocket_bot
          </a>
        </p>
      </div>
      {(error || source?.health.state === 'error') && (
        <div className="inline-error">{error || t('listingConnectionError')}</div>
      )}
      <div className="dialog-actions">
        {source?.keyConfigured && (
          <Button type="button" variant="ghost" disabled={busy} onClick={() => void save(true)}>
            {t('removeListingKey')}
          </Button>
        )}
        <Button
          type="submit"
          variant="primary"
          disabled={busy || !source || (!source.keyConfigured && !key.trim())}
        >
          {busy ? t('saving') : t('saveListingSource')}
        </Button>
      </div>
    </form>
  )
}
