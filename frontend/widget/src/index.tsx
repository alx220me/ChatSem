import { createRoot } from 'react-dom/client'
import { ChatWindow } from './components/ChatWindow'
import { ApiClient } from './api/client'
import { initToken, getToken } from './hooks/useAuth'
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

    // Initialise module-level token storage
    initToken(config.token)

    const api = new ApiClient('/api', getToken, config.onTokenExpired)

    createRoot(container).render(<ChatWindow config={config} api={api} />)

    console.info('[ChatSem] widget mounted', 'event_id', config.eventId)
  },
}
