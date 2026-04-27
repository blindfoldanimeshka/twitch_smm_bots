import { useState, useRef } from "react"
import { motion, useMotionValue, useSpring, useTransform } from "framer-motion"
import { LayoutDashboard, Bot, Settings } from 'lucide-react'

interface DockItem {
  id: string
  name: string
  icon: React.ReactNode
  color: string
}

interface DockTabsProps {
  currentPage: string
  onNavigate: (page: string) => void
}

function DockIcon({
  item,
  mouseX,
  isActive,
  onClick
}: {
  item: DockItem
  mouseX: any
  isActive: boolean
  onClick: () => void
}) {
  const ref = useRef<HTMLDivElement>(null)

  const distance = useTransform(mouseX, (val: number) => {
    const bounds = ref.current?.getBoundingClientRect() ?? { x: 0, width: 0 }
    return val - bounds.x - bounds.width / 2
  })

  const widthSync = useTransform(distance, [-150, 0, 150], [50, 80, 50])
  const width = useSpring(widthSync, { mass: 0.1, stiffness: 150, damping: 12 })

  const heightSync = useTransform(distance, [-150, 0, 150], [50, 80, 50])
  const height = useSpring(heightSync, { mass: 0.1, stiffness: 150, damping: 12 })

  const [isHovered, setIsHovered] = useState(false)

  return (
    <motion.div
      ref={ref}
      style={{ width, height }}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      onClick={onClick}
      className="aspect-square cursor-pointer flex items-center justify-center relative group"
      whileTap={{ scale: 0.95 }}
    >
      <motion.div
        className={`w-full h-full rounded-2xl shadow-lg flex items-center justify-center text-white relative overflow-hidden ${item.color}`}
        animate={{
          y: isHovered ? -8 : 0,
        }}
        transition={{
          type: "spring",
          stiffness: 400,
          damping: 17,
        }}
      >
        <motion.div
          className="text-xl"
          animate={{
            scale: isHovered ? 1.1 : 1,
          }}
          transition={{
            type: "spring",
            stiffness: 400,
            damping: 17,
          }}
        >
          {item.icon}
        </motion.div>

        {/* Neon glow effect */}
        <motion.div
          className="absolute inset-0 bg-gradient-to-br from-white/20 to-transparent rounded-2xl"
          animate={{
            opacity: isHovered ? 0.3 : 0.1,
          }}
          transition={{ duration: 0.2 }}
        />
      </motion.div>

      {/* Tooltip */}
      <motion.div
        initial={{ opacity: 0, y: 10, scale: 0.8 }}
        animate={{
          opacity: isHovered ? 1 : 0,
          y: isHovered ? -20 : 10,
          scale: isHovered ? 1 : 0.8,
        }}
        transition={{
          type: "spring",
          stiffness: 500,
          damping: 30,
        }}
        className="absolute -top-12 left-1/2 transform -translate-x-1/2 bg-gray-800/90 text-white text-xs px-2 py-1 rounded-md whitespace-nowrap pointer-events-none backdrop-blur-sm"
        style={{
          boxShadow: isActive ? '0 0 8px rgba(115,115,115,0.5)' : 'none',
          borderColor: isActive ? 'rgba(115,115,115,0.5)' : 'transparent',
          borderWidth: '1px',
        }}
      >
        {item.name}
      </motion.div>

      {/* Active indicator dot */}
      {isActive && (
        <motion.div
          className="absolute -bottom-1 left-1/2 transform -translate-x-1/2 w-1.5 h-1.5 rounded-full"
          style={{
            backgroundColor: '#737373',
            boxShadow: '0 0 8px rgba(115,115,115,0.8), 0 0 16px rgba(115,115,115,0.4)',
          }}
          animate={{
            scale: [1, 1.2, 1],
            opacity: [0.8, 1, 0.8],
          }}
          transition={{
            duration: 2,
            repeat: Infinity,
            ease: "easeInOut",
          }}
        />
      )}
    </motion.div>
  )
}

export function DockTabs({ currentPage, onNavigate }: DockTabsProps) {
  const mouseX = useMotionValue(Infinity)

  const dockItems: DockItem[] = [
    {
      id: "dashboard",
      name: "Дашборд",
      icon: <LayoutDashboard />,
      color: "bg-gradient-to-br from-gray-700 to-gray-900"
    },
    {
      id: "bots",
      name: "Боты",
      icon: <Bot />,
      color: "bg-gradient-to-br from-gray-600 to-gray-800"
    },
    {
      id: "settings",
      name: "Настройки",
      icon: <Settings />,
      color: "bg-gradient-to-br from-gray-700 to-gray-900"
    },
  ]

  return (
    <div className="fixed bottom-8 left-1/2 transform -translate-x-1/2 z-50">
      <motion.div
        onMouseMove={(e) => mouseX.set(e.pageX)}
        onMouseLeave={() => mouseX.set(Infinity)}
        className="flex h-20 items-end gap-4 rounded-3xl px-4 pb-3.5 border-2 shadow-xl"
        style={{
          backgroundColor: 'rgba(25, 25, 25, 0.8)',
          backdropFilter: 'blur(12px)',
          borderColor: 'rgba(82, 82, 82, 0.3)',
          boxShadow: '0 0 24px rgba(115, 115, 115, 0.15), 0 8px 32px rgba(0, 0, 0, 0.4)',
        }}
        initial={{ y: 100, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        transition={{
          type: "spring",
          stiffness: 260,
          damping: 20,
          delay: 0.1,
        }}
      >
        {dockItems.map((item) => (
          <DockIcon
            key={item.id}
            item={item}
            mouseX={mouseX}
            isActive={currentPage === item.id}
            onClick={() => onNavigate(item.id)}
          />
        ))}
      </motion.div>
    </div>
  )
}
