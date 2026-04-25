import { useRef, useEffect, forwardRef, type ReactNode } from 'react'

interface GridBeamProps {
  rows: number
  cols: number
  colorVariant?: 'colorful' | 'ocean' | 'neon'
  theme?: 'dark' | 'light'
  active?: boolean
  borderRadius?: number
  className?: string
  id?: string
  children: ReactNode
}

interface UseGridBeamOptions {
  rows: number
  cols: number
  colorVariant?: 'colorful' | 'ocean' | 'neon'
  theme?: 'dark' | 'light'
  active?: boolean
}

export function GridBeam({
  rows,
  cols,
  colorVariant = 'neon',
  active = true,
  borderRadius = 8,
  className = '',
  children,
}: GridBeamProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    if (!active || !canvasRef.current) return

    const canvas = canvasRef.current
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = window.devicePixelRatio || 1
    const rect = canvas.getBoundingClientRect()

    canvas.width = rect.width * dpr
    canvas.height = rect.height * dpr
    ctx.scale(dpr, dpr)

    const cellWidth = rect.width / cols
    const cellHeight = rect.height / rows

    let animationFrame: number
    let beams: Array<{
      row: number
      col: number
      direction: 'horizontal' | 'vertical'
      progress: number
      speed: number
      color: string
    }> = []

    const colors = {
      neon: ['#39FF6A', '#72FF8D', '#55E7FF'],
      ocean: ['#00D4FF', '#0099CC', '#0066FF'],
      colorful: ['#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A'],
    }

    const palette = colors[colorVariant]

    // Создаём начальные лучи
    const createBeam = () => {
      const isHorizontal = Math.random() > 0.5
      return {
        row: Math.floor(Math.random() * rows),
        col: Math.floor(Math.random() * cols),
        direction: (isHorizontal ? 'horizontal' : 'vertical') as 'horizontal' | 'vertical',
        progress: 0,
        speed: 0.02 + Math.random() * 0.03,
        color: palette[Math.floor(Math.random() * palette.length)],
      }
    }

    // Инициализация
    for (let i = 0; i < 3; i++) {
      beams.push(createBeam())
    }

    const animate = () => {
      ctx.clearRect(0, 0, rect.width, rect.height)

      // Обновляем и рисуем лучи
      beams = beams.filter((beam) => {
        beam.progress += beam.speed

        if (beam.progress > 1) return false

        const alpha = Math.sin(beam.progress * Math.PI) * 0.6

        ctx.save()
        ctx.globalAlpha = alpha

        if (beam.direction === 'horizontal') {
          const y = beam.row * cellHeight + cellHeight / 2
          const x = beam.progress * rect.width

          const gradient = ctx.createLinearGradient(x - 50, y, x + 50, y)
          gradient.addColorStop(0, 'transparent')
          gradient.addColorStop(0.5, beam.color)
          gradient.addColorStop(1, 'transparent')

          ctx.strokeStyle = gradient
          ctx.lineWidth = 2
          ctx.beginPath()
          ctx.moveTo(Math.max(0, x - 50), y)
          ctx.lineTo(Math.min(rect.width, x + 50), y)
          ctx.stroke()

          // Glow effect
          ctx.shadowBlur = 15
          ctx.shadowColor = beam.color
          ctx.stroke()
        } else {
          const x = beam.col * cellWidth + cellWidth / 2
          const y = beam.progress * rect.height

          const gradient = ctx.createLinearGradient(x, y - 50, x, y + 50)
          gradient.addColorStop(0, 'transparent')
          gradient.addColorStop(0.5, beam.color)
          gradient.addColorStop(1, 'transparent')

          ctx.strokeStyle = gradient
          ctx.lineWidth = 2
          ctx.beginPath()
          ctx.moveTo(x, Math.max(0, y - 50))
          ctx.lineTo(x, Math.min(rect.height, y + 50))
          ctx.stroke()

          // Glow effect
          ctx.shadowBlur = 15
          ctx.shadowColor = beam.color
          ctx.stroke()
        }

        ctx.restore()
        return true
      })

      // Добавляем новые лучи случайно
      if (Math.random() < 0.03 && beams.length < 5) {
        beams.push(createBeam())
      }

      animationFrame = requestAnimationFrame(animate)
    }

    animate()

    return () => {
      cancelAnimationFrame(animationFrame)
    }
  }, [rows, cols, colorVariant, active])

  return (
    <div className={className} style={{ position: 'relative', overflow: 'hidden', borderRadius: `${borderRadius}px` }}>
      <GridBeamDividers rows={rows} cols={cols} />
      <canvas
        ref={canvasRef}
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          pointerEvents: 'none',
          zIndex: 1,
        }}
      />
      <div style={{ position: 'relative', zIndex: 2 }}>{children}</div>
    </div>
  )
}

