import { useRef, useState } from 'react'

export interface Size {
  w: number
  h: number
}

/** Supported resize directions matching CSS cursor names. */
export type ResizeDir = 'e' | 's' | 'se' | 'sw' | 'w'

const MIN_SIZE: Size = { w: 240, h: 280 }
const MAX_W = 900

interface ResizeStart {
  pointerX: number
  pointerY: number
  originW: number
  originH: number
  dir: ResizeDir
}

export interface UseResizeResult {
  size: Size
  isResizing: boolean
  /** Returns pointer-event handlers to spread on a resize handle element. */
  getHandlers: (dir: ResizeDir) => {
    onPointerDown: (e: React.PointerEvent) => void
    onPointerMove: (e: React.PointerEvent) => void
    onPointerUp: (e: React.PointerEvent) => void
  }
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

export function useResize(initial: Size): UseResizeResult {
  const [size, setSize] = useState<Size>(() => ({
    w: clamp(initial.w, MIN_SIZE.w, MAX_W),
    h: clamp(initial.h, MIN_SIZE.h, Math.floor(window.innerHeight * 0.9)),
  }))
  const [isResizing, setIsResizing] = useState(false)
  const startRef = useRef<ResizeStart | null>(null)

  function getHandlers(dir: ResizeDir) {
    return {
      onPointerDown(e: React.PointerEvent) {
        e.stopPropagation() // prevent drag from firing on the same event
        e.currentTarget.setPointerCapture(e.pointerId)
        startRef.current = {
          pointerX: e.clientX,
          pointerY: e.clientY,
          originW: size.w,
          originH: size.h,
          dir,
        }
        setIsResizing(true)

        if (import.meta.env.DEV) {
          console.debug('[useResize] resize start', { dir, size })
        }
      },

      onPointerMove(e: React.PointerEvent) {
        const s = startRef.current
        if (!s) return

        const dx = e.clientX - s.pointerX
        const dy = e.clientY - s.pointerY

        let newW = s.originW
        let newH = s.originH

        if (s.dir === 'e' || s.dir === 'se') newW = s.originW + dx
        if (s.dir === 'w' || s.dir === 'sw') newW = s.originW - dx
        if (s.dir === 's' || s.dir === 'se' || s.dir === 'sw') newH = s.originH + dy

        const maxH = Math.floor(window.innerHeight * 0.9)
        setSize({
          w: clamp(newW, MIN_SIZE.w, MAX_W),
          h: clamp(newH, MIN_SIZE.h, maxH),
        })
      },

      onPointerUp(_e: React.PointerEvent) {
        if (!startRef.current) return

        if (import.meta.env.DEV) {
          console.debug('[useResize] resize end', size)
        }
        startRef.current = null
        setIsResizing(false)
      },
    }
  }

  return { size, isResizing, getHandlers }
}
