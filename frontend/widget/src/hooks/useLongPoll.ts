import { useEffect, useRef } from 'react'
import type { ApiClient } from '../api/client'
import type { Message } from '../types'

export function useLongPoll(
  api: ApiClient,
  chatId: string | null,
  onMessages: (msgs: Message[]) => void,
  onError?: (err: unknown) => void,
): void {
  const onMessagesRef = useRef(onMessages)
  onMessagesRef.current = onMessages
  const onErrorRef = useRef(onError)
  onErrorRef.current = onError

  useEffect(() => {
    if (!chatId) return

    const controller = new AbortController()
    let lastKnownSeq = 0
    let running = true
    const seenIds = new Set<string>()

    async function loop() {
      while (running && !controller.signal.aborted) {
        if (import.meta.env.DEV) {
          console.debug('[useLongPoll] poll', chatId, 'after', lastKnownSeq)
        }

        try {
          const response = await api.poll(chatId!, lastKnownSeq, controller.signal)
          const msgs = response.messages ?? []

          if (msgs.length > 0) {
            if (import.meta.env.DEV) {
              console.debug('[useLongPoll] received', msgs.length, 'messages')
            }

            // Deduplicate by id
            const fresh = msgs.filter((m) => !seenIds.has(m.id))
            fresh.forEach((m) => seenIds.add(m.id))

            if (fresh.length > 0) {
              const maxSeq = fresh.reduce((max, m) => Math.max(max, m.seq), lastKnownSeq)
              lastKnownSeq = maxSeq
              onMessagesRef.current(fresh)
            }
          }
          // Empty response (204 or messages:[]) → immediate reconnect (no delay)
        } catch (err) {
          if (!running || controller.signal.aborted) break

          if (import.meta.env.DEV) {
            console.warn('[useLongPoll] disconnected, reconnecting', err)
          }

          onErrorRef.current?.(err)

          // Reconnect from lastKnownSeq - 1 to catch any missed messages
          lastKnownSeq = Math.max(0, lastKnownSeq - 1)

          // Brief delay before reconnect to avoid tight loop on server errors
          await new Promise((resolve) => setTimeout(resolve, 1000))
        }
      }
    }

    void loop()

    return () => {
      running = false
      controller.abort()
    }
  }, [api, chatId])
}
