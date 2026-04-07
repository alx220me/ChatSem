import React, { useEffect, useState } from 'react'

export type ToastVariant = 'success' | 'error'

export interface ToastState {
  message: string
  variant: ToastVariant
}

interface ToastProps {
  state: ToastState | null
  onDismiss: () => void
  /** Auto-dismiss delay in ms. Default: 3000 */
  duration?: number
}

const BG: Record<ToastVariant, string> = {
  success: '#16a34a',
  error:   '#dc2626',
}

const ICON: Record<ToastVariant, string> = {
  success: '✓',
  error:   '✕',
}

export function Toast({ state, onDismiss, duration = 3000 }: ToastProps): React.ReactElement | null {
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    if (!state) {
      setVisible(false)
      return
    }

    if (import.meta.env.DEV) {
      console.debug('[Toast] show', state.variant, state.message)
    }

    setVisible(true)
    const timer = setTimeout(() => {
      setVisible(false)
      onDismiss()
    }, duration)

    return () => clearTimeout(timer)
  }, [state, duration, onDismiss])

  if (!state) return null

  return (
    <div
      role="status"
      aria-live="polite"
      onClick={onDismiss}
      style={{
        position: 'fixed',
        bottom: 16,
        right: 16,
        zIndex: 200,
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: '8px 14px',
        borderRadius: 6,
        backgroundColor: BG[state.variant],
        color: '#fff',
        fontSize: 13,
        fontFamily: 'system-ui, sans-serif',
        boxShadow: '0 2px 8px rgba(0,0,0,0.25)',
        cursor: 'pointer',
        opacity: visible ? 1 : 0,
        transition: 'opacity 0.15s ease',
        userSelect: 'none',
        maxWidth: 300,
        wordBreak: 'break-word',
        pointerEvents: visible ? 'auto' : 'none',
      }}
    >
      <span style={{ fontWeight: 700, fontSize: 14 }}>{ICON[state.variant]}</span>
      <span>{state.message}</span>
    </div>
  )
}
