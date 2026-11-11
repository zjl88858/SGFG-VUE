const COLORS = ['#34d399', '#6ee7b7', '#a7f3d0', '#f9a8d4', '#f472b6', '#fcd34d', '#93c5fd']

export function candyBurst(x: number, y: number) {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  const count = 5 + Math.floor(Math.random() * 3)
  for (let i = 0; i < count; i++) {
    const dot = document.createElement('div')
    const size = 3 + Math.random() * 4
    const color = COLORS[Math.floor(Math.random() * COLORS.length)]
    dot.style.cssText = `position:fixed;left:${x}px;top:${y}px;width:${size}px;height:${size}px;border-radius:50%;background:${color};pointer-events:none;z-index:1000;box-shadow:0 0 6px ${color};`
    document.body.appendChild(dot)

    const angle = Math.random() * Math.PI * 2
    const dist = 20 + Math.random() * 30
    const dx = Math.cos(angle) * dist
    const dy = Math.sin(angle) * dist - 12

    dot.animate(
      [
        { transform: 'translate(-50%,-50%) scale(1)', opacity: 1 },
        { transform: `translate(calc(-50% + ${dx * 0.7}px), calc(-50% + ${dy * 0.7}px)) scale(1.05)`, opacity: 0.9, offset: 0.55 },
        { transform: `translate(calc(-50% + ${dx}px), calc(-50% + ${dy + 26}px)) scale(0.2)`, opacity: 0 },
      ],
      { duration: 520 + Math.random() * 240, easing: 'cubic-bezier(0.22, 0.9, 0.36, 1)' },
    ).onfinish = () => dot.remove()
  }
}
