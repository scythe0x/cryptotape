import { forwardRef, type ComponentPropsWithoutRef } from 'react'
import * as Primitive from '@radix-ui/react-select'
import { Check, ChevronDown } from 'lucide-react'
import * as ScrollPrimitive from '@radix-ui/react-scroll-area'
import './controls.css'

type Props = {
  id?: string
  value: string
  onValueChange: (value: string) => void
  options: readonly { value: string; label: string }[]
  disabled?: boolean
  'aria-label'?: string
}

const emptyValue = '__select_empty__'

export function Select({ id, value, onValueChange, options, disabled, 'aria-label': ariaLabel }: Props) {
  return (
    <Primitive.Root value={value || emptyValue} onValueChange={(next) => onValueChange(next === emptyValue ? '' : next)} disabled={disabled}>
      <Primitive.Trigger id={id} className="input select-trigger" aria-label={ariaLabel}>
        <Primitive.Value />
        <Primitive.Icon><ChevronDown size={13} /></Primitive.Icon>
      </Primitive.Trigger>
      <Primitive.Portal>
        <Primitive.Content className="select-content" position="popper" sideOffset={4} collisionPadding={16}>
          <ScrollPrimitive.Root type="auto" className="scroll-area select-scroll-area">
            <Primitive.Viewport asChild>
              <SelectScrollViewport className="select-viewport">
                {options.map((option) => (
                  <Primitive.Item key={option.value} value={option.value || emptyValue} className="select-item">
                    <Primitive.ItemText>{option.label}</Primitive.ItemText>
                    <Primitive.ItemIndicator><Check size={12} /></Primitive.ItemIndicator>
                  </Primitive.Item>
                ))}
              </SelectScrollViewport>
            </Primitive.Viewport>
            <ScrollPrimitive.Scrollbar orientation="vertical" className="scroll-bar">
              <ScrollPrimitive.Thumb className="scroll-thumb" />
            </ScrollPrimitive.Scrollbar>
          </ScrollPrimitive.Root>
        </Primitive.Content>
      </Primitive.Portal>
    </Primitive.Root>
  )
}

const SelectScrollViewport = forwardRef<HTMLDivElement, ComponentPropsWithoutRef<typeof ScrollPrimitive.Viewport>>(
  function SelectScrollViewport({ style, ...props }, ref) {
    // select supplies an overflow shorthand; scroll area owns both axes.
    const viewportStyle = { ...style }
    delete viewportStyle.overflow
    return <ScrollPrimitive.Viewport {...props} ref={ref} style={viewportStyle} />
  },
)
