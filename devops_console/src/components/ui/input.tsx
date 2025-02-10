import { Box } from "@radix-ui/themes"
import { ComponentProps, forwardRef } from "react"

export type InputProps = ComponentProps<'input'> & {
  className?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, ...props }, ref) => {
    return (
      <Box>
        <input
          className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
          ref={ref}
          {...props}
        />
      </Box>
    )
  }
)

Input.displayName = "Input"
