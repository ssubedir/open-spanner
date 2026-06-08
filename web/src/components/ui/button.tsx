import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '../../lib/utils'

const buttonVariants = cva('button', {
  defaultVariants: {
    size: 'default',
    variant: 'default',
  },
  variants: {
    size: {
      default: 'button-default',
      icon: 'button-icon',
      sm: 'button-sm',
    },
    variant: {
      default: 'button-primary',
      ghost: 'button-ghost',
      outline: 'button-outline',
      secondary: 'button-secondary',
    },
  },
})

export type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants>

export function Button({ className, size, variant, ...props }: ButtonProps) {
  return <button className={cn(buttonVariants({ size, variant }), className)} {...props} />
}
