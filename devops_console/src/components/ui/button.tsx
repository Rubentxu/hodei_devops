import { Button as RadixButton } from "@radix-ui/themes"
import { ComponentProps, forwardRef } from "react"

export type ButtonProps = ComponentProps<typeof RadixButton>

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ children, ...props }, ref) => {
    return (
      <RadixButton ref={ref} {...props}>
        {children}
      </RadixButton>
    )
  }
)
