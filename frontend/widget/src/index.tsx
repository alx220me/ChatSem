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
    // In floating mode containerId is optional — use a stable default if omitted.
    const containerId = config.containerId || 'chatsem-widget'
    let container = document.getElementById(containerId)

    if (!container) {
      if (config.floating) {
        // Floating mode: create the mount point automatically so the host page
        // doesn't need a placeholder div.
        container = document.createElement('div')
        container.id = containerId
        document.body.appendChild(container)
        if (import.meta.env.DEV) {
          console.debug('[ChatSem] created container for floating widget', containerId)
        }
      } else {
        console.error('[ChatSem] container not found:', containerId)
        return
      }
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
      (config.apiUrl ?? '') + '/api',
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
