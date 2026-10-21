import type { ComponentPropsWithoutRef, Ref } from 'react'
import * as Primitive from '@radix-ui/react-scroll-area'
import './controls.css'

type Props = ComponentPropsWithoutRef<typeof Primitive.Root> & {
  viewportRef?: Ref<HTMLDivElement>
  viewportClassName?: string
  viewportLabel?: string
  onScroll?: ComponentPropsWithoutRef<typeof Primitive.Viewport>['onScroll']
}

export function ScrollArea({ children, className = '', viewportClassName = '', viewportLabel, viewportRef, onScroll, ...props }: Props) {
  return (
    <Primitive.Root type="auto" className={`scroll-area ${className}`} {...props}>
      <Primitive.Viewport
        ref={viewportRef}
        onScroll={onScroll}
        className={`scroll-viewport ${viewportClassName}`}
        tabIndex={0}
        role={viewportLabel ? 'region' : undefined}
        aria-label={viewportLabel}
      >
        {children}
      </Primitive.Viewport>
      <Primitive.Scrollbar orientation="vertical" className="scroll-bar">
        <Primitive.Thumb className="scroll-thumb" />
      </Primitive.Scrollbar>
      <Primitive.Scrollbar orientation="horizontal" className="scroll-bar">
        <Primitive.Thumb className="scroll-thumb" />
      </Primitive.Scrollbar>
      <Primitive.Corner />
    </Primitive.Root>
  )
}
