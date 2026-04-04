import type { WidgetConfig } from '../types'

// Token stored in module memory — not in React state, not in localStorage (security requirement)
let _token = ''

export function initToken(token: string): void {
  _token = token
}

export function getToken(): string {
  return _token
}

export function useAuth(config: WidgetConfig) {
  // Initialise token from config on first call
  if (_token === '' && config.token) {
    _token = config.token
  }

  async function refreshToken(): Promise<string> {
    if (!config.onTokenExpired) {
      console.warn('[useAuth] session expired, no refresh callback')
      return _token
    }
    const newToken = await config.onTokenExpired()
    _token = newToken
    return newToken
  }

  return { getToken, refreshToken }
}
