import * as React from "react"
import * as SwitchPrimitives from "@radix-ui/react-switch"

const Switch = React.forwardRef<
  React.ElementRef<typeof SwitchPrimitives.Root>,
  React.ComponentPropsWithoutRef<typeof SwitchPrimitives.Root>
>(({ className, ...props }, ref) => (
  <SwitchPrimitives.Root
    className={`
      peer inline-flex h-6 w-11 shrink-0 cursor-pointer items-center
      border-2 border-transparent transition-colors
      focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-purple-500 focus-visible:ring-offset-2 focus-visible:ring-offset-neutral-900
      disabled:cursor-not-allowed disabled:opacity-50
      data-[state=checked]:bg-purple-600 data-[state=unchecked]:bg-neutral-800
      ${className}
    `}
    style={{ borderRadius: '4px' }}
    {...props}
    ref={ref}
  >
    <SwitchPrimitives.Thumb
      className={`
        pointer-events-none block h-5 w-5 bg-white shadow-lg
        transition-transform data-[state=checked]:translate-x-5 data-[state=unchecked]:translate-x-0
      `}
      style={{ borderRadius: '2px' }}
    />
  </SwitchPrimitives.Root>
))
Switch.displayName = SwitchPrimitives.Root.displayName

export { Switch }
