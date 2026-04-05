import { useState, useEffect } from 'react'
import type { ApiClient } from '../api/client'

const HEARTBEAT_INTERVAL = 30_000 // 30s
const POLL_INTERVAL = 15_000      // 15s

export function useOnline(api: ApiClient, chatId: string | null): number {
  const [count, setCount] = useState(0)

  useEffect(() => {
    if (!chatId) return

    let destroyed = false

    async function sendHeartbeat() {
      try {
        await api.heartbeat(chatId!)
      } catch {
        // ignore — presence is best-effort
      }
    }

    async function fetchCount() {
      try {
        const n = await api.getOnlineCount(chatId!)
        if (!destroyed) setCount(n)
      } catch {
        // ignore
      }
    }

    // Heartbeat first so the user is counted before we fetch the number
    async function init() {
      await sendHeartbeat()
      await fetchCount()
    }
    void init()

    const heartbeatTimer = setInterval(sendHeartbeat, HEARTBEAT_INTERVAL)
    const pollTimer = setInterval(fetchCount, POLL_INTERVAL)

    // keepalive fetch — survives page close / tab switch
    function onBeforeUnload() {
      api.leaveBeacon(chatId!)
    }
    window.addEventListener('beforeunload', onBeforeUnload)

    return () => {
      destroyed = true
      clearInterval(heartbeatTimer)
      clearInterval(pollTimer)
      window.removeEventListener('beforeunload', onBeforeUnload)
      // Normal unmount (widget destroyed, SPA navigation, etc.)
      api.leave(chatId!).catch(() => {})
    }
  }, [api, chatId])

  return count
}
