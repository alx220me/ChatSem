import { useState, useEffect, useRef } from 'react'
import type { ApiClient } from '../api/client'

const HEARTBEAT_INTERVAL = 30_000 // 30s

export function useOnline(
  api: ApiClient,
  chatId: string | null,
  enabled = true,
  onSuccess?: () => void,
): number {
  const onSuccessRef = useRef(onSuccess)
  onSuccessRef.current = onSuccess

  const [count, setCount] = useState(0)

  useEffect(() => {
    if (!chatId || !enabled) return

    let destroyed = false

    async function sendHeartbeat() {
      try {
        const n = await api.heartbeat(chatId!)
        if (!destroyed) setCount(n)
        onSuccessRef.current?.()
      } catch {
        // ignore — presence is best-effort
      }
    }

    void sendHeartbeat()

    const heartbeatTimer = setInterval(sendHeartbeat, HEARTBEAT_INTERVAL)

    // keepalive fetch — survives page close / tab switch
    function onBeforeUnload() {
      api.leaveBeacon(chatId!)
    }
    window.addEventListener('beforeunload', onBeforeUnload)

    return () => {
      destroyed = true
      clearInterval(heartbeatTimer)
      window.removeEventListener('beforeunload', onBeforeUnload)
      // Normal unmount (widget destroyed, SPA navigation, etc.)
      api.leave(chatId!).catch(() => {})
    }
  }, [api, chatId, enabled])

  return count
}