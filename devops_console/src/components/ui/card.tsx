import { Box, Card as RadixCard, Flex, Heading, Text } from "@radix-ui/themes"
import { ComponentProps } from "react"

export const Card = ({ children, ...props }: ComponentProps<typeof RadixCard>) => {
  return <RadixCard {...props}>{children}</RadixCard>
}

export const CardHeader = ({ children, className, ...props }: ComponentProps<typeof Box>) => {
  return (
    <Box className="p-6 pb-4" {...props}>
      {children}
    </Box>
  )
}

export const CardTitle = ({ children, ...props }: ComponentProps<typeof Heading>) => {
  return <Heading size="4" mb="1" {...props}>{children}</Heading>
}

export const CardDescription = ({ children, ...props }: ComponentProps<typeof Text>) => {
  return <Text size="2" color="gray" {...props}>{children}</Text>
}

export const CardContent = ({ children, className, ...props }: ComponentProps<typeof Box>) => {
  return (
    <Box className="p-6 pt-0" {...props}>
      {children}
    </Box>
  )
}

export const CardFooter = ({ children, className, ...props }: ComponentProps<typeof Flex>) => {
  return (
    <Flex className="p-6 pt-0" justify="between" gap="3" {...props}>
      {children}
    </Flex>
  )
}
