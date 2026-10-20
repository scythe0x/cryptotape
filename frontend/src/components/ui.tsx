import type { ButtonHTMLAttributes, InputHTMLAttributes } from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...values: Parameters<typeof clsx>): string {
  return twMerge(clsx(...values))
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'ghost'
  size?: 'default' | 'icon'
}

export function Button({ variant = 'secondary', size = 'default', className, ...props }: ButtonProps) {
  return <button className={cn('button', `button-${variant}`, size === 'icon' && 'button-icon', className)} {...props} />
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={cn('input', props.className)} />
}
