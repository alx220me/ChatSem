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
    dragStartRef.current = {
      pointerX: e.clientX,
      pointerY: e.clientY,
      originX: origin.x,
      originY: origin.y,
    }
    setIsDragging(true)

    if (import.meta.env.DEV) {
      console.debug('[useDrag] drag start', origin)
    }
  }

  function onPointerMove(e: React.PointerEvent) {
    if (!dragStartRef.current) return

    const dx = e.clientX - dragStartRef.current.pointerX
    const dy = e.clientY - dragStartRef.current.pointerY
    setPos({
      x: dragStartRef.current.originX + dx,
      y: dragStartRef.current.originY + dy,
    })
  }

  function onPointerUp(e: React.PointerEvent) {
    if (!dragStartRef.current) return

    const finalPos = {
      x: dragStartRef.current.originX + (e.clientX - dragStartRef.current.pointerX),
      y: dragStartRef.current.originY + (e.clientY - dragStartRef.current.pointerY),
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
