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
  init(config: WidgetConfig) {
    const container = document.getElementById(config.containerId)
    if (!container) {
      console.error('[ChatSem] container not found:', config.containerId)
      return
    }

    // Per-instance token closure — supports multiple widgets on the same page
    let instanceToken = config.token
    const getInstanceToken = () => instanceToken

    const api = new ApiClient(
      '/api',
      getInstanceToken,
      config.onTokenExpired
        ? async () => {
            const newToken = await config.onTokenExpired!()
            instanceToken = newToken
            return newToken
          }
        : undefined,
    )

    createRoot(container).render(<ChatWindow config={config} api={api} />)

    console.info('[ChatSem] widget mounted', 'event_id', config.eventId)
  },
}
