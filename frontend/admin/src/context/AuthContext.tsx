import React, { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { AdminApiClient } from '../api/adminClient'

interface AuthState {
  token: string | null
  userName: string | null
}

interface AuthContextValue {
  token: string | null
  eventId: string | null
  userName: string | null
  api: AdminApiClient | null
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const SESSION_KEY = 'chatsem_admin_token'
const SESSION_NAME_KEY = 'chatsem_admin_name'

function loadSession(): AuthState {
  return {
    token: sessionStorage.getItem(SESSION_KEY),
    userName: sessionStorage.getItem(SESSION_NAME_KEY),
  }
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }): React.ReactElement {
  const [state, setState] = useState<AuthState>(loadSession)

  const getToken = useCallback(() => state.token ?? '', [state.token])

  const logout = useCallback(() => {
    sessionStorage.removeItem(SESSION_KEY)
    sessionStorage.removeItem(SESSION_NAME_KEY)
    setState({ token: null, userName: null })
  }, [])

  const api = useMemo(
    () => (state.token ? new AdminApiClient('', getToken, logout) : null),
    [state.token, getToken, logout],
  )

  const login = useCallback(async (username: string, password: string) => {
    if (import.meta.env.DEV) {
      console.debug('[AuthContext] login attempt', username)
    }

    const res = await fetch('/api/admin/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })

    if (!res.ok) {
      console.warn('[AuthContext] login failed', res.status)
      throw new Error(res.status === 401 ? 'Invalid credentials' : `Login failed: ${res.status}`)
    }

    const data = (await res.json()) as { token: string }
    sessionStorage.setItem(SESSION_KEY, data.token)
    sessionStorage.setItem(SESSION_NAME_KEY, username)
    setState({ token: data.token, userName: username })

    console.info('[AuthContext] logged in', username)
  }, [])

  return (
    <AuthContext.Provider value={{ ...state, eventId: null, api, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
