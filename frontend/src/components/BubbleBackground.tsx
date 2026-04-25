import { useEffect, useRef } from 'react'
import './BubbleBackground.css'

interface BubbleBackgroundProps {
  interactive?: boolean
  className?: string
}

class Bubble {
  x: number
  y: number
  radius: number
  color: string
  vx: number
  vy: number
  targetX: number
  targetY: number
  canvasWidth: number
  canvasHeight: number

  constructor(canvasWidth: number, canvasHeight: number, colors: string[]) {
    this.canvasWidth = canvasWidth
    this.canvasHeight = canvasHeight
    this.x = Math.random() * canvasWidth
    this.y = Math.random() * canvasHeight
    this.radius = Math.random() * 60 + 20
    this.color = colors[Math.floor(Math.random() * colors.length)]
    this.vx = (Math.random() - 0.5) * 0.5
    this.vy = (Math.random() - 0.5) * 0.5
    this.targetX = this.x
    this.targetY = this.y
  }

  update(mouseX: number, mouseY: number, isInteractive: boolean) {
    // Interactive mode: bubbles react to mouse
    if (isInteractive) {
      const dx = mouseX - this.x
      const dy = mouseY - this.y
      const distance = Math.sqrt(dx * dx + dy * dy)
      const maxDistance = 150

      if (distance < maxDistance) {
        const force = (maxDistance - distance) / maxDistance
        this.targetX = this.x - (dx / distance) * force * 50
        this.targetY = this.y - (dy / distance) * force * 50
      } else {
        this.targetX = this.x
        this.targetY = this.y
      }

      // Smooth movement towards target
      this.x += (this.targetX - this.x) * 0.1
      this.y += (this.targetY - this.y) * 0.1
    }

    // Normal floating movement
    this.x += this.vx
    this.y += this.vy

    // Bounce off edges
    if (this.x < -this.radius) this.x = this.canvasWidth + this.radius
    if (this.x > this.canvasWidth + this.radius) this.x = -this.radius
    if (this.y < -this.radius) this.y = this.canvasHeight + this.radius
    if (this.y > this.canvasHeight + this.radius) this.y = -this.radius
  }

  draw(ctx: CanvasRenderingContext2D) {
    ctx.beginPath()
    ctx.arc(this.x, this.y, this.radius, 0, Math.PI * 2)

    // Gradient fill
    const gradient = ctx.createRadialGradient(
      this.x - this.radius * 0.3,
      this.y - this.radius * 0.3,
      0,
      this.x,
      this.y,
      this.radius
    )
    gradient.addColorStop(0, this.color.replace(/[\d.]+\)$/, '0.8)'))
    gradient.addColorStop(1, this.color.replace(/[\d.]+\)$/, '0.1)'))

    ctx.fillStyle = gradient
    ctx.fill()

    // Glow effect
    ctx.shadowBlur = 30
    ctx.shadowColor = this.color
    ctx.fill()
    ctx.shadowBlur = 0
  }
}

export function BubbleBackground({ interactive = true, className = '' }: BubbleBackgroundProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const bubblesRef = useRef<Bubble[]>([])
  const mouseRef = useRef({ x: 0, y: 0 })
  const animationFrameRef = useRef<number | undefined>(undefined)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const ctx = canvas.getContext('2d')
    if (!ctx) return

    // Twitch color palette
    const colors = [
      'rgba(145, 70, 255, 0.6)',   // Twitch purple
      'rgba(169, 112, 255, 0.5)',  // Light purple
      'rgba(100, 65, 165, 0.6)',   // Dark purple
      'rgba(191, 148, 255, 0.4)',  // Pastel purple
      'rgba(83, 53, 138, 0.5)',    // Deep purple
    ]

    const resize = () => {
      if (!canvas) return
      canvas.width = canvas.offsetWidth
      canvas.height = canvas.offsetHeight

      // Recreate bubbles on resize
      const bubbleCount = Math.floor((canvas.width * canvas.height) / 15000)
      bubblesRef.current = Array.from(
        { length: Math.max(8, Math.min(bubbleCount, 20)) },
        () => new Bubble(canvas.width, canvas.height, colors)
      )
    }

    resize()
    window.addEventListener('resize', resize)

    const handleMouseMove = (e: MouseEvent) => {
      if (!canvas) return
      const rect = canvas.getBoundingClientRect()
      mouseRef.current.x = e.clientX - rect.left
      mouseRef.current.y = e.clientY - rect.top
    }

    if (interactive) {
      canvas.addEventListener('mousemove', handleMouseMove)
    }

    const animate = () => {
      if (!canvas || !ctx) return
      ctx.clearRect(0, 0, canvas.width, canvas.height)

      bubblesRef.current.forEach((bubble) => {
        bubble.update(mouseRef.current.x, mouseRef.current.y, interactive)
        bubble.draw(ctx)
      })

      animationFrameRef.current = requestAnimationFrame(animate)
    }

    animate()

    return () => {
      window.removeEventListener('resize', resize)
      if (interactive && canvas) {
        canvas.removeEventListener('mousemove', handleMouseMove)
      }
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current)
      }
    }
  }, [interactive])

  return (
    <canvas
      ref={canvasRef}
      className={`bubble-background ${className}`}
    />
  )
}
