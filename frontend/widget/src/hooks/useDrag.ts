import { useRef, useState } from 'react'

export interface DragPos {
  x: number
  y: number
}

export interface UseDragResult {
  /** Current position. null means "use CSS default (bottom-right snap)". */
  pos: DragPos | null
  /** True while the pointer is held down and moving. */
  isDragging: boolean
  /** Spread these onto the drag-handle element. */
  dragHandlers: {
    onPointerDown: (e: React.PointerEvent) => void
    onPointerMove: (e: React.PointerEvent) => void
    onPointerUp: (e: React.PointerEvent) => void
  }
}

interface DragStart {
  pointerX: number
  pointerY: number
  originX: number
  originY: number
  elW: number
  elH: number
}

/** Returns the bottom-right fallback position when no drag has occurred yet. */
function getDefaultPos(): DragPos {
  return {
    x: window.innerWidth - 372,
    y: window.innerHeight - 524,
  }
}

/**
 * useDrag — pointer-capture drag logic for the floating widget.
 *
 * `pos` is null until the user drags for the first time; the caller should
 * render using CSS `right/bottom` when pos is null (snap-to-corner default).
 */
export function useDrag(initial?: DragPos): UseDragResult {
  const [pos, setPos] = useState<DragPos | null>(initial ?? null)
  const [isDragging, setIsDragging] = useState(false)
  const dragStartRef = useRef<DragStart | null>(null)

  function onPointerDown(e: React.PointerEvent) {
    // Capture so that move/up fire even if pointer leaves the element.
    e.currentTarget.setPointerCapture(e.pointerId)
    // Prevent text selection during drag.
    e.preventDefault()

    const origin = pos ?? getDefaultPos()
    const rect = e.currentTarget.getBoundingClientRect()
    dragStartRef.current = {
      pointerX: e.clientX,
      pointerY: e.clientY,
      originX: origin.x,
      originY: origin.y,
      elW: rect.width,
      elH: rect.height,
    }
    setIsDragging(true)

    if (import.meta.env.DEV) {
      console.debug('[useDrag] drag start', origin)
    }
  }

  function onPointerMove(e: React.PointerEvent) {
    if (!dragStartRef.current) return

    const { pointerX, pointerY, originX, originY, elW, elH } = dragStartRef.current
    const dx = e.clientX - pointerX
    const dy = e.clientY - pointerY
    setPos({
      x: Math.max(0, Math.min(originX + dx, window.innerWidth - elW)),
      y: Math.max(0, Math.min(originY + dy, window.innerHeight - elH)),
    })
  }

  function onPointerUp(e: React.PointerEvent) {
    if (!dragStartRef.current) return

    const { pointerX, pointerY, originX, originY, elW, elH } = dragStartRef.current
    const finalPos = {
      x: Math.max(0, Math.min(originX + (e.clientX - pointerX), window.innerWidth - elW)),
      y: Math.max(0, Math.min(originY + (e.clientY - pointerY), window.innerHeight - elH)),
    }
    dragStartRef.current = null
    setIsDragging(false)

    if (import.meta.env.DEV) {
      console.debug('[useDrag] drag end', finalPos)
    }
  }

  return {
    pos,
    isDragging,
    dragHandlers: { onPointerDown, onPointerMove, onPointerUp },
  }
}
