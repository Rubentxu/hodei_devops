import { ScrollArea as RadixScrollArea } from "@radix-ui/themes"
import { ComponentProps, forwardRef } from "react"

export type ScrollAreaProps = ComponentProps<typeof RadixScrollArea>

export const ScrollArea = forwardRef<HTMLDivElement, ScrollAreaProps>(
  ({ children, ...props }, ref) => {
    return (
      <RadixScrollArea ref={ref} {...props}>
        {children}
      </RadixScrollArea>
    )
  }
)
