import type { Rule } from '../api'
import { useTranslation } from '../i18n'

const options = [
  { seconds: 0, label: 'bannerHidden' },
  { seconds: 5, label: 'banner5Seconds' },
  { seconds: 15, label: 'banner15Seconds' },
  { seconds: 30, label: 'banner30Seconds' },
  { seconds: 60, label: 'banner1Minute' },
  { seconds: 300, label: 'banner5Minutes' },
  { seconds: -1, label: 'bannerUntilClosed' },
] as const

type Props = {
  value: Rule['bannerSeconds']
  onChange: (value: Rule['bannerSeconds']) => void
}

export function BannerDurationField({ value, onChange }: Props) {
  const { t } = useTranslation()
  const index = options.findIndex((option) => option.seconds === value)
  const label = options[index]?.label ?? 'bannerUntilClosed'

  return (
    <div className="banner-duration-field">
      <div className="banner-duration-heading">
        <label htmlFor="rule-banner-duration">{t('bannerDuration')}</label>
        <output htmlFor="rule-banner-duration">{t(label)}</output>
      </div>
      <input
        id="rule-banner-duration"
        type="range"
        min="0"
        max={options.length - 1}
        step="1"
        value={index}
        aria-valuetext={t(label)}
        onChange={(event) => {
          const option = options[Number(event.target.value)]
          if (option) onChange(option.seconds)
        }}
      />
      <div className="banner-duration-steps" aria-hidden="true">
        {options.map((option) => <span key={option.seconds} title={t(option.label)} />)}
      </div>
    </div>
  )
}
