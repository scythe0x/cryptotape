import type { ButtonHTMLAttributes, ComponentProps } from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

function cn(...values: Parameters<typeof clsx>): string {
  return twMerge(clsx(...values))
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'ghost'
  size?: 'default' | 'icon'
}

export function Button({ variant = 'secondary', size = 'default', className, ...props }: ButtonProps) {
  return <button className={cn('button', `button-${variant}`, size === 'icon' && 'button-icon', className)} {...props} />
}

export function Input(props: ComponentProps<'input'>) {
  return <input {...props} className={cn('input', props.className)} />
}

export function Checkbox(props: Omit<ComponentProps<'input'>, 'type'>) {
  return <input {...props} type="checkbox" className={cn('checkbox', props.className)} />
}