export function GridBeamDividers({ rows, cols }: { rows: number; cols: number }) {
  return (
    <div
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        width: '100%',
        height: '100%',
        pointerEvents: 'none',
        zIndex: 0,
      }}
    >
      {/* Horizontal lines */}
      {Array.from({ length: rows - 1 }).map((_, i) => (
        <div
          key={`h-${i}`}
          style={{
            position: 'absolute',
            top: `${((i + 1) / rows) * 100}%`,
            left: 0,
            width: '100%',
            height: '1px',
            background: 'var(--border-subtle)',
            opacity: 0.3,
          }}
        />
      ))}
      {/* Vertical lines */}
      {Array.from({ length: cols - 1 }).map((_, i) => (
        <div
          key={`v-${i}`}
          style={{
            position: 'absolute',
            left: `${((i + 1) / cols) * 100}%`,
            top: 0,
            width: '1px',
            height: '100%',
            background: 'var(--border-subtle)',
            opacity: 0.3,
          }}
        />
      ))}
    </div>
  )
}

export const GridBeamCanvas = forwardRef<HTMLCanvasElement, { borderRadius?: number }>(
  ({ borderRadius = 8 }, ref) => {
    return (
      <canvas
        ref={ref}
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          pointerEvents: 'none',
          zIndex: 1,
          borderRadius: `${borderRadius}px`,
        }}
      />
    )
  }
)

GridBeamCanvas.displayName = 'GridBeamCanvas'

export function GridBeamContent({ children }: { children: ReactNode }) {
  return <div style={{ position: 'relative', zIndex: 2 }}>{children}</div>
}

export function useGridBeam({
  rows,
  cols,
  colorVariant = 'neon',
  active = true,
}: UseGridBeamOptions) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    if (!active || !canvasRef.current) return

    const canvas = canvasRef.current
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = window.devicePixelRatio || 1
    const rect = canvas.getBoundingClientRect()

    canvas.width = rect.width * dpr
    canvas.height = rect.height * dpr
    ctx.scale(dpr, dpr)

    const cellWidth = rect.width / cols
    const cellHeight = rect.height / rows

    let animationFrame: number
    let beams: Array<{
      row: number
      col: number
      direction: 'horizontal' | 'vertical'
      progress: number
      speed: number
      color: string
    }> = []

    const colors = {
      neon: ['#39FF6A', '#72FF8D', '#55E7FF'],
      ocean: ['#00D4FF', '#0099CC', '#0066FF'],
      colorful: ['#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A'],
    }

    const palette = colors[colorVariant]

    const createBeam = () => {
      const isHorizontal = Math.random() > 0.5
      return {
        row: Math.floor(Math.random() * rows),
        col: Math.floor(Math.random() * cols),
        direction: (isHorizontal ? 'horizontal' : 'vertical') as 'horizontal' | 'vertical',
        progress: 0,
        speed: 0.02 + Math.random() * 0.03,
        color: palette[Math.floor(Math.random() * palette.length)],
      }
    }

    for (let i = 0; i < 3; i++) {
      beams.push(createBeam())
    }

    const animate = () => {
      ctx.clearRect(0, 0, rect.width, rect.height)

      beams = beams.filter((beam) => {
        beam.progress += beam.speed
        if (beam.progress > 1) return false

        const alpha = Math.sin(beam.progress * Math.PI) * 0.6

        ctx.save()
        ctx.globalAlpha = alpha

        if (beam.direction === 'horizontal') {
          const y = beam.row * cellHeight + cellHeight / 2
          const x = beam.progress * rect.width

          const gradient = ctx.createLinearGradient(x - 50, y, x + 50, y)
          gradient.addColorStop(0, 'transparent')
          gradient.addColorStop(0.5, beam.color)
          gradient.addColorStop(1, 'transparent')

          ctx.strokeStyle = gradient
          ctx.lineWidth = 2
          ctx.shadowBlur = 15
          ctx.shadowColor = beam.color
          ctx.beginPath()
          ctx.moveTo(Math.max(0, x - 50), y)
          ctx.lineTo(Math.min(rect.width, x + 50), y)
          ctx.stroke()
        } else {
          const x = beam.col * cellWidth + cellWidth / 2
          const y = beam.progress * rect.height

          const gradient = ctx.createLinearGradient(x, y - 50, x, y + 50)
          gradient.addColorStop(0, 'transparent')
          gradient.addColorStop(0.5, beam.color)
          gradient.addColorStop(1, 'transparent')

          ctx.strokeStyle = gradient
          ctx.lineWidth = 2
          ctx.shadowBlur = 15
          ctx.shadowColor = beam.color
          ctx.beginPath()
          ctx.moveTo(x, Math.max(0, y - 50))
          ctx.lineTo(x, Math.min(rect.height, y + 50))
          ctx.stroke()
        }

        ctx.restore()
        return true
      })

      if (Math.random() < 0.03 && beams.length < 5) {
        beams.push(createBeam())
      }

      animationFrame = requestAnimationFrame(animate)
    }

    animate()

    return () => {
      cancelAnimationFrame(animationFrame)
    }
  }, [rows, cols, colorVariant, active])

  return { canvasRef, rows, cols }
}
