import { Text } from "@radix-ui/themes"
import { ComponentProps, forwardRef } from "react"

export type LabelProps = ComponentProps<typeof Text>

export const Label = forwardRef<HTMLLabelElement, LabelProps>(
  ({ children, ...props }, ref) => {
    return (
      <Text as="label" size="2" weight="bold" ref={ref} {...props}>
        {children}
      </Text>
    )
  }
)
