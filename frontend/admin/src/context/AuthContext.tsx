import React, { createContext, useCallback, useContext, useState } from 'react'
import { AdminApiClient } from '../api/adminClient'

interface AuthState {
  token: string | null
  eventId: string | null
  userName: string | null
}

interface AuthContextValue {
  token: string | null
  eventId: string | null
  userName: string | null
  api: AdminApiClient | null
  login: (eventId: string, apiSecret: string, name: string) => Promise<void>
  logout: () => void
}

const SESSION_KEY = 'chatsem_admin_token'
const SESSION_EVENT_KEY = 'chatsem_admin_event'
const SESSION_NAME_KEY = 'chatsem_admin_name'

function loadSession(): AuthState {
  return {
    token: sessionStorage.getItem(SESSION_KEY),
    eventId: sessionStorage.getItem(SESSION_EVENT_KEY),
    userName: sessionStorage.getItem(SESSION_NAME_KEY),
  }
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }): React.ReactElement {
  const [state, setState] = useState<AuthState>(loadSession)

  const getToken = useCallback(() => state.token ?? '', [state.token])
  const api = state.token ? new AdminApiClient('', getToken) : null

  const login = useCallback(async (eventId: string, apiSecret: string, name: string) => {
    if (import.meta.env.DEV) {
      console.debug('[AuthContext] login attempt', eventId)
    }

    const res = await fetch('/api/auth/token', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${apiSecret}`,
      },
      body: JSON.stringify({
        external_user_id: `admin-${name}`,
        event_id: eventId,
        name,
        role: 'admin',
      }),
    })

    if (!res.ok) {
      console.warn('[LoginPage] auth failed', res.status)
      throw new Error(res.status === 401 ? 'Invalid secret' : `Auth failed: ${res.status}`)
    }

    const data = (await res.json()) as { token: string }
    sessionStorage.setItem(SESSION_KEY, data.token)
    sessionStorage.setItem(SESSION_EVENT_KEY, eventId)
    sessionStorage.setItem(SESSION_NAME_KEY, name)
    setState({ token: data.token, eventId, userName: name })

    console.info('[AuthContext] logged in', eventId)
  }, [])

  const logout = useCallback(() => {
    sessionStorage.removeItem(SESSION_KEY)
    sessionStorage.removeItem(SESSION_EVENT_KEY)
    sessionStorage.removeItem(SESSION_NAME_KEY)
    setState({ token: null, eventId: null, userName: null })
  }, [])

  return (
    <AuthContext.Provider value={{ ...state, api, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
