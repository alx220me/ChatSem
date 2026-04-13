import React, { createContext, useCallback, useContext, useState } from 'react'
import { Toast } from '../components/Toast'
import type { ToastState, ToastVariant } from '../components/Toast'

interface ToastContextValue {
  showToast: (message: string, variant: ToastVariant) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: React.ReactNode }): React.ReactElement {
  const [state, setState] = useState<ToastState | null>(null)

  const showToast = useCallback((message: string, variant: ToastVariant) => {
    if (import.meta.env.DEV) {
      console.debug('[ToastContext] showToast', variant, message)
    }
    setState({ message, variant })
  }, [])

  const handleDismiss = useCallback(() => {
    setState(null)
  }, [])

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}
      <Toast state={state} onDismiss={handleDismiss} />
    </ToastContext.Provider>
  )
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    throw new Error('useToast must be used within a ToastProvider')
  }
  return ctx
}
