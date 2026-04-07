import { describe, it, expect, beforeEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useDrag } from '../useDrag'

// jsdom doesn't implement setPointerCapture — stub it.
const setPointerCapture = vi.fn()
const releasePointerCapture = vi.fn()

beforeEach(() => {
  setPointerCapture.mockClear()
  releasePointerCapture.mockClear()
  // Large viewport so clamping doesn't interfere with position assertions.
  Object.defineProperty(window, 'innerWidth', { value: 4000, configurable: true })
  Object.defineProperty(window, 'innerHeight', { value: 4000, configurable: true })
})

function makePointerEvent(overrides: Partial<PointerEvent> = {}): React.PointerEvent {
  return {
    pointerId: 1,
    clientX: 0,
    clientY: 0,
    preventDefault: vi.fn(),
    currentTarget: {
      setPointerCapture,
      releasePointerCapture,
      getBoundingClientRect: () => ({ width: 360, height: 520, top: 0, left: 0, right: 360, bottom: 520 }),
    } as unknown as EventTarget,
    ...overrides,
  } as unknown as React.PointerEvent
}

describe('useDrag', () => {
  describe('initial state', () => {
    it('pos is null when called without arguments', () => {
      const { result } = renderHook(() => useDrag())
      expect(result.current.pos).toBeNull()
    })

    it('pos equals the provided initial value', () => {
      const { result } = renderHook(() => useDrag({ x: 100, y: 200 }))
      expect(result.current.pos).toEqual({ x: 100, y: 200 })
    })

    it('isDragging is false initially', () => {
      const { result } = renderHook(() => useDrag())
      expect(result.current.isDragging).toBe(false)
    })

    it('exposes dragHandlers with the three pointer callbacks', () => {
      const { result } = renderHook(() => useDrag())
      expect(typeof result.current.dragHandlers.onPointerDown).toBe('function')
      expect(typeof result.current.dragHandlers.onPointerMove).toBe('function')
      expect(typeof result.current.dragHandlers.onPointerUp).toBe('function')
    })
  })

  describe('drag lifecycle', () => {
    it('sets isDragging to true on pointerDown', () => {
      const { result } = renderHook(() => useDrag({ x: 50, y: 50 }))

      act(() => {
        result.current.dragHandlers.onPointerDown(makePointerEvent({ clientX: 50, clientY: 50 }))
      })

      expect(result.current.isDragging).toBe(true)
    })

    it('updates pos on pointerMove relative to drag start', () => {
      const { result } = renderHook(() => useDrag({ x: 100, y: 100 }))

      act(() => {
        result.current.dragHandlers.onPointerDown(makePointerEvent({ clientX: 0, clientY: 0 }))
      })
      act(() => {
        result.current.dragHandlers.onPointerMove(makePointerEvent({ clientX: 30, clientY: -20 }))
      })

      expect(result.current.pos).toEqual({ x: 130, y: 80 })
    })

    it('sets isDragging to false on pointerUp', () => {
      const { result } = renderHook(() => useDrag({ x: 0, y: 0 }))

      act(() => {
        result.current.dragHandlers.onPointerDown(makePointerEvent({ clientX: 0, clientY: 0 }))
      })
      expect(result.current.isDragging).toBe(true)

      act(() => {
        result.current.dragHandlers.onPointerUp(makePointerEvent({ clientX: 10, clientY: 10 }))
      })
      expect(result.current.isDragging).toBe(false)
    })

    it('pos remains correct after pointerUp', () => {
      const { result } = renderHook(() => useDrag({ x: 200, y: 300 }))

      act(() => {
        result.current.dragHandlers.onPointerDown(makePointerEvent({ clientX: 0, clientY: 0 }))
      })
      act(() => {
        result.current.dragHandlers.onPointerMove(makePointerEvent({ clientX: 50, clientY: 50 }))
      })
      act(() => {
        result.current.dragHandlers.onPointerUp(makePointerEvent({ clientX: 50, clientY: 50 }))
      })

      expect(result.current.pos).toEqual({ x: 250, y: 350 })
    })

    it('does not update pos on pointerMove without a preceding pointerDown', () => {
      const { result } = renderHook(() => useDrag({ x: 10, y: 10 }))

      act(() => {
        result.current.dragHandlers.onPointerMove(makePointerEvent({ clientX: 999, clientY: 999 }))
      })

      expect(result.current.pos).toEqual({ x: 10, y: 10 })
    })

    it('calls setPointerCapture on pointerDown', () => {
      const { result } = renderHook(() => useDrag({ x: 0, y: 0 }))

      act(() => {
        result.current.dragHandlers.onPointerDown(makePointerEvent({ pointerId: 7 }))
      })

      expect(setPointerCapture).toHaveBeenCalledWith(7)
    })
  })

  describe('null pos (default bottom-right)', () => {
    it('pos transitions from null to a value after first drag', () => {
      // Mock window dimensions so getDefaultPos is deterministic.
      Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true })
      Object.defineProperty(window, 'innerHeight', { value: 768, configurable: true })

      const { result } = renderHook(() => useDrag())
      expect(result.current.pos).toBeNull()

      act(() => {
        result.current.dragHandlers.onPointerDown(makePointerEvent({ clientX: 0, clientY: 0 }))
      })
      act(() => {
        result.current.dragHandlers.onPointerMove(makePointerEvent({ clientX: 10, clientY: 5 }))
      })

      // After drag, pos should be non-null
      expect(result.current.pos).not.toBeNull()
      expect(typeof result.current.pos?.x).toBe('number')
      expect(typeof result.current.pos?.y).toBe('number')
    })
  })
})
