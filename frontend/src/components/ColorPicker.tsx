import * as Popover from '@radix-ui/react-popover'
import { useTranslation } from '../i18n'
import { Check } from 'lucide-react'
import { alertColors } from '../colors'

export function ColorPicker({ value, onChange }: { value: string; onChange: (color: string) => void }) {
  const { t } = useTranslation()
  return (
    <Popover.Root>
      <Popover.Trigger
        type="button"
        className="color-trigger"
        aria-label={t('alertColor')}
        style={{ backgroundColor: value }}
      />
      <Popover.Portal>
        <Popover.Content
          className="color-palette"
          sideOffset={8}
          align="end"
          collisionPadding={12}
          aria-label={t('alertColors')}
        >
          {alertColors.map(([name, color]) => (
            <Popover.Close asChild key={color}>
              <button
                type="button"
                className="color-swatch"
                style={{ backgroundColor: color }}
                aria-label={t(name)}
                aria-pressed={value.toLowerCase() === color}
                onClick={() => onChange(color)}
              >
                {value.toLowerCase() === color && <Check size={15} />}
              </button>
            </Popover.Close>
          ))}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
