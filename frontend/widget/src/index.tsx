import { createRoot } from 'react-dom/client'
import { ChatWindow } from './components/ChatWindow'
import { ApiClient } from './api/client'
import type { WidgetConfig } from './types'

declare global {
  interface Window {
    ChatSem: {
      init: (config: WidgetConfig) => void
    }
  }
}

window.ChatSem = {
  async init(config: WidgetConfig) {
    const container = document.getElementById(config.containerId)
    if (!container) {
      console.error('[ChatSem] container not found:', config.containerId)
      return
    }

    // Resolve the initial token: tokenProvider takes priority over static token.
    let instanceToken: string
    if (config.tokenProvider) {
      instanceToken = await config.tokenProvider()
    } else if (config.token) {
      instanceToken = config.token
    } else {
      console.error('[ChatSem] provide either token or tokenProvider')
      return
    }

    // Per-instance token closure — supports multiple widgets on the same page.
    // tokenProvider is used for both init and refresh; onTokenExpired is the legacy fallback.
    const refreshSource = config.tokenProvider ?? config.onTokenExpired
    const api = new ApiClient(
      '/api',
      () => instanceToken,
      refreshSource
        ? async () => {
            const newToken = await refreshSource()
            instanceToken = newToken
            return newToken
          }
        : undefined,
    )

    createRoot(container).render(<ChatWindow config={config} api={api} />)

    console.info('[ChatSem] widget mounted', 'event_id', config.eventId)
  },
}
