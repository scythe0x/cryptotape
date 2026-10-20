import * as Tooltip from '@radix-ui/react-tooltip'
import { useTranslation } from '../i18n'
import type { ReactNode } from 'react'

export function HelpTip({ children, label }: { children: ReactNode; label?: string }) {
  const { t } = useTranslation()
  return (
    <Tooltip.Provider delayDuration={0}>
      <Tooltip.Root>
        <Tooltip.Trigger type="button" className="help-trigger" aria-label={label ?? t('help')}>
          ?
        </Tooltip.Trigger>
        <Tooltip.Portal>
          <Tooltip.Content className="help-tip" sideOffset={7} collisionPadding={12}>
            {children}
            <Tooltip.Arrow className="help-arrow" />
          </Tooltip.Content>
        </Tooltip.Portal>
      </Tooltip.Root>
    </Tooltip.Provider>
  )
}
