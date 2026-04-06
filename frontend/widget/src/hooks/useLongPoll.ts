import { useEffect, useRef } from 'react'
import { HttpError } from '../api/client'
import type { ApiClient } from '../api/client'
import type { Message } from '../types'

export function useLongPoll(
  api: ApiClient,
  chatId: string | null,
  onMessages: (msgs: Message[], deletedIds: string[]) => void,
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
    let lastKnownDeleteSeq = 0
    let running = true
    const seenIds = new Set<string>()
    let retryDelay = 1_000        // starts at 1s
    const MAX_RETRY_DELAY = 30_000 // caps at 30s

    async function loop() {
      while (running && !controller.signal.aborted) {
        if (import.meta.env.DEV) {
          console.debug('[useLongPoll] poll', chatId, 'after', lastKnownSeq, 'delete_seq', lastKnownDeleteSeq)
        }

        try {
          const response = await api.poll(chatId!, lastKnownSeq, lastKnownDeleteSeq, controller.signal)
          const msgs = response.messages ?? []
          const deletedIds = response.deleted_ids ?? []

          // Successful response — reset backoff
          retryDelay = 1_000

          // Advance delete cursor
          if (response.last_delete_seq != null && response.last_delete_seq > lastKnownDeleteSeq) {
            lastKnownDeleteSeq = response.last_delete_seq
          }

          // Deduplicate new messages by id
          const fresh = msgs.filter((m) => !seenIds.has(m.id))
          fresh.forEach((m) => seenIds.add(m.id))

          if (fresh.length > 0) {
            if (import.meta.env.DEV) {
              console.debug('[useLongPoll] received', fresh.length, 'messages')
            }
            const maxSeq = fresh.reduce((max, m) => Math.max(max, m.seq), lastKnownSeq)
            lastKnownSeq = maxSeq
          }

          if (fresh.length > 0 || deletedIds.length > 0) {
            if (import.meta.env.DEV && deletedIds.length > 0) {
              console.debug('[useLongPoll] deletions', deletedIds.length)
            }
            onMessagesRef.current(fresh, deletedIds)
          }
          // Empty response (204) → immediate reconnect, no delay
        } catch (err) {
          if (!running || controller.signal.aborted) break

          onErrorRef.current?.(err)

          // 401 — token expired and no refresh available; stop polling
          if (err instanceof HttpError && err.status === 401) {
            if (import.meta.env.DEV) {
              console.warn('[useLongPoll] session expired, stopping poll loop')
            }
            break
          }

          let delay = retryDelay

          if (err instanceof HttpError && err.status === 429) {
            // Rate limited — do NOT decrement seq, just wait
            delay = err.retryAfter > 0 ? err.retryAfter * 1000 : 60_000
            retryDelay = 1_000 // reset backoff after rate-limit clears
            if (import.meta.env.DEV) {
              console.warn('[useLongPoll] rate limited, waiting', delay, 'ms')
            }
          } else {
            // Network/server error — step back one seq to avoid missing a message on reconnect
            lastKnownSeq = Math.max(0, lastKnownSeq - 1)
            // Exponential backoff: 1s → 2s → 4s → … → 30s
            retryDelay = Math.min(retryDelay * 2, MAX_RETRY_DELAY)
            if (import.meta.env.DEV) {
              console.warn('[useLongPoll] disconnected, retry in', delay, 'ms', err)
            }
          }

          await new Promise((resolve) => setTimeout(resolve, delay))
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
